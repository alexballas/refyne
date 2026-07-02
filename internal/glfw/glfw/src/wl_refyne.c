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

#include <math.h>

#if defined(_GLFW_WAYLAND)

// Approximate libadwaita's active CSD shadow layers in one cached alpha atlas:
// a broad 6% shadow plus a tighter 4% shadow. The atlas keeps a longer tail
// than the CSS blur radius so the fade remains smooth at its outer edge.
#define GLFW_REFYNE_SHADOW_SIZE 12
// Keep in sync with internal/driver/glfw.windowCornerRadius.
#define GLFW_REFYNE_SHADOW_CORNER_RADIUS 12
#define GLFW_REFYNE_SHADOW_BROAD_SPREAD 5
#define GLFW_REFYNE_SHADOW_BROAD_ALPHA 0.06f
#define GLFW_REFYNE_SHADOW_TIGHT_SPREAD 2
#define GLFW_REFYNE_SHADOW_TIGHT_SIZE 7
#define GLFW_REFYNE_SHADOW_TIGHT_ALPHA 0.04f

static float calculateRefyneShadowLayer(float distanceSquared,
                                        int spread,
                                        int size,
                                        float alpha)
{
    const float spreadSquared = (float) (spread * spread);
    const float sizeSquared = (float) (size * size);

    if (distanceSquared <= spreadSquared)
        return alpha;
    if (distanceSquared >= sizeSquared)
        return 0.f;

    const float strength =
        1.f - (distanceSquared - spreadSquared) /
              (sizeSquared - spreadSquared);
    return alpha * strength * strength;
}

