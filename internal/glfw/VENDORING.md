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

Backported upstream fixes (post-3.4 glfw/glfw master; drop when re-syncing to a
base that already contains them — check GLFW_C_REVISION.txt against each SHA):
- 42dc1ff — Wayland: free fractionalScale/scalingViewport in _glfwDestroyWindowWayland (leak)
- 3573c5a — Wayland: create keyRepeatTimerfd in the wl_seat registry handler instead of
  _glfwInitWayland, so glfwInit no longer segfaults on a compositor with no seat (#2517)
- ac10768 — Wayland: free the partial buffer when reading a data offer fails midway (leak)
- 1ce855b — Wayland: bail out of lockPointer/confinePointer when pointer-constraints-unstable-v1
  is absent. NOTE: includes a deliberate deviation — we add the `return;` that upstream omits,
  since without it the guarded path still calls *_lock/confine_pointer(NULL) and segfaults.
- bb80481 — Wayland: destroy the wl_callback proxy from glfwPostEmptyEvent via a no-op listener (#2836, leak)
- 162896e + b579ea6 — Wayland: defer freeing all dynamically loaded modules (libdecor, wayland-egl,
  xkbcommon, wayland-cursor, libEGL, libwayland-client) to the end of _glfwTerminateWayland,
  after wl_display_disconnect and proxy teardown (#2744). Pairs with the egl_context.c guard
  `&& _glfw.platform.platformID != GLFW_PLATFORM_WAYLAND` in _glfwTerminateEGL.
- 506c11b + feb2a6b + 768e81a + a98badf — Wayland: harden key repeat handling by
  ignoring timer events when no window has keyboard focus, stopping the repeat timer when
  the focused window is destroyed, only stopping repeat on release of the repeating
  scancode, and seeding default repeat info for pre-v4 wl_keyboard objects.
- 001f94e + 99cdcfb + 05f57c0 — Wayland: batch pointer events on wl_pointer.frame
  and prefer axis_value120 high-resolution wheel data when available. Adapted to keep
  Refyne's pointerFocus, fallback decoration focus, refyneShadow focus, and shadow resize
  behavior instead of switching wholesale to upstream's pointerSurface-only flow.
- Skipped 50b0a13 (depends on the unapplied EGL-swap fix fdd14e65) and the drag-enter NULL guard
  51b6434 (already covered by refyne's portal rewrite of dataDeviceHandleEnter).

Re-sync procedure: recopy upstream over this dir, delete go.mod/go.sum,
then re-apply the patches above (they are isolated to the listed files
except the wl_init.c registry binding).
