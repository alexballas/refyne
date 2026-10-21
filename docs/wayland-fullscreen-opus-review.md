# Opus 5.5 review of Wayland fullscreen behavior

Reviewer: Claude Opus 5.5 (`claude-opus-5-5`), high effort, via Claude Code. Completed October 21, 2026. Claude session: `e808cbe9-3a50-4474-aca8-6a4ed6298886`.

The reviewer received the new left-monitor reproduction, the history of both fixes, the current uncommitted diff, and the test results and limitations. The review was read-only; the recommendations below have not been implemented.

Parent verification notes:

- The monitor-selection call path was checked against the current code: a windowed Wayland window returns no fullscreen monitor, then the driver explicitly targets GLFW's first registered monitor. The actual title-bar maximize path does not select an output.
- Finding F5 below is not established for normal explicit shadow disabling: `glfwRefyneSetWindowShadow(false)` calls `_glfwRefyneDestroyWindowShadow`, which destroys all slices even when they are hidden. The early return cited by the reviewer does not demonstrate a leak on that path.
- The reviewer could not recompute the tree hash because that tool call was denied. The parent independently recomputed the Git tree hash without writing the index and confirmed it matches `0852d7eb38926d585e021e63509330994cd804a0`.
- Suggested implementation details and tests remain proposals. The hypothesized teardown and disconnect crashes have not been reproduced.

The original review follows unchanged.

---

# Review: Wayland fullscreen fixes and the left-to-right monitor jump

**In short:** both existing fixes hold up, and I found no regression in the code as written. The jump has a clear cause: Fyne doesn't know which monitor a normal window is on under Wayland, so it falls back to the first monitor GLFW registered. It then asks GNOME to go fullscreen on that specific monitor, which is your right-hand one. The real maximize button takes a different path and should not jump.

This was a read-only review. I edited nothing, ran no tests, launched no app and committed nothing. My attempt to recompute the cache-busting tree SHA was blocked by permissions, so that value is unchecked.

## 1. Findings on the existing fixes

### Confirmed by reading the code

**F1 (Medium): the new test will toggle fullscreen on your real desktop.**
- `TestWaylandFullScreenRoundTrip` (`internal/driver/glfw/window_notdarwin_notwasm_test.go:53-56`) only checks `runningWayland()`. That is just `glfw.GetPlatform() == PlatformWayland` (`loop_desktop.go:59`).
- So a plain `go test ./internal/driver/glfw/...` inside your GNOME 50.1 / NVIDIA session would fullscreen a window five times on the live desktop. That is the setup that crashed the session before.
- Fix: also require an opt-in environment variable (for example `REFYNE_WAYLAND_FULLSCREEN_TEST=1`) that the headless helper sets.

**F2 (Low): the restore size is only a fallback, and it can be wrong.**
- `wl.restore` (`wl_window.c:695-699`) is used only when the unfullscreen configure is `0x0`. Mutter normally sends its saved windowed size, so usually the compositor's size wins.
- When it is used, its source is Fyne's `w.width/w.height`, and two places can overwrite that with the fullscreen size:
  - `RescaleContext` sets `w.width, w.height = w.viewport.GetSize()` while fullscreen (`window_desktop.go:872-873`). A scale change while fullscreen would cause this, which becomes likelier once fullscreen happens on a monitor with a different scale.
  - `processResized` records sizes whenever `!w.fullScreen` (`window.go:312-315`). If you exit before the enter configure has arrived, the late fullscreen-sized configure gets recorded as the windowed size.
- Both are pre-existing. The patch doesn't make them worse, but rapid toggling and mixed-scale monitors make them easier to hit.

**F3 (Low, cosmetic): Fyne's state changes before the window does.**
- `SetFullScreen(false)` immediately sets `w.fullScreen=false` and rounds the corners again (`window_desktop.go:148-158`).
- Edge-resize cursors, the window menu and resize handles switch back on at once (`decorations_wayland.go:227, 267, 284`), but the native window stays fullscreen-sized until the configure arrives.
- Expect at most a frame or two of windowed chrome at fullscreen size. This is inherent to waiting for the compositor and is acceptable.

**F4 (Info): edge cases that are handled correctly.**
- A leftover `restore` from a quick exit followed by re-enter is always overwritten or cleared (`wl_window.c:747-751`), and it is only read for a non-fullscreen `0x0` configure.
- Hiding the window while an exit is pending is fine: the restore value is consumed by the first `0x0` configure after Show.
- Going from maximized to fullscreen and back works, because the compositor sends the maximized size.
- The unchanged libdecor branches never run, because the driver disables libdecor (`loop_desktop.go:48`).