static void setRefyneShadowPixel(unsigned char* pixel, float distanceSquared)
{
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

static void destroyRefyneShadowSlice(_GLFWfallbackEdgeWayland* slice)
{
    // The slices double as pointer-enterable resize handles, and they are
    // destroyed exactly when the pointer is likely on one (grabbing an edge
    // starts an interactive resize, whose first RESIZING configure tears the
    // shadow down). Without this the seat's cached focus would keep pointing
    // at the freed wl_surface proxy.
    invalidatePointerSurface(slice->surface);

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
    window->wl.refyneShadow.focus = NULL;
}

uint32_t _glfwRefyneWindowShadowEdge(_GLFWwindow* window,
                                    struct wl_surface* surface)
{
    if (!surface || !window->wl.refyneShadow.visible)
        return XDG_TOPLEVEL_RESIZE_EDGE_NONE;

    const struct
    {
        struct wl_surface* surface;
        uint32_t edge;
    } slices[] =
    {
        { window->wl.refyneShadow.topLeft.surface,     XDG_TOPLEVEL_RESIZE_EDGE_TOP_LEFT },
        { window->wl.refyneShadow.top.surface,         XDG_TOPLEVEL_RESIZE_EDGE_TOP },
        { window->wl.refyneShadow.topRight.surface,    XDG_TOPLEVEL_RESIZE_EDGE_TOP_RIGHT },
        { window->wl.refyneShadow.left.surface,        XDG_TOPLEVEL_RESIZE_EDGE_LEFT },
        { window->wl.refyneShadow.right.surface,       XDG_TOPLEVEL_RESIZE_EDGE_RIGHT },
        { window->wl.refyneShadow.bottomLeft.surface,  XDG_TOPLEVEL_RESIZE_EDGE_BOTTOM_LEFT },
        { window->wl.refyneShadow.bottom.surface,      XDG_TOPLEVEL_RESIZE_EDGE_BOTTOM },
        { window->wl.refyneShadow.bottomRight.surface, XDG_TOPLEVEL_RESIZE_EDGE_BOTTOM_RIGHT },
    };

    for (size_t i = 0;  i < sizeof(slices) / sizeof(slices[0]);  i++)
    {
        if (slices[i].surface == surface)
            return slices[i].edge;
    }

    return XDG_TOPLEVEL_RESIZE_EDGE_NONE;
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

    // A bare commit latches all staged double-buffered state. If a size
    // change is still waiting for its matching content buffer, let the
    // imminent eglSwapBuffers carry the geometry instead of scaling the old
    // buffer to the new size.
    if (!window->wl.sizeCommitPending)
        wl_surface_commit(window->wl.surface);
}

static GLFWbool createRefyneShadowBuffer(_GLFWwindow* window)
{
    if (window->wl.refyneShadow.buffer)
        return GLFW_TRUE;

    const int margin = GLFW_REFYNE_SHADOW_SIZE;
    const int radius = GLFW_REFYNE_SHADOW_CORNER_RADIUS;
    const int roundedCenter = margin + radius;
    const int roundedSide = roundedCenter * 2 + 1;
    const int squareSide = margin * 2 + 1;
    const int atlasWidth = roundedSide;
    const int atlasHeight = roundedSide + squareSide;
    unsigned char* pixels =
        _glfw_calloc((size_t) atlasWidth * atlasHeight, 4);
    if (!pixels)
        return GLFW_FALSE;

    // Top-corner tiles model the same radius as the canvas title bar. They
    // extend under the transparent framebuffer corners so the tight shadow
    // layer follows the curved body instead of revealing a square outline.
    for (int y = 0;  y < roundedSide;  y++)
    {
        for (int x = 0;  x < roundedSide;  x++)
        {
            const int dx = x - roundedCenter;
            const int dy = y - roundedCenter;
            float distance =
                sqrtf((float) (dx * dx + dy * dy)) - (float) radius;
            if (distance < 0.f)
                distance = 0.f;

            unsigned char* pixel =
                pixels + ((size_t) y * atlasWidth + x) * 4;
            setRefyneShadowPixel(pixel, distance * distance);
        }
    }

    // The window body still has square bottom corners. Store their smaller
    // atlas below the rounded one so all eight slices share one wl_shm buffer.
    for (int y = 0;  y < squareSide;  y++)
    {
        for (int x = 0;  x < squareSide;  x++)
        {
            const int dx = x - margin;
            const int dy = y - margin;
            unsigned char* pixel =
                pixels + ((size_t) (roundedSide + y) * atlasWidth + x) * 4;
            setRefyneShadowPixel(pixel, (float) (dx * dx + dy * dy));
        }
    }

    const GLFWimage image = { atlasWidth, atlasHeight, pixels };
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

    // Keep shadow commits synchronized with the parent content surface. During
    // interactive resize the compositor sends a configure, we update the shadow
    // slice viewports, then Go renders and swaps the matching EGL buffer. If
    // these subsurfaces are desynchronized, their viewport commits can take
    // effect before that buffer swap and the shadow/resize extents visually
    // lead the content by one frame on Mutter/Mesa.
    wl_subsurface_set_sync(slice->subsurface);

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

    // Keep the default whole-surface input region. The eight non-overlapping
    // slices double as forgiving edge/corner resize handles, matching native
    // CSD where the shadow margin contributes to the effective resize area.

    wl_surface_commit(slice->surface);
    return GLFW_TRUE;
}

static GLFWbool createRefyneWindowShadow(_GLFWwindow* window)
{
    const int m = GLFW_REFYNE_SHADOW_SIZE;
    const int r = GLFW_REFYNE_SHADOW_CORNER_RADIUS;
    const int c = m + r;
    const int squareY = c * 2 + 1;
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
                                 0, 0, c, c, -m, -m, c, c) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.top,
                                 c, 0, 1, m, r, -m, w - r * 2, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.topRight,
                                 c + 1, 0, c, c, w - r, -m, c, c) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.left,
                                 0, squareY + m, m, 1, -m, r, m, h - r) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.right,
                                 m + 1, squareY + m, m, 1, w, r, m, h - r) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.bottomLeft,
                                 0, squareY + m + 1, m, m, -m, h, m, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.bottom,
                                 m, squareY + m + 1, 1, m, 0, h, w, m) ||
        !createRefyneShadowSlice(window, &window->wl.refyneShadow.bottomRight,
                                 m + 1, squareY + m + 1, m, m, w, h, m, m))
    {
        destroyRefyneShadowSurfaces(window);
        return GLFW_FALSE;
    }

    window->wl.refyneShadow.visible = GLFW_TRUE;
    setRefyneWindowGeometry(window);
    return GLFW_TRUE;
}

