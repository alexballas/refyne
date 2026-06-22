# Upstream GLFW Adoption Plan

Snapshot checked: GLFW `master` at `567b1ec2442d59525e24c19e8d413df6baf02496`
on 2026-06-14.

Current Refyne C baseline: `7b6aead9fb88b3623e3b3725ebb42670cbe4c579`
from `GLFW_C_REVISION.txt`.

Compare URL:
https://github.com/glfw/glfw/compare/7b6aead9fb88b3623e3b3725ebb42670cbe4c579...master

Do not wholesale replace `internal/glfw/glfw`. Refyne has local Wayland patches
documented in `VENDORING.md`, including CSD/shadow resize helpers,
cursor-shape-v1, xdg-toplevel-icon-v1, and FileTransfer portal drop support.

## Progress

- Done: Wayland key repeat fixes, implemented on 2026-06-15 and recorded in
  `VENDORING.md`.
- Done: Wayland pointer/frame/scroll handling, implemented on 2026-06-15 and
  recorded in `VENDORING.md`.
- Next on the Wayland track: Wayland fallback decoration fixes.
- Remaining: X11 robustness fixes, Wayland fallback decoration fixes, optional
  public API additions, and the Wayland EGL swap-buffer timeout decision.
- Verification note: compile-only checks pass for default and Wayland builds;
  full driver UI tests currently fail in pre-existing/environment-sensitive
  image and clipboard cases listed under items 1 and 3.

## 1. Wayland Key Repeat Fixes (Done)

Summary:
Backport upstream key-repeat fixes so repeat events do not fire after focus loss,
the repeat timer is stopped when the focused window is destroyed, releasing an
unrelated key does not stop the repeating key, and very old compositors get sane
repeat defaults.

Status:
Implemented on 2026-06-15 in `internal/glfw/glfw/src/wl_window.c` and recorded
in `internal/glfw/VENDORING.md`.

Verification on 2026-06-15:
- `gofumpt -l -w .` passed.
- `go test -run '^$' ./internal/glfw ./internal/driver/glfw` passed.
- `go test -run '^$' -tags wayland ./internal/glfw ./internal/driver/glfw`
  passed.
- Full `go test ./internal/glfw ./internal/driver/glfw` and
  `go test -tags wayland ./internal/glfw ./internal/driver/glfw` were attempted
  but failed in `internal/driver/glfw` UI tests with image-master mismatch and
  clipboard-state failures unrelated to the key-repeat C path.

Upstream commits to inspect:
- `506c11b` Wayland: Ignore key repeat events when no window has keyboard focus
- `feb2a6b` Wayland: Reset key repeat timer on window destruction
- `768e81a` Wayland: Fix key repeat halting
- `a98badf` Wayland: Fix key repeat on very old compositors

Local files:
- `internal/glfw/glfw/src/wl_window.c`

Implementation notes:
- In `handleEvents`, guard timerfd repeat dispatch with `_glfw.wl.keyboardFocus`.
- In `_glfwDestroyWindowWayland`, stop `keyRepeatTimerfd` before clearing
  `keyboardFocus`.
- In `keyboardHandleKey`, stop the repeat timer on release only if the released
  scancode is `_glfw.wl.keyRepeatScancode`.
- In `seatHandleCapabilities`, initialize default repeat rate/delay when the
  compositor's keyboard object is older than `WL_KEYBOARD_REPEAT_INFO`.

Benefit examples:
- Prevents stuck or crashing key repeats after closing a focused window.
- Keeps holding Backspace or arrow keys reliable while other keys are released.
- Improves text input on old or minimal Wayland compositors.

Verification:
- Build with Wayland tags: `go test -tags wayland ./internal/glfw ./internal/driver/glfw`
- Manual Wayland smoke test: hold a repeating key, release another key, switch
  focus, and close the focused window while a key is repeating.

## 2. X11 Robustness Fixes

Summary:
Backport small X11 stability fixes for tiny scaled windows, monitor polling,
floating windows, EWMH error handler cleanup, and dynamic module unload.

Upstream commits to inspect:
- `cf4734c` X11: Fix detectEWMH not releasing error handler
- `6de70d8` X11: Prevent BadWindow when creating small windows
- `4df5129` X11: check crtcInfo for NULL when polling monitors
- `9363075` X11: Clamp w,h in glfwSetWindowSize to >= 1
- `a228a8b` X11: Fix window made non-floating by being hidden
- `a3095e1` X11: Fix libXext not unloaded at termination

Local files:
- `internal/glfw/glfw/src/x11_init.c`
- `internal/glfw/glfw/src/x11_monitor.c`
- `internal/glfw/glfw/src/x11_window.c`
- `internal/glfw/glfw/src/x11_platform.h` if needed by the libXext cleanup