**F5 (Info): hidden shadow surfaces can outlive their use.**
- If the shadow is turned off while its surfaces are hidden, the early return at `wl_refyne.c:405-413` keeps them alive until the shell objects are torn down (`wl_window.c:1117`).
- It's a harmless leak.

### Hypotheses (not verified)

**H1 (Medium): some paths still destroy shadow surfaces while fullscreen.**
- Close or Hide while fullscreen still destroys them, via `destroyShellObjects` → `_glfwRefyneDestroyWindowShadow` (`wl_window.c:1115-1117`, `wl_refyne.c:450-452`). The failure path in `createRefyneWindowShadow` (`wl_refyne.c:343`) does too.
- Normally the surfaces are already hidden by then, which is probably safe. The hide only takes effect on the next parent commit, though (`wl_refyne.c:128-136`). So entering fullscreen and then closing or hiding before the next frame could still destroy live surfaces while fullscreen.
- The headless software compositor almost certainly never runs Mutter's direct-scanout check, so this crash path has only been validated by your confirmation on real hardware.

**H2 (Medium, pre-existing; matters for the jump fix): unplugging a monitor may crash GLFW.**
- When an output is removed (`wl_init.c:238-251` → `monitor.c:118-152` → `wl_output_destroy` at `wl_monitor.c:194-195`), the window's output list keeps a dangling pointer.
- If a `wl_surface.leave` for that output arrives afterwards, libwayland passes a NULL output. `surfaceHandleLeave` then calls `wl_proxy_get_tag(NULL)` (`wl_window.c:546`), which would crash.
- Any fix that relies on tracking which outputs the window is on must remove disconnected outputs from that list and handle a NULL output.

**H3: the root-cause attribution for the crash.** Matching the crash to Mutter's scanout check came from comparing assembly by hand, without a symbolized stack. The workaround still makes sense either way. It's worth reporting upstream to Mutter.

### Validation gaps
- No test covers two monitors, real hardware or direct scanout.
- There is no pixel-level check; GL_FRONT readback returned zeros.
- I could not independently recompute `upstreamTreeSHA` (`glfw_tree_rebuild.go:12`).

## 2. Exact call path for the jump

**Fullscreen (the toggle button in your test app):**

1. `SetFullScreen(true)` (`window_desktop.go:148`) calls `doSetFullScreen` (`window_notdarwin.go:13-16`), which calls `getMonitorForWindow()` (`window_desktop.go:374`).
2. On Wayland, `getMonitorForWindow` skips the position-based search (`:375`) because Wayland gives no window position.
3. It calls `viewport.GetMonitor()` (`:400`). That returns the window's *fullscreen* monitor (`internal/glfw/window.go:715`), which is nil for a normal window.
4. It falls back to `glfw.GetPrimaryMonitor()` (`:402`). On Wayland that is simply `_glfw.monitors[0]` (`monitor.c:316-324`): the first monitor GLFW registered (`wl_monitor.c:106`). It knows nothing about where the window is, and here it is your right monitor.
5. `doApplyFullScreen` calls `SetMonitor(right, …)` (`window_notdarwin.go:32`), reaching `_glfwSetWindowMonitorWayland` (`wl_window.c:3139`) and then `acquireMonitor` (`:591`).
6. `acquireMonitor` sends `xdg_toplevel_set_fullscreen(toplevel, right_output)` (`:600`).
7. GNOME obeys the explicit output and moves the window to the right monitor. As far as I recall, Mutter moves the window to that monitor before fullscreening it; please check this against the 50.1 source.

Starting a window in fullscreen has the same flaw: `window.go:172-176` / `1051-1054` lead to `createShellObjects` (`wl_window.c:1060-1063`), which also names the output explicitly.

**Maximize (the real title-bar button or double-click):**

1. `decorations_wayland.go:151-159` calls `viewport.Maximize()`.
2. `_glfwMaximizeWindowWayland` (`wl_window.c:3055-3063`) sends `xdg_toplevel_set_maximized` with **no output**.
3. GNOME maximizes the window on the monitor it is already on, and Fyne never chooses a monitor.

Restore uses `unset_maximized` (`:3032-3053`). So this route should not jump. If the maximize button really does jump too, that is a separate bug.

## 3. Recommended fix (not implemented)

**Recommendation: on Wayland, let GNOME choose the output when you ask for fullscreen "on the current monitor", while keeping GLFW's normal monitor-based fullscreen state.** The Wayland protocol allows a null output, meaning the compositor picks an appropriate one.

