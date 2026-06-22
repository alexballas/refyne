# go2tv xfilepicker Idle Blank List

## Issue Report

`go2tv` embeds `xfilepicker` for file selection. When the file picker is left open but idle in the background, or on another GNOME workspace, the file grid can return in a visually blank state.

Observed behavior:

- The file picker window remains responsive.
- The sidebar, breadcrumbs, scrollbars, and footer remain visible.
- The file grid area appears empty.
- Clicking or marquee-selecting inside the blank file area makes the files immediately reappear.

Expected behavior:

- Files should remain visible after the picker has been idle or backgrounded.

The important clue is that clicking the blank area can select a file and repopulate the view. That means the directory data and hit targets are still present. The failure is in rendering/cache state, not in file listing.

## Previous Attempt

An earlier attempted fix added `_glfwInputWindowDamage(window)` in GLFW Wayland `surfaceHandleEnter`.

Rationale:

- The issue appeared when returning to a GNOME workspace/window.
- A missing Wayland damage/repaint signal could plausibly leave stale pixels on screen.

Result:

- The issue still reproduced after an overnight idle test.
- The fix was reverted.

Why it was not sufficient:

- `internal/driver/glfw/window.go` already marks the frame ready and sets the canvas dirty on focus gain.
- Another Wayland repaint trigger does not address renderer/cache state being stale or evicted after a long idle period.
- The symptom recovers through a picker-level refresh, not through a compositor-only event.

## Deeper Analysis

The xfilepicker click recovery path is direct:

- `fileDialog.Select` calls `fileList.refresh()`.
- In grid mode, `fileList.refresh()` calls `f.grid.Refresh()`.
- `GridWrap.Refresh()` calls `updateGrid(false)`.
- `updateGrid(false)` reruns `UpdateItem` for all visible grid cells, repopulating labels and thumbnails.

Relevant paths:

- `/home/alex/test/xfilepicker/dialog/multiselect.go`
- `/home/alex/test/xfilepicker/dialog/file_list.go`
- `/home/alex/test/refyne/widget/gridwrap.go`

The stronger root cause candidate was the render/cache timestamp path:

- `internal/cache/base.go` uses `coarseTimestamp` in `setAlive()`.
- `coarseTimestamp` was only refreshed by `cache.Clean()`.
- `cache.Clean()` runs at the end of a frame.
- After long idle, a new frame could walk visible widgets and touch renderer/canvas caches before `coarseTimestamp` had been refreshed.
- Those touched cache entries could be marked alive using an old timestamp, leaving their expiry in the past.
- The same frame can then call `cache.Clean(true)` and destroy renderers/canvases that were just used.

This matches the screenshots:

- Non-virtualized chrome remains visible.
- The virtualized file grid area is blank.
- Clicking forces a full `GridWrap.Refresh()`, which rebuilds the visible item state.

## Current Fix

The current fix introduces an explicit frame-start timestamp refresh:

- `internal/cache/base.go` now exposes `cache.BeginFrame()`.
- `internal/driver/glfw/loop.go` calls `cache.BeginFrame()` at the start of `drawSingleFrame()`, before resize handling, repaint walks, and cache keepalive walks.

This keeps `setAlive()` using a current frame timestamp before any render/cache hit can update expiry. `cache.Clean()` still refreshes the timestamp as well, but it is no longer the first timestamp update after a long idle.

Changed files:

- `/home/alex/test/refyne/internal/cache/base.go`
- `/home/alex/test/refyne/internal/cache/base_test.go`
- `/home/alex/test/refyne/internal/driver/glfw/loop.go`

## Regression Coverage

Added regression:

- `TestBeginFrameKeepsTouchedRendererAliveAfterIdle`

The test simulates:

1. A renderer is cached.
2. Wall time advances beyond the cache TTL while the coarse timestamp remains stale.
3. A new frame begins with `BeginFrame()`.
4. The renderer is touched.
5. `Clean(true)` runs.
6. The renderer must not be destroyed.

This covers the suspected root cause deterministically without requiring GNOME, Wayland, real GL, or overnight idle timing.

## Verification

Passing checks:

- `gofumpt -l -w .`
- `go test ./internal/cache`
- `go test ./internal/cache -run TestBeginFrameKeepsTouchedRendererAliveAfterIdle -count=1 -v`
- `go test ./internal/driver/glfw -run 'TestDecideRepaint|TestHasReadyDirtyWindow|TestNeedsFrameTickWithAnimation|TestResizeCoalescing|TestNoGateAlwaysReady|TestNewPresentGateReadyByDefault' -count=1`
- `git diff --check`

Known unrelated local test noise:

- Full `go test ./internal/driver/glfw` failed on a menu golden-image mismatch and a clipboard test reading existing system clipboard contents.
- `go test ./internal/driver/mobile` failed in `Test_canvas_Dragged`; the mobile frame loop was not changed in the final patch.

## Emulation Options

Deterministic emulation:

- Use the new cache regression test. It models the stale timestamp cache-expiry hazard directly.

Manual symptom emulation:

```sh
FYNE_CACHE=2s ../go2tv
```

Then open the file picker, leave it idle/backgrounded long enough for cache cleanup to become relevant, and return to the picker. Compare current code against a build without `cache.BeginFrame()`.

High-fidelity automated emulation would require a GL-backed fixture app using `GridWrap`, shortened cache duration, idle timing, and screenshot/pixel capture. That would test the user-visible blank-grid symptom, but it would be more fragile than the cache-level regression.