Implementation notes:
- Clamp X11 create and set-size dimensions to at least `1`.
- Guard `XRRGetCrtcInfo` returning NULL during monitor polling.
- Preserve `_NET_WM_STATE_ABOVE` across hide/show for floating windows.
- Ensure `detectEWMH` releases the X11 error handler on early return.
- Free `xshape.handle` during X11 termination and keep local portal changes.

Benefit examples:
- Avoids X protocol BadValue errors for tiny/scaled windows.
- Reduces crashes during monitor hotplug or idle display wake.
- Keeps always-on-top windows above after hide/show.
- Avoids leaking `libXext` module handles across init/terminate cycles.

Verification:
- Default Linux tests: `go test ./internal/glfw ./internal/driver/glfw`
- Manual X11 smoke test: create tiny windows, hide/show floating windows, and
  test monitor hotplug or display wake where available.

## 3. Wayland Pointer, Frame, And Scroll Handling (Done)

Summary:
Adopt upstream `wl_pointer.frame` batching and `axis_value120` handling while
preserving Refyne's custom shadow resize and fallback decoration paths.

Status:
Implemented on 2026-06-15 in `internal/glfw/glfw/src/wl_window.c`,
`internal/glfw/glfw/src/wl_platform.h`, and `internal/glfw/glfw/src/wl_init.c`
and recorded in `internal/glfw/VENDORING.md`.

Verification on 2026-06-15:
- `gofumpt -l -w .` passed.
- `go test -run '^$' ./internal/glfw ./internal/driver/glfw` passed.
- `go test -run '^$' -tags wayland ./internal/glfw ./internal/driver/glfw`
  passed.
- Full `go test ./internal/glfw ./internal/driver/glfw` and
  `go test -tags wayland ./internal/glfw ./internal/driver/glfw` were attempted
  but failed in `internal/driver/glfw` UI tests with image-master mismatch
  output matching the existing environment-sensitive failure pattern.

Upstream commits to inspect:
- `001f94e` Wayland: Add support for pointer event frames
- `99cdcfb` Wayland: Unify pointer input paths
- `05f57c0` Wayland: Improve support for discrete scrolling

Local files:
- `internal/glfw/glfw/src/wl_window.c`
- `internal/glfw/glfw/src/wl_platform.h`

Implementation notes:
- Add upstream pending pointer event state to `_GLFWlibraryWayland`.
- Extend the pointer listener with frame, axis source, axis stop, discrete, and
  value120 handlers where supported by the protocol headers.
- Preserve Refyne-specific behavior:
  - `refyneShadow.focus`
  - `_glfwRefyneWindowShadowEdge`
  - `setCursorNameWayland`
  - cursor-shape-v1 fallback behavior
  - fallback decoration move/resize/menu behavior
- Ensure scroll events are emitted only for the content surface, not decoration
  or shadow surfaces.
- Use `axis_value120` for wheel-like scrolling when available; otherwise keep
  the legacy fixed-value path.

Benefit examples:
- Fixes duplicate scroll events on some GNOME versions.
- Keeps two-dimensional touchpad scrolling as a single coherent event.
- Improves mouse wheel distance on compositors that report high-resolution
  scroll values.
- Reduces odd cursor/menu timing caused by processing enter/motion/button
  events before the compositor's pointer frame is complete.

Verification:
- Wayland tests: `go test -tags wayland ./internal/glfw ./internal/driver/glfw`
- Manual Wayland smoke test on GNOME and KDE:
  - mouse wheel vertical scroll
  - touchpad two-axis scroll
  - horizontal scroll
  - resize via Refyne shadow edges
  - right-click window menu on fallback decorations
  - drag title area and resize fallback edges

## 4. Wayland Fallback Decoration Fixes

Summary:
Backport upstream fixes around fallback decoration cursor position, cursor
updates, button handling, scroll suppression, and window menu placement.

Upstream commits to inspect:
- `ddbb8e0` Wayland: Fix fallback decoration cursor position
- `161fb1b` Wayland: Fix fallback decoration scroll events
- `bfa1c42` Wayland: Fix fallback decoration menu placement
- `3cf9f67` Wayland: Fix fallback decoration cursor updating
- `abb9db0` Wayland: Fix fallback decoration button input

Local files:
- `internal/glfw/glfw/src/wl_window.c`
- `internal/glfw/glfw/src/wl_platform.h`

Implementation notes:
- Prefer doing this after the pointer-frame work, because upstream's current
  fallback decoration behavior depends on the unified pointer path.
