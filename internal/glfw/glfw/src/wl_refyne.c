//========================================================================
// refyne Wayland decoration helpers (patch over go-gl/glfw v3.4).
//
// Compiled as part of the Wayland cgo translation unit, AFTER wl_window.c
// (see c_glfw_lin_wayland.go), so internal.h, the _glfw global and all
// _GLFW* types plus the generated xdg-shell wrappers are already in scope.
//
// These expose the things refyne cannot otherwise reach for custom
// client-side decorations, all by reusing state GLFW already tracks:
//   - interactive move           -> xdg_toplevel_move
//   - interactive edge/corner resize -> xdg_toplevel_resize
//   - compositor window menu     -> xdg_toplevel_show_window_menu
//   - the granted xdg-decoration mode (SSD vs CSD detection)
//   - cached 8-slice subsurface shadows for refyne's custom decorations
//
// This file is a local patch; see ../../VENDORING.md. Keep it self-contained
// so re-syncs with upstream go-gl/glfw stay mechanical.
//========================================================================

#include "internal.h"

#if defined(_GLFW_WAYLAND)

// Approximate libadwaita's active CSD shadow layers in one cached alpha atlas:
// a broad 15% shadow plus a tighter 10% shadow. The atlas keeps a longer tail
// than the CSS blur radius so the fade remains smooth at its outer edge.
#define GLFW_REFYNE_SHADOW_SIZE 24
#define GLFW_REFYNE_SHADOW_BROAD_SPREAD 5
#define GLFW_REFYNE_SHADOW_BROAD_ALPHA 0.15f
#define GLFW_REFYNE_SHADOW_TIGHT_SPREAD 2
#define GLFW_REFYNE_SHADOW_TIGHT_SIZE 7
#define GLFW_REFYNE_SHADOW_TIGHT_ALPHA 0.10f

static float calculateRefyneShadowLayer(int distanceSquared,
                                        int spread,
                                        int size,
                                        float alpha)
{
    const int spreadSquared = spread * spread;
    const int sizeSquared = size * size;

    if (distanceSquared <= spreadSquared)
        return alpha;
    if (distanceSquared >= sizeSquared)
        return 0.f;

    const float strength =
        1.f - (float) (distanceSquared - spreadSquared) /
              (float) (sizeSquared - spreadSquared);
    return alpha * strength * strength;
}

static void destroyRefyneShadowSlice(_GLFWfallbackEdgeWayland* slice)
{
    if (slice->viewport)
        wp_viewport_destroy(slice->viewport);
    if (slice->subsurface)
        wl_subsurface_destroy(slice->subsurface);
    if (slice->surface)
        wl_surface_destroy(slice->surface);

    slice->surface = NULL;
    slice->subsurface = NULL;
    slice->viewport = NULL;
}

static void destroyRefyneShadowSurfaces(_GLFWwindow* window)
{
    destroyRefyneShadowSlice(&window->wl.refyneShadow.topLeft);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.top);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.topRight);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.left);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.right);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.bottomLeft);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.bottom);
    destroyRefyneShadowSlice(&window->wl.refyneShadow.bottomRight);
    window->wl.refyneShadow.visible = GLFW_FALSE;
}

static void setRefyneWindowGeometry(_GLFWwindow* window)
{
    if (!window->wl.xdg.surface)
        return;

    // Keep visual-only shadow subsurfaces out of Mutter's frame rectangle.
    // This also refreshes the effective geometry after removing the shadow for
    // maximized/fullscreen windows, when the content dimensions may change.
    xdg_surface_set_window_geometry(window->wl.xdg.surface,
                                    0, 0,
                                    window->wl.width, window->wl.height);
}

static void commitRefyneWindowGeometry(_GLFWwindow* window)
{
    if (!window->wl.surface)
        return;

    setRefyneWindowGeometry(window);
    wl_surface_commit(window->wl.surface);
}

