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

#endif // _GLFW_WAYLAND
