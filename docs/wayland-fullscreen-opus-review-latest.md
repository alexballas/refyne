# Latest fullscreen changes: Opus 5.5 review

Completed October 22, 2026 using Claude Opus 5.5 (`claude-opus-5-5`) with high effort through Claude Code. Session: `b06c6f0c-86d1-4dde-b575-c1b5c4f374e7`.

This is a fresh, static review of the current working tree, including the untracked implementation and tests. The reviewer made no implementation changes and ran no tests.

Parent verification:

- At review time, the hot-unplug finding below was supported by code inspection only. The underlying panic path predates the latest fullscreen-selection change. It was subsequently reproduced and fixed as described below.
- The reviewer could not recompute the cache hash with its restricted shell permissions. The parent independently recomputed the vendored Git tree hash without writing the index and confirmed it matches `7256782d35d7d6026ff046542ccf851e36a9be67`.
- The original reviewer report below describes the tree before the follow-up fixes. The earlier review remains in `docs/wayland-fullscreen-opus-review.md`.

## Follow-up fixes and verification

Implemented after this review on October 22, 2026; Opus has not reviewed these follow-up changes.

- Reproduced D1 by disabling the fullscreen output in a private headless GNOME session. Before the fix the app panicked with `FeatureUnavailable: Wayland: The platform does not support setting the window position` (`/tmp/refyne-fullscreen-disconnect-before.log`).
- Automatic fullscreen now retains a surviving monitor until surface enter/leave can reconcile the compositor's choice. Explicit fullscreen exits without attempting unsupported positioning, clears Fyne's fullscreen flags, and restores the saved windowed bounds. Pending exit bounds can be updated without resizing EGL before the configure.
- Reconciliation also runs when an output first becomes registered; duplicate enter events no longer add duplicate scale entries. When no output is entered during a transition, integer scaling retains its last known value until the next enter.
- `RescaleContext` preserves windowed restore bounds while fullscreen. Switching between explicit fullscreen outputs no longer sends an intermediate unset request, which could replace Mutter's saved windowed geometry with fullscreen dimensions.
- H3 exposed an additional lifecycle limitation: generic GLFW Show/Hide ignored fullscreen windows entirely. The local Wayland path now permits unmapping/remapping fullscreen, including showing a window first made fullscreen while hidden. Automatic fullscreen is idempotent, giving one `set_fullscreen(nil)` per remapped shell role.
- Added isolated Mutter integration tests for automatic and explicit output removal, removal of an unmapped window's placeholder output, switching between different fullscreen outputs, three Hide/Show cycles, preservation of restore bounds across a rescale callback, and fullscreen from a window spanning both outputs. Existing repeated fullscreen and canvas/layout restoration tests also pass.
- Protocol tracing confirmed both output enters before the spanning-window fullscreen request and a subsequent leave of the other output (`/tmp/refyne-fullscreen-spanning-verified.log`). It also confirmed one automatic fullscreen request per remapped role and direct explicit retargeting without intermediate unfullscreen (`/tmp/refyne-fullscreen-lifecycle-suite.log`).

The tests use software rendering and virtual monitor disable/re-enable. They do not cover physical unplug, real GPU/direct scanout, removal of the last output, or real mixed/fractional DPI transitions. Deliberately reordered `enter`/`done` and duplicate-enter events have been handled in code but not forced in this Mutter session.

The original reviewer report follows unchanged.

---

# Static review: Wayland compositor-selected fullscreen

This was a read-only review. I didn't edit files, run tests, launch anything or touch the compositor. Everything below comes from reading the working tree and the existing log. Nothing was reproduced.

## 1. Findings, ordered by severity

### Confirmed by code trace (not reproduced)

**D1 — Medium-High, pre-existing. Removing an output while GLFW thinks a fullscreen window is on it should panic the Go app. The new reconciliation rarely prevents this.**

How it happens:
1. `registryHandleGlobalRemove` (`internal/glfw/glfw/src/wl_init.c:250-252`) hands the removed monitor to the generic disconnect handler.
2. For any window whose `window->monitor` is that monitor, `monitor.c:128-131` calls `setWindowMonitor(NULL, …)` and then `_glfw.platform.setWindowPos`.
3. On Wayland, `setWindowPos` always raises `GLFW_FEATURE_UNAVAILABLE` (`wl_window.c:2921-2927`).
4. The next `glfw.PollEvents`/`WaitEvents` calls `panicError()` (`internal/glfw/window.go:1009-1011`, `1027-1030`). `acceptError()` with no accepted codes reaches the `default:` branch and panics on `FeatureUnavailable` (`internal/glfw/error.go:212-221`).