static GLFWbool createRefyneShadowBuffer(_GLFWwindow* window)
{
    if (window->wl.refyneShadow.buffer)
        return GLFW_TRUE;

    const int margin = GLFW_REFYNE_SHADOW_SIZE;
    const int side = margin * 2 + 1;
    unsigned char* pixels = _glfw_calloc((size_t) side * side, 4);
    if (!pixels)
        return GLFW_FALSE;

    for (int y = 0;  y < side;  y++)
    {
        for (int x = 0;  x < side;  x++)
        {
            const int dx = x - margin;
            const int dy = y - margin;
            const int distanceSquared = dx * dx + dy * dy;
            unsigned char* pixel = pixels + ((size_t) y * side + x) * 4;
            const float broad = calculateRefyneShadowLayer(
                distanceSquared,
                GLFW_REFYNE_SHADOW_BROAD_SPREAD,
                GLFW_REFYNE_SHADOW_SIZE,
                GLFW_REFYNE_SHADOW_BROAD_ALPHA);
            const float tight = calculateRefyneShadowLayer(
                distanceSquared,
                GLFW_REFYNE_SHADOW_TIGHT_SPREAD,
                GLFW_REFYNE_SHADOW_TIGHT_SIZE,
                GLFW_REFYNE_SHADOW_TIGHT_ALPHA);
            const float alpha = broad + tight - broad * tight;

            pixel[3] = (unsigned char) (255.f * alpha + 0.5f);
        }
    }

    const GLFWimage image = { side, side, pixels };
    window->wl.refyneShadow.buffer = createShmBuffer(&image);
    _glfw_free(pixels);
    return window->wl.refyneShadow.buffer != NULL;
}

static GLFWbool createRefyneShadowSlice(_GLFWwindow* window,
                                        _GLFWfallbackEdgeWayland* slice,
                                        int sourceX, int sourceY,
                                        int sourceWidth, int sourceHeight,
                                        int x, int y, int width, int height)
{
    slice->surface = wl_compositor_create_surface(_glfw.wl.compositor);
    if (!slice->surface)
        return GLFW_FALSE;

    wl_surface_set_user_data(slice->surface, window);
    wl_proxy_set_tag((struct wl_proxy*) slice->surface, &_glfw.wl.tag);

    slice->subsurface =
        wl_subcompositor_get_subsurface(_glfw.wl.subcompositor,
                                        slice->surface, window->wl.surface);
    if (!slice->subsurface)
        return GLFW_FALSE;

    wl_subsurface_set_position(slice->subsurface, x, y);
    wl_subsurface_place_below(slice->subsurface, window->wl.surface);
    wl_subsurface_set_desync(slice->subsurface);

    slice->viewport =
        wp_viewporter_get_viewport(_glfw.wl.viewporter, slice->surface);
    if (!slice->viewport)
        return GLFW_FALSE;

    wp_viewport_set_source(slice->viewport,
                           wl_fixed_from_int(sourceX),
                           wl_fixed_from_int(sourceY),
                           wl_fixed_from_int(sourceWidth),
                           wl_fixed_from_int(sourceHeight));
    wp_viewport_set_destination(slice->viewport, width, height);
    wl_surface_attach(slice->surface, window->wl.refyneShadow.buffer, 0, 0);

    // Shadows are visual only. Pointer input continues to windows underneath.
    struct wl_region* region =
        wl_compositor_create_region(_glfw.wl.compositor);
    if (region)
    {
        wl_surface_set_input_region(slice->surface, region);
        wl_region_destroy(region);
    }

    wl_surface_commit(slice->surface);
    return GLFW_TRUE;
}

