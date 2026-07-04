# Review: `fyne-develop-6080-wayland-present-gate.patch`

Review of the proposed fix for [fyne-io/fyne#6080](https://github.com/fyne-io/fyne/issues/6080) — "Wayland: UI updated from `fyne.Do` hangs when the window is hidden."

## Background

**Symptom:** A Fyne app built with `-tags wayland`, moved to a non-visible workspace, and updated from a goroutine via `fyne.Do` becomes unresponsive within ~30s; the compositor reports it as hung.

**Root cause (per issue thread, maintainer-endorsed):** The render loop keeps calling `SwapBuffers` on a window even after the compositor stops presenting it (e.g. Hyprland suspends surfaces on hidden workspaces and stops sending `wl_surface.frame` callbacks to them). Mesa's `eglSwapBuffers`, with the default swap interval, blocks waiting for a frame callback that never arrives — hanging the single OS-locked GL thread, which in turn stalls `fyne.Do`'s queue.

**Maintainer-suggested direction:** avoid autonomous redraws for backgrounded/non-visible-workspace windows unless the compositor explicitly requests one; defer the dirty frame until the window is presentable again.

## What the patch does

Two independent mechanisms, added together:

1. **`glfw.SwapInterval(0)`** (`window_desktop.go`, in `create()`) — set once per window when `build.IsWayland` is true. Disables the swap-interval throttle so `SwapBuffers` returns immediately instead of blocking on a frame callback that may never come.
2. **A `presentGate` abstraction** (`present.go`, `present_wayland.go`, `present_other.go`) — on Wayland, backed by `wl_surface.frame` callbacks via cgo:
   - `arm()` requests a frame callback right before `SwapBuffers` commits (the swap's implicit commit carries the request).
   - `ready()` reports `false` from the moment a callback is requested until the compositor fires it.
   - `drawSingleFrame`'s `decideRepaint(visible, ready, checkDirtyAndClear)` only calls `CheckDirtyAndClear` (and thus only repaints) when the window is both visible **and** presentable — so a pending dirty flag is preserved, not lost, while waiting.
   - `markReady()` is called on window focus-gain as a backstop, in case a compositor discards a pending callback instead of firing it while hidden.
   - Off Wayland (or on wasm), `noGate` is a no-op that always reports `ready() == true`, so behavior is unchanged.

## Verification performed

- Applied cleanly to `develop` (`git apply --check`).
- Built successfully for: default (no tags), `-tags wayland` (Linux, with `wayland-client` available), and `GOOS=js GOARCH=wasm`.
- `go vet` clean under default and `-tags wayland`.
- New tests (`present_test.go`) pass under both default and `-tags wayland` builds.
- Full existing `internal/driver/glfw` test suite: same 3 pre-existing failures on patched and unpatched `develop` (headless-environment clipboard/timing issues, e.g. `TestWindow_ClipboardCopy_DisabledEntry`) — **no regressions introduced**.
- Empirically confirmed, on a real GNOME/Mutter Wayland session, that a **plain untagged `go build`** already auto-detects and runs under native Wayland via GLFW 3.4's platform auto-selection (`glfw.GetPlatform() == PlatformWayland`), which is the basis of Gap 1 below.

## What's correct

- `decideRepaint`'s short-circuit order (`visible && ready && checkDirtyAndClear()`) is the crux of the fix and is correct: dirty is only cleared when a repaint will actually happen, so a deferred frame isn't lost while waiting for presentability. Covered by a solid table-driven test.
- The `wl_surface.frame` request/commit sequencing (`arm()` before `SwapBuffers()`) matches the correct Wayland idiom — a frame callback only fires after a subsequent commit, and `SwapBuffers` supplies that commit.
- Re-arming destroys any still-pending, never-fired callback first, avoiding a `wl_callback` proxy leak in the hidden→focus-backstop→redraw path.
- All gate reads/writes and the `frame_done` C callback execute on the single OS-locked main thread (`runtime.LockOSThread()` + the single-threaded `runGL()` loop), so the unsynchronized `int ready` / `wl_callback *cb` fields are not actually racy despite no atomics/mutex.
- The cross-cgo-package `unsafe.Pointer` handoff in `windowSurface` (passing a `*C.struct_wl_surface` from the `go-gl/glfw` package into `internal/driver/glfw`'s own cgo domain) is safe and matches an existing pattern already used in this codebase (`window_x11wayland.go:23`, `window_windows.go:65`).
- Build tags on `present_wayland.go` / `present_other.go` are proper logical complements — confirmed by building all three target configurations without gaps or conflicts.

## Gaps found

### Gap 1 (significant): the frame-callback gate is compile-time opt-in; Wayland detection isn't

There are two different "are we on Wayland?" signals in the codebase, and the patch's two fixes each check a different one:

| Fix | Mechanism | Signal checked |
|---|---|---|
| `SwapInterval(0)` | stops the literal hang | **runtime** — `build.IsWayland`, set by `loop_desktop.go` whenever `glfw.GetPlatform() == PlatformWayland` |
| `presentGate` (real impl) | avoids wasteful redraws while hidden | **compile-time** — the `wayland` Go build tag on `present_wayland.go` |

GLFW 3.4 auto-detects the display platform at runtime and needs no build tag to run under native Wayland — confirmed empirically above. That means a plain `go build .` (no tags), which is how most Fyne apps are likely built, already runs under real Wayland on a Wayland desktop, and `build.IsWayland` correctly becomes `true` for it. `SwapInterval(0)` therefore activates for it.

But `present_wayland.go` is marked `//go:build wayland`, so it is only *compiled into the binary at all* when someone explicitly passes `-tags wayland`. If they don't, that file doesn't exist in the resulting executable — `present_other.go`'s permanent `noGate` stub is compiled in instead, and no runtime flag can bring back code that was never compiled in.

**Net effect:** two developers running identical apps on the identical Wayland desktop get different outcomes:
- `go build .` (no tags): gets `SwapInterval(0)` (likely enough to stop the literal freeze) but never gets the frame-callback gate — permanently `noGate`, always "ready," so it keeps redrawing on the internal 60Hz ticker regardless of compositor presentability.
- `go build -tags wayland .`: gets both fixes.

Only the second group gets the complete behavior the maintainers asked for. The first group — likely the majority, since GLFW 3.4 no longer requires the tag to actually run under Wayland — gets a partial fix.

**Suggested resolution:** make `newPresentGate()` dispatch on the runtime `build.IsWayland` flag rather than relying on the build tag to select the implementation, e.g. always compile the Wayland cgo gate on Linux/BSD (GLFW's own Wayland backend, and its `-lwayland-client` link line, are already compiled in unconditionally for the default build per `go-gl/glfw`'s `build.go`), and have `newPresentGate()` return `noGate{}` vs. the real tracker based on `build.IsWayland` at call time instead of at compile time.

### Gap 2 (minor, consequence of Gap 1): `SwapInterval(0)` has no compensating throttle in the default build

`SwapInterval(0)` is applied unconditionally for the whole Wayland session (visible or hidden), not just while hidden. In the `-tags wayland` build this is fine because the frame-callback gate reintroduces compositor-accurate pacing. In the default/auto-detect build (Gap 1), there's no such compensation — visible windows redraw un-paced by the compositor, relying solely on the internal 60Hz software ticker, which can drift from the display's actual refresh behavior (potential stutter/uneven pacing vs. the previous blocking-vsync behavior). Fixing Gap 1 fixes this too.

### Gap 3 (minor): focus-gain backstop isn't scoped to Wayland

In `processFocused` (`window.go`), `w.frame.markReady(); w.canvas.SetDirty()` runs unconditionally on every window focus-gain event, on every platform (Windows/macOS/X11/wasm included), not just Wayland. `markReady()` is a harmless no-op off-Wayland, but `SetDirty()` forces a full canvas repaint on every alt-tab/focus event everywhere, even though the backstop only exists to correct a Wayland-specific edge case (a compositor discarding a pending frame callback instead of firing it while hidden). Cheap to set, but avoidable extra work on platforms that never needed it.

**Suggested resolution:** gate with `if build.IsWayland { w.frame.markReady(); w.canvas.SetDirty() }`.

## Bottom line

The mechanism is sound and well-tested where it applies, and the literal hang from the issue should be fixed for both the tagged repro case and (via the runtime-gated `SwapInterval(0)`) the default-build case. However, the more complete "don't redraw a backgrounded window at all" behavior — the part the maintainers actually asked for — only reaches users who explicitly build with `-tags wayland`, which is likely a minority now that GLFW 3.4 auto-detects Wayland without it. Recommend resolving Gap 1 before merging so the fix's coverage matches its runtime detection.