Fyne avoids `SetPos` on Wayland itself (`window_desktop.go:180`), but this call happens inside GLFW.

Why the new code mostly doesn't help:
- `reconcileFullscreenOutput` only moves the GLFW monitor when exactly one entered output remains (`wl_window.c:517`).
- **Typical trigger:** a window is fullscreen on output X and X is unplugged or disabled. The surface had entered only X, so removing it leaves zero entered outputs. No reconciliation happens, and the panic path runs.
- **Newer trigger:** the monitor GLFW records for a fullscreen window may be a placeholder that the window isn't actually on. That can happen before the first enter event (fullscreen-before-Show), while the window is hidden, or while it straddles two outputs. Unplugging that placeholder output now takes a window that is really on a surviving output down the same path. Before this change, the explicit `set_fullscreen(monitors[0])` meant the window really was on that monitor.
- The comment at `wl_init.c:247-249` says reconciliation happens "before the generic disconnect handler runs". That only holds if the compositor has already sent `enter` for another output before `global_remove`. The ordering in Mutter is unverified.

Even without the panic, the generic path leaves fullscreen while Fyne's `w.fullScreen` stays true. It also stores the fullscreen size as `wl.restore` (`wl_window.c:3208-3209`).

The older review's H2 (a NULL `leave` crash) is fixed by the new guards at `wl_window.c:543` and `wl_window.c:590`. This later panic was not covered.

### Hypotheses (not verified)

- **H1 — Low, new.** The GLFW monitor for the window can stay stale indefinitely:
  - If a surface enters an output before that output's `wl_output.done`, the output isn't in `_glfw.monitors` yet. `monitorForEnteredOutput` (`wl_window.c:520-524`) returns NULL, and nothing re-runs reconciliation when the monitor is added later.
  - An unbalanced enter/leave pair keeps the count at 2 or more.

  In Fyne this mainly affects `GetMonitor()` and D1. `getMonitorForWindow` is only used for centering and scale, and both are no-ops on Wayland.
- **H2 — Low, behavior change.** `_glfwRemoveOutputWayland` now recomputes the buffer scale on removal (`wl_window.c:581-582`). With zero entered outputs, `maxScale` falls to 1 (`wl_window.c:497-506`), which fires a content-scale callback until the next `enter`. GNOME uses fractional-scale-v1 and returns early at `:493`, so this mostly affects integer-scale compositors. It is arguably more correct than the old dangling entry.
- **H3 — Info.** Hide followed by Show while fullscreen now re-maps with `set_fullscreen(NULL)` (`wl_window.c:1096-1098`). Mutter will choose the output by its own placement rules (pointer or focus monitor), not necessarily where the window was before. The old code always used `monitors[0]`, so this is unlikely to be worse, but it is untested.

### Checked and found correct

- **Bypassing `glfwSetWindowMonitor`** (`wl_refyne.c:26-46`):
  - The generic entry point only adds validation and `window->videoMode` (`window.c:997-1015`). Wayland never reads `videoMode`.
  - Skipping `releaseMonitor` when switching from an explicit monitor to compositor-selected is correct, and it avoids the unset/fallback-decoration flash.
  - `acquireMonitor` still sets the idle inhibitor.
  - Auto-iconify is disabled by the driver (`window_desktop.go:913`). Libdecor is disabled too (`loop_desktop.go:48`), so the libdecor branches never run.
- **Platform guarding:** `runningWayland()` reflects `glfw.GetPlatform()` (`loop_desktop.go:59`), so the C helper never touches the `window->wl` fields of an X11 window. The build tags of `fullscreen_wayland.go`, `c_glfw_lin_wayland.go` and `native_linbsd_wayland_refyne.go` are equivalent. `wl_refyne.c` uses the static helpers from `wl_window.c` legitimately, because both files are in the same translation unit (`c_glfw_lin_wayland.go:7-10`).
- **Policy transitions** (`wl_window.c:3181-3192`):
  - The compositor-selected flag is cleared on every explicit request and every exit.
  - An explicit request for the same monitor now re-sends with the output named.
  - Repeated ordinary entry re-sends `set_fullscreen(NULL)`, which doesn't move the window.
  - In a rapid exit→enter, `restore` is either used by a 0×0 non-fullscreen configure or overwritten later. I found no stale-use path.
