//========================================================================
// refyne Wayland decoration helpers (patch over go-gl/glfw v3.4).
//
// Compiled as part of the Wayland cgo translation unit, AFTER wl_window.c
// (see c_glfw_lin_wayland.go), so internal.h, the _glfw global and all
// _GLFW* types plus the generated xdg-shell wrappers are already in scope.
//
// These expose the three things refyne cannot otherwise reach for custom
// client-side decorations, all by reusing state GLFW already tracks:
//   - interactive move           -> xdg_toplevel_move
//   - interactive edge/corner resize -> xdg_toplevel_resize
//   - the granted xdg-decoration mode (SSD vs CSD detection)
//
// This file is a local patch; see ../../VENDORING.md. Keep it self-contained
// so re-syncs with upstream go-gl/glfw stay mechanical.
//========================================================================

#include "internal.h"

#if defined(_GLFW_WAYLAND)

// The public GLFWwindow* handle is, internally, a _GLFWwindow*. Reuse GLFW's
// already-tracked seat and most recent input serial (updated by GLFW's own
// pointer/keyboard handlers) so the compositor accepts the move/resize grab.

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
                            (uint32_t) edges, _glfw.wl.serial);
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