static void resizeRefyneWindowShadow(_GLFWwindow* window)
{
    const int m = GLFW_REFYNE_SHADOW_SIZE;
    const int r = GLFW_REFYNE_SHADOW_CORNER_RADIUS;
    const int w = window->wl.width;
    const int h = window->wl.height;

    wl_subsurface_set_position(window->wl.refyneShadow.topRight.subsurface,
                               w - r, -m);
    wl_subsurface_set_position(window->wl.refyneShadow.right.subsurface,
                               w, r);
    wl_subsurface_set_position(window->wl.refyneShadow.bottomLeft.subsurface,
                               -m, h);
    wl_subsurface_set_position(window->wl.refyneShadow.bottom.subsurface,
                               0, h);
    wl_subsurface_set_position(window->wl.refyneShadow.bottomRight.subsurface,
                               w, h);

    wp_viewport_set_destination(window->wl.refyneShadow.top.viewport,
                                w - r * 2, m);
    wp_viewport_set_destination(window->wl.refyneShadow.left.viewport,
                                m, h - r);
    wp_viewport_set_destination(window->wl.refyneShadow.right.viewport,
                                m, h - r);
    wp_viewport_set_destination(window->wl.refyneShadow.bottom.viewport, w, m);

    wl_surface_commit(window->wl.refyneShadow.top.surface);
    wl_surface_commit(window->wl.refyneShadow.left.surface);
    wl_surface_commit(window->wl.refyneShadow.right.surface);
    wl_surface_commit(window->wl.refyneShadow.bottom.surface);

    // Update the window geometry as pending double-buffered state only; do NOT
    // commit the main surface here. During interactive resize a fresh
    // xdg_surface.configure arrives every frame, and committing now would latch
    // the new geometry against the still-attached old-size content buffer
    // (EGL has not swapped yet). The compositor would then position the surface
    // for the new geometry while showing old-size content for one frame, then
    // snap back on the next eglSwapBuffers -> the content trembles in the
    // resize direction. Every size-changed path that reaches here is followed
    // by _glfwInputWindowDamage -> a Fyne paint -> eglSwapBuffers, which commits
    // the main surface and atomically applies this geometry plus the synced
    // shadow subsurface positions and viewport commits with the matching
    // buffer. This mirrors the maximized/fullscreen branch in
    // _glfwRefyneUpdateWindowShadow, which defers to the content swap for the
    // same reason.
    setRefyneWindowGeometry(window);
}

// Returns true when it changed state that is synchronized by a parent commit.
// Size-changing configure paths should let the next content-buffer swap commit
// it, so the window geometry and visible buffer stay in phase.
GLFWbool _glfwRefyneUpdateWindowShadow(_GLFWwindow* window)
{
    if (!window->wl.refyneShadow.requested)
    {
        if (window->wl.refyneShadow.visible)
        {
            destroyRefyneShadowSurfaces(window);
            setRefyneWindowGeometry(window);
            return GLFW_TRUE;
        }
        return GLFW_FALSE;
    }

    // Shadows stay up during interactive resize, matching native CSD
    // (GTK/libadwaita). Their slice updates are synchronized subsurface state
    // plus parent-latched positions, so they apply atomically with the
    // matching content-buffer swap; hiding them here was only ever a
    // workaround for the resize wobble whose real cause (the deferred EGL
    // window resize) has since been fixed.
    if (window->wl.maximized || window->wl.fullscreen)
    {
        if (window->wl.refyneShadow.visible)
        {
            destroyRefyneShadowSurfaces(window);
            setRefyneWindowGeometry(window);
            return GLFW_TRUE;
        }
        // Let the next content-buffer swap apply the geometry; committing
        // here would clamp the pending size change to the old buffer.
        setRefyneWindowGeometry(window);
        return GLFW_FALSE;
    }

    if (!window->wl.refyneShadow.visible)
    {
        if (!createRefyneWindowShadow(window))
        {
            setRefyneWindowGeometry(window);
            return GLFW_FALSE;
        }
        return GLFW_TRUE;
    }

    resizeRefyneWindowShadow(window);
    return GLFW_TRUE;
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
        // Same rule as the configure path: never commit bare while staged
        // size state awaits its matching content buffer.
        if (_glfwRefyneUpdateWindowShadow(window) &&
            !window->wl.sizeCommitPending)
        {
            wl_surface_commit(window->wl.surface);
        }
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

// Reports whether the window is inside a compositor-driven interactive resize
// (the xdg_toplevel RESIZING state). While this is set, the client must follow
// the compositor's configure sizes; client-initiated resizes would fight the
// grab and make the window size oscillate.
GLFWAPI int glfwRefyneInteractiveResizing(GLFWwindow* handle)
{
    _GLFWwindow* window = (_GLFWwindow*) handle;
    if (!window)
        return 0;
    return window->wl.resizing == GLFW_TRUE;
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