- **Disconnect cleanup:** removing output references before `wl_output_destroy` is ordered correctly (`wl_init.c:250-252` runs before `monitor.c:151-152` frees the monitor). `_glfwRemoveOutputWayland` is safe for windows without a surface (`wl_window.c:581`).
- **Earlier fixes:**
  - The shadow show/hide logic reads the configured `wl.fullscreen`/`wl.maximized` state (`wl_refyne.c:444`), not `window->monitor`, so the placeholder monitor doesn't affect it.
  - The deferred exit-resize is untouched by the new code.
- **Log:** `/tmp/refyne-fullscreen-current-output-verified.log` matches what you described: seven `set_fullscreen(nil)` and one `set_fullscreen(wl_output#7)` (lines 653–2061), and all listed tests show PASS.

## 2. Does it fix the jump without breaking anything?

- **The jump: yes.** The ordinary toggle no longer names an output (`window_notdarwin.go:14`, `wl_window.c:624-625`), so Mutter fullscreens the window where it already is. This is the protocol-sanctioned fix.
- **Explicit selection: preserved.** `RequestFullScreenSecondary` still goes through `doApplyFullScreen` with a named monitor. The same-monitor short-circuit is handled, and the protocol log confirms both.
- **Initial fullscreen: sound.** The placeholder is replaced once `enter` arrives.
- **Monitor lifecycle: only partly addressed.** The dangling-pointer and NULL-leave crashes are fixed. Unplugging the output a fullscreen window is on still runs into upstream GLFW's disconnect fallback, which reaches a Go panic (D1). For the new compositor-selected mode that fallback is also the wrong behavior, because the compositor would normally move the window and keep it fullscreen.

## 3. Follow-ups (proposals only, not implemented)

1. **Fix D1 in `registryHandleGlobalRemove` before `_glfwInputMonitor`:**
   - If a window is compositor-selected and its monitor is the one being removed, reassign it to any surviving monitor (`_glfw.monitors[j] != monitor`). The window then stays fullscreen and the compositor relocates it.
   - For windows with an explicit monitor, avoid the `setWindowPos` error. Either skip that call for Wayland in `monitor.c:130-131` (a vendored patch, to be listed in `VENDORING.md`), or accept `featureUnavailable` in the Go `PollEvents`/`WaitEvents*` wrappers.
   - Also decide how Fyne's `w.fullScreen` should follow a forced exit.
2. **Re-run reconciliation when a monitor is connected** (after `_glfwInputMonitor(GLFW_CONNECTED)` in `wl_monitor.c`), for any window with `fullscreenCurrentOutput` set. This covers H1.
3. **Tests**, gated like the existing ones and run only on a private Mutter:
   - **Hot-unplug:** fullscreen on LEFT, then remove LEFT (a virtual-monitor stream stop or a DisplayConfig change). Assert no panic, and that the GLFW and Fyne fullscreen states agree. Add a second case that removes the placeholder monitor while a fullscreen-before-Show window is mapped on the other output.
   - **Compositor-selected → explicit secondary on a *different* monitor:** assert `unset_fullscreen` then `set_fullscreen(#N)`, with the native size matching the target. Only the same-monitor case is covered today (`fullscreen_wayland_test.go:77-83`).
   - **Hide/Show while fullscreen:** assert a single `set_fullscreen(nil)` per map and a matching size.
   - **Straddling window:** fullscreen lands on exactly one output and the GLFW monitor reconciles after the `leave`.

**Validation gaps:**
- The fullscreen-before-Show assertion (`fullscreen_wayland_test.go:88-101`) depends on Mutter's initial placement choosing LEFT.
- There is no coverage of real GPU or direct scanout, fractional scale, or hot-unplug.
- I couldn't independently recompute `upstreamTreeSHA` (`glfw_tree_rebuild.go:12`) because my shell access was restricted.

## 4. Conclusion

From reading the code, the fix for the left-monitor jump looks correct. Ordinary fullscreen now lets GNOME keep the window on its current monitor, `RequestFullScreenSecondary` still names its monitor explicitly (including the same-monitor case), and the earlier crash and blank-window fixes are unaffected.

The one significant problem I found isn't new, but these changes are meant to cover it and only partly do. If the monitor a fullscreen window is on is unplugged or disabled, GLFW's built-in disconnect handling should make the app panic. The new output tracking only avoids that in a narrow case. It's worth fixing before you rely on multi-monitor fullscreen, along with a hot-unplug test. The other items are low-risk edge cases.

I didn't reproduce any of these issues or run any tests; everything is based on reading the code and the existing log.