static GLFWbool createRefyneWindowShadow(_GLFWwindow* window)
{
    const int m = GLFW_REFYNE_SHADOW_SIZE;
    const int w = window->wl.width;
    const int h = window->wl.height;

    if (!_glfw.wl.viewporter || !_glfw.wl.subcompositor ||
        !window->wl.surface || !window->wl.xdg.surface)
    {
        return GLFW_FALSE;
    }

    if (!createRefyneShadowBuffer(window))
        return GLFW_FALSE;

    if (!createRefyneShadowSlice(window, &window->wl.refyneShadow.topLeft,
                                 0, 0, m, m, -m, -m, m, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.top,
                                 m, 0, 1, m, 0, -m, w, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.topRight,
                                 m + 1, 0, m, m, w, -m, m, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.left,
                                 0, m, m, 1, -m, 0, m, h) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.right,
                                 m + 1, m, m, 1, w, 0, m, h) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.bottomLeft,
                                 0, m + 1, m, m, -m, h, m, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.bottom,
                                 m, m + 1, 1, m, 0, h, w, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.bottomRight,
                                 m + 1, m + 1, m, m, w, h, m, m))
    {
        destroyRefyneShadowSurfaces(window);
        return GLFW_FALSE;
    }

    window->wl.refyneShadow.visible = GLFW_TRUE;
    commitRefyneWindowGeometry(window);
    return GLFW_TRUE;
}

static void resizeRefyneWindowShadow(_GLFWwindow* window)
{
    const int m = GLFW_REFYNE_SHADOW_SIZE;
    const int w = window->wl.width;
    const int h = window->wl.height;

    wl_subsurface_set_position(window->wl.refyneShadow.topRight.subsurface,
                               w, -m);
    wl_subsurface_set_position(window->wl.refyneShadow.right.subsurface,
                               w, 0);
    wl_subsurface_set_position(window->wl.refyneShadow.bottomLeft.subsurface,
                               -m, h);
    wl_subsurface_set_position(window->wl.refyneShadow.bottom.subsurface,
                               0, h);
    wl_subsurface_set_position(window->wl.refyneShadow.bottomRight.subsurface,
                               w, h);

    wp_viewport_set_destination(window->wl.refyneShadow.top.viewport, w, m);
    wp_viewport_set_destination(window->wl.refyneShadow.left.viewport, m, h);
    wp_viewport_set_destination(window->wl.refyneShadow.right.viewport, m, h);
    wp_viewport_set_destination(window->wl.refyneShadow.bottom.viewport, w, m);

    wl_surface_commit(window->wl.refyneShadow.top.surface);
    wl_surface_commit(window->wl.refyneShadow.left.surface);
    wl_surface_commit(window->wl.refyneShadow.right.surface);
    wl_surface_commit(window->wl.refyneShadow.bottom.surface);

    commitRefyneWindowGeometry(window);
}

void _glfwRefyneUpdateWindowShadow(_GLFWwindow* window)
{
    if (!window->wl.refyneShadow.requested)
    {
        if (window->wl.refyneShadow.visible)
            destroyRefyneShadowSurfaces(window);
        return;
    }

    if (window->wl.maximized || window->wl.fullscreen)
    {
        if (window->wl.refyneShadow.visible)
            destroyRefyneShadowSurfaces(window);
        // Let the next content-buffer swap apply the geometry. Committing here
        // would clamp an expanding maximized/fullscreen size to the old buffer.
        setRefyneWindowGeometry(window);
        return;
    }

    if (!window->wl.refyneShadow.visible)
    {
        if (!createRefyneWindowShadow(window))
            setRefyneWindowGeometry(window);
        return;
    }

    resizeRefyneWindowShadow(window);
}

void _glfwRefyneDestroyWindowShadow(_GLFWwindow* window)
{
    destroyRefyneShadowSurfaces(window);

    if (window->wl.refyneShadow.buffer)
        wl_buffer_destroy(window->wl.refyneShadow.buffer);

    window->wl.refyneShadow.buffer = NULL;
    window->wl.refyneShadow.requested = GLFW_FALSE;
}

// The public GLFWwindow* handle is, internally, a _GLFWwindow*. Reuse GLFW's
// already-tracked seat and most recent input serial (updated by GLFW's own
// pointer/keyboard handlers) so the compositor accepts the move/resize grab.