- Reconcile upstream's `pointerSurface` approach with Refyne's current
  `pointerFocus`, `fallback.focus`, and `refyneShadow.focus`.
- Keep Refyne's shadow surface cursor behavior intact.

Benefit examples:
- Prevents scroll callbacks from firing when the pointer is over fallback
  decoration surfaces.
- Makes fallback resize cursors track the correct edge.
- Shows the compositor window menu at the expected position.
- Improves behavior on compositors where libdecor is disabled or unavailable.

Verification:
- Same as section 3, with extra focus on fallback CSD paths.

## 5. Public API Additions

Summary:
Optionally expose newer GLFW public APIs through the Go wrapper.

Upstream commits to inspect:
- `bf945f1` Unlimited mouse button input mode
- `621e99d` Add glfwGetEGLConfig native access function
- `8e15281` Add glfwGetGLXFBConfig native access function
- `228e582` EGL: Allow native access with defaults on Wayland

Local files:
- `internal/glfw/glfw/include/GLFW/glfw3.h`
- `internal/glfw/glfw/include/GLFW/glfw3native.h`
- `internal/glfw/glfw/src/input.c`
- `internal/glfw/glfw/src/internal.h`
- `internal/glfw/glfw/src/egl_context.c`
- `internal/glfw/glfw/src/glx_context.c`
- `internal/glfw/input.go`
- `internal/glfw/native_linbsd_wayland.go`
- `internal/glfw/native_linbsd_x11.go`

Implementation notes:
- Add `UnlimitedMouseButtonsMode` to the Go `InputMode` constants if adopting
  `GLFW_UNLIMITED_MOUSE_BUTTONS`.
- Add Go wrappers for `GetEGLConfig` and `GetGLXFBConfig` only if there is a
  concrete Refyne or application need.
- Check that allowing EGL native access with `GLFW_NATIVE_CONTEXT_API` does not
  conflict with existing context setup.

Benefit examples:
- Apps can receive mouse buttons beyond button 8 when opted in.
- Advanced renderers can inspect the actual EGL/GLX framebuffer config.
- Debugging platform-specific GL setup becomes easier.

Verification:
- Build all affected cgo paths.
- Add wrapper tests only where the existing GLFW wrapper test style supports
  native handles.

## 6. Wayland EGL Swap-Buffer Timeout Series

Summary:
Upstream now avoids indefinite blocking in Wayland `eglSwapBuffers` by using a
per-window frame callback queue and timeout-aware wait.

Upstream commits to inspect:
- `fdd14e6` Wayland: Fix EGL buffer swap blocking indefinitely
- `50b0a13` Wayland: Fix deadlock causing timeout

Local files:
- `internal/glfw/glfw/src/wl_window.c`
- `internal/glfw/glfw/src/wl_platform.h`
- `internal/glfw/glfw/src/wl_init.c`
- `internal/glfw/glfw/src/egl_context.c`
- `internal/driver/glfw/present_wayland.go`
- `internal/driver/glfw/window_desktop.go`

Implementation notes:
- Treat this as an architectural decision, not a simple cherry-pick.
- Refyne currently calls `glfw.SwapInterval(0)` on Wayland and uses its own
  `presentGate` frame callback in Go. That already mitigates suspended surface
  hangs.
- Do not run both pacing systems blindly. Choose one:
  - keep Refyne's Go `presentGate` and skip upstream's EGL frame wait, or
  - adopt upstream's EGL wait and simplify/remove the Go `presentGate`.
- If adopting upstream's EGL queue, add the new dynamically loaded Wayland
  symbols in `wl_platform.h` and `wl_init.c`.

Benefit examples:
- Could make swap behavior more GLFW-native and robust.
- May reduce custom Refyne driver code over time.
- Useful if hangs remain despite the current `presentGate` and `SwapInterval(0)`
  approach.

Verification:
- Wayland manual smoke test with visible, hidden, minimized, covered, and
  suspended windows.
- Confirm no double-throttling or frame starvation.
- Confirm `SwapInterval(0)` behavior still matches Refyne expectations if the
  Go `presentGate` remains.

## Suggested Order

1. X11 robustness fixes.
2. Wayland pointer/frame/scroll handling.
3. Wayland fallback decoration fixes.
4. Public API additions, only if needed.
5. Wayland EGL swap-buffer timeout, only after deciding whether GLFW C or
   Refyne Go should own Wayland frame pacing.

## General Verification

After code changes, run:

```sh
gofumpt -l -w .
go test ./internal/glfw ./internal/driver/glfw
go test -tags wayland ./internal/glfw ./internal/driver/glfw
```

If a change touches broader platform-neutral GLFW wrapper code, expand to:

```sh
go test ./...
```
