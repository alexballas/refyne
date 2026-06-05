# Vendored copy of go-gl/glfw

Source: github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1
Upstream C revision: see GLFW_C_REVISION.txt
Local patches (re-apply after any re-sync):
- glfw/src/wl_refyne.c — Wayland move/resize/window-menu/decoration-mode helpers + xdg-toplevel-icon setter + cached CSD shadow resize handles
- native_linbsd_wayland_refyne.go — Go wrappers for the above
- c_glfw_lin_wayland.go + c_glfw_bsd.go — extra #include for wl_refyne.c
- glfw/src/wl_platform.h + glfw/src/wl_window.c — per-window shadow state + resize/lifecycle/pointer hooks + cursor-shape-v1 themed cursors (set_shape with a wl_cursor_theme buffer fallback)
- wl_init.c — bind xdg_toplevel_icon_manager_v1 and wp_cursor_shape_manager_v1 from the registry
- glfw/deps/wayland/xdg-toplevel-icon-v1.xml + generated headers
- glfw/deps/wayland/cursor-shape-v1.xml + generated headers (glfw/src/cursor-shape-v1-client-protocol*.h). This is the upstream staging protocol with the get_tablet_tool_v2 request removed: refyne only drives the pointer cursor and does not vendor tablet-unstable-v2, so keeping it would leave zwp_tablet_tool_v2_interface undefined at link time.
- glfw/src/file_transfer_portal.[ch] + glfw/src/wl_*.c + glfw/src/x11_*.c + c_glfw.go — optional runtime-loaded DBus support for FileTransfer portal drops, based on glfw/glfw#2763 and extended to X11
- build_cgo_hack.go — retain the root glfw/include package so go mod vendor exports generated Wayland protocol headers

Re-sync procedure: recopy upstream over this dir, delete go.mod/go.sum,
then re-apply the patches above (they are isolated to the listed files
except the wl_init.c registry binding).