GLFWAPI void glfwRefyneSetWindowShadow(GLFWwindow* handle, int enabled)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (!window)
        return;

    if (enabled)
    {
        window->wl.refyneShadow.requested = GLFW_TRUE;
        _glfwRefyneUpdateWindowShadow(window);
    }
    else
    {
        _glfwRefyneDestroyWindowShadow(window);
        commitRefyneWindowGeometry(window);
    }
}

GLFWAPI void glfwRefyneStartWindowMove(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (window && window->wl.xdg.toplevel && _glfw.wl.seat)
        xdg_toplevel_move(window->wl.xdg.toplevel, _glfw.wl.seat, _glfw.wl.serial);
}

GLFWAPI void glfwRefyneStartWindowResize(GLFWwindow* handle, int edges)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (window && window->wl.xdg.toplevel && _glfw.wl.seat)
        xdg_toplevel_resize(window->wl.xdg.toplevel, _glfw.wl.seat,
                            _glfw.wl.serial, (uint32_t) edges);
}

GLFWAPI void glfwRefyneShowWindowMenu(GLFWwindow* handle, int xpos, int ypos)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (window && window->wl.xdg.toplevel && _glfw.wl.seat)
    {
        xdg_toplevel_show_window_menu(window->wl.xdg.toplevel, _glfw.wl.seat,
                                      _glfw.wl.serial, xpos, ypos);
    }
}

// 0 = unknown/none (e.g. GNOME, no decoration manager),
// 1 = client-side, 2 = server-side (matches zxdg_toplevel_decoration_v1 mode).
GLFWAPI int glfwRefyneDecorationMode(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (!window)
        return 0;
    return (int) window->wl.xdg.decorationMode;
}

// glfwRefyneSetWindowIcon pushes an application icon to the compositor via the
// xdg-toplevel-icon-v1 protocol (supported by e.g. KDE Plasma 6). 'pixels' is
// tightly-packed, non-premultiplied RGBA8888 (Go image order) of width*height*4
// bytes. Returns 1 on success, 0 when unsupported (no icon manager bound, e.g.
// GNOME/Mutter) or on failure.
//
// The protocol mandates a square, wl_shm-backed buffer and would raise a fatal
// 'invalid_buffer' error otherwise, so non-square input is rejected here. We
// reuse GLFW's own createShmBuffer() (defined earlier in this translation unit
// in wl_window.c), which builds a premultiplied ARGB8888 wl_shm buffer.
GLFWAPI int glfwRefyneSetWindowIcon(GLFWwindow* handle,
                                    const unsigned char* pixels,
                                    int width, int height)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (!window || !window->wl.xdg.toplevel || !_glfw.wl.iconManager)
        return 0;
    if (!pixels || width <= 0 || height <= 0 || width != height)
        return 0;

    GLFWimage image;
    image.width  = width;
    image.height = height;
    image.pixels = (unsigned char*) pixels; // createShmBuffer only reads pixels

    struct wl_buffer* buffer = createShmBuffer(&image);
    if (!buffer)
        return 0;

    struct xdg_toplevel_icon_v1* icon =
        xdg_toplevel_icon_manager_v1_create_icon(_glfw.wl.iconManager);
    xdg_toplevel_icon_v1_add_buffer(icon, buffer, 1 /* scale */);
    xdg_toplevel_icon_manager_v1_set_icon(_glfw.wl.iconManager,
                                          window->wl.xdg.toplevel, icon);

    // set_icon latches the (immutable) state, applied on the toplevel's next
    // surface commit; the toplevel keeps its icon even after the icon object is
    // destroyed. The wl_buffer must outlive the icon, so destroy icon first.
    xdg_toplevel_icon_v1_destroy(icon);
    wl_buffer_destroy(buffer);
    return 1;
}

#endif // _GLFW_WAYLAND