C side (`wl_platform.h`, `wl_window.c`, `wl_refyne.c`, plus a Go wrapper):
- Add a per-window flag, e.g. `window->wl.fullscreenAnyOutput`, set through a new native call such as `glfwRefyneSetWindowFullScreenAnyOutput(handle, enabled)`.
- In `acquireMonitor` (`:600`) and `createShellObjects` (`:1062`), send `fullscreenAnyOutput ? NULL : window->monitor->wl.output`.
- Still pass a real, non-nil monitor to `SetMonitor`. That keeps `GetMonitor()` non-nil while fullscreen, keeps `SetMonitor(nil)` meaning "leave fullscreen", and leaves the idle inhibitor, auto-iconify and the fullscreen size no-op unchanged.
- Make the early return in `_glfwSetWindowMonitorWayland` (`:3145`) also notice a change of the flag. Otherwise switching from auto to an explicit `RequestFullScreenSecondary` on the same placeholder monitor would be skipped.
- Reconcile while fullscreen: when the window has entered exactly one output, update `window->monitor` with `_glfwInputWindowMonitor`. Without this, unplugging the *placeholder* monitor would wrongly drop the window out of fullscreen (`monitor.c:123-133`).
- Harden output tracking: remove disconnected outputs from `outputScales` and ignore a NULL output in leave (see H2).

Go side:
- `doSetFullScreen` sets the flag only when `runningWayland()`.
- `doSetFullScreen2` (`RequestFullScreenSecondary`) clears it and keeps sending an explicit output.
- For the placeholder monitor, choose in this order: current fullscreen monitor, then the single output the window is on, then primary. Put this in a pure helper so it can be unit-tested.
- X11 and macOS are unaffected.

**Tradeoffs:**
- **Compositor-selected output (recommended):** handles a window straddling two monitors and a window going fullscreen before it is first shown, which output tracking cannot. Briefly, `GetMonitor()` may report the placeholder until the reconcile step runs.
- **Output tracking only (smaller):** a getter returns the window's monitor when it is on exactly one output. That fixes your exact case with no protocol change. It still guesses wrong when the window straddles two monitors or hasn't been shown yet, and it still needs the disconnect hardening.

Any C change also requires regenerating `upstreamTreeSHA`, adding a `VENDORING.md` entry and running `gofumpt -l -w .`.

## 4. Focused regression tests

All of these should be gated by the opt-in variable from F1 and run only on a private headless compositor.

1. **Unit test, no compositor:** a table test of the pure selection helper covering 0, 1 and 2 entered outputs, already fullscreen, and secondary requested.
2. **Two monitors, left of primary:** run headless Mutter with two virtual monitors of *different sizes* (e.g. left 1024x768, right 1280x720), so the fullscreen canvas size alone shows which monitor was used. Place the window on the left monitor; I haven't checked the tooling for this (DisplayConfig layout plus an unsafe-mode `Eval` to move the window). Then assert:
   - a `WAYLAND_DEBUG` trace shows `set_fullscreen(nil)`;
   - the canvas size is 1024x768;
   - after exiting, the window has entered only the left output.

   Repeat the toggle five times.
3. **`RequestFullScreenSecondary`:** still sends an explicit output, and switching from auto fullscreen to it is not short-circuited.
4. **Straddling both monitors:** fullscreen lands on exactly one monitor with the matching size, and there is no crash.
5. **Fractional scale:** left monitor at 1.25 or 1.5. Assert the logical canvas size, and that the windowed size after exit equals the size before entering. This covers F2's `RescaleContext` path.
6. **Unplug a monitor while fullscreen on it:** hot-remove a virtual monitor (a ScreenCast `RecordVirtual` stream, then stop it). Assert no crash in `surfaceHandleLeave`, and that GLFW and Fyne agree on the resulting fullscreen state.
7. **Close or Hide while fullscreen**, including in the same frame as entering (H1).

## 5. Conclusion and open questions

**For the user:** the crash fix and the fix for the blank window after leaving fullscreen both look correct. The jump happens because, on Wayland, Fyne can't tell which monitor the window is on. It falls back to the first monitor GLFW knows about, your right one, and tells GNOME to go fullscreen *there*. The fix is to stop naming a monitor and let GNOME fullscreen the window where it is. The real maximize button doesn't go through this code.

**Open questions:**
- Please confirm that "maximize/minimize" meant the fullscreen toggle button. If the title-bar maximize also jumps, that is a different bug.
- Does leaving fullscreen bring the window back to the left monitor?
- Which monitor is first in GLFW's list (`_glfw.monitors[0]`) versus which monitor GNOME calls primary? This doesn't change the cause.
- How Mutter 50.1 handles a null output still needs to be checked against its source.
- Remove the live-session exposure of the new test (F1) before anyone runs the package tests in a real session.
