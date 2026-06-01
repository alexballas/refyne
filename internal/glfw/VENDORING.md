# Vendored copy of go-gl/glfw

Source: github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1
Upstream C revision: see GLFW_C_REVISION.txt
Local patches (re-apply after any re-sync):
- glfw/src/wl_refyne.c — Wayland move/resize/window-menu/decoration-mode helpers + xdg-toplevel-icon setter + cached CSD shadow resize handles
- native_linbsd_wayland_refyne.go — Go wrappers for the above
- c_glfw_lin_wayland.go + c_glfw_bsd.go — extra #include for wl_refyne.c
- glfw/src/wl_platform.h + glfw/src/wl_window.c — per-window shadow state + resize/lifecycle/pointer hooks
- wl_init.c — bind xdg_toplevel_icon_manager_v1 from the registry
- glfw/deps/wayland/xdg-toplevel-icon-v1.xml + generated headers

Re-sync procedure: recopy upstream over this dir, delete go.mod/go.sum,
then re-apply the patches above (they are isolated to the listed files
except the wl_init.c registry binding).
