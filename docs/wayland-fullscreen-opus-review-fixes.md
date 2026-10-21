# Wayland fullscreen: Opus 5.5 review and fixes

Completed October 22, 2026 by Claude Opus 5.5 (`claude-opus-5-5`), high effort, via Claude Code. The same agent reviewed the code, implemented every fix and ran the tests over two rounds:

1. Review of the whole working tree, including the untracked fullscreen implementation and tests, with fixes for F1–F4.
2. A parent-requested follow-up that fixed F5 and F6, the F3 hidden-exit case, and test-hygiene issues.

Nothing was committed or staged.

Claude Code session: `16b5c478-e1bc-4c70-aee7-363079d2961d`. Both rounds used this same Opus 5.5 session at high effort.

Runtime checks used only private headless GNOME Shell sessions with software rendering, or Xvfb, each with a private D-Bus session and a private `XDG_RUNTIME_DIR`. Nothing ran against the real desktop. Nothing was validated on real hardware, fractional scaling or direct scanout.

## 1. Findings, by severity

Every finding below is now fixed. Each has a runtime reproduction, except where marked as static inference.

| ID | Severity | Origin | Summary |
|---|---|---|---|
| F6 | High | Pre-existing (`3128ff658`) | Visible fixed-size windows recurse between `fitContent` and `processResized` until the stack overflows |
| F1 | Medium | New (Wayland Hide in fullscreen) | Exiting fullscreen while hidden leaves the canvas at fullscreen size |
| F2 | Medium (crash risk) | New | Hide in the same turn as a fullscreen request destroys mapped shadow subsurfaces under a fullscreen toplevel |
| F3 | Medium | Pre-existing | A window first shown fullscreen exits to GLFW's grey fallback bars with no Fyne title bar |
| F4 | Medium (latent) | Pre-existing | The CSD shadow can send negative `wp_viewport` destinations, a fatal protocol error |
| F5 | Low–Medium | Pre-existing | Queued exit → enter → exit requests leave the window windowed at monitor size |
| F7 | Low | Pre-existing (upstream GLFW) | Non-resizable Wayland windows keep their creation-time xdg min/max size after a client resize |

### F6 — fixed-size recursion

**Repro:** `fatal error: stack overflow` on `Show`, under both Wayland and X11.

**Cause:** for fixed-size windows, `processResized` (`window.go:322`) calls `fitContent`. Since `3128ff658`, `fitContent` (`window_desktop.go:343-347`) calls `SetSize` and `processResized` to report the size. A hidden window returns early, so the existing hidden-window tests never saw the loop. Any visible window recursed forever.

### F1 — hidden exit leaves a fullscreen canvas

**Repro:** fullscreen → Hide → `SetFullScreen(false)` → Show left the native window at 640x392 but the canvas at 1024x768.

**Cause:** a hidden window has no xdg role, so GLFW resizes it without a size callback. The new role's 0x0 configure then reports no change, so Fyne never learns the windowed size.

### F2 — subsurfaces destroyed under a fullscreen toplevel

**Protocol evidence:** `SetFullScreen(true); Hide()` sent `set_fullscreen(nil)`, then destroyed eight still-mapped shadow subsurfaces, and only then sent `xdg_toplevel.destroy`. This is the state the earlier Mutter scanout-crash fix avoids. Only the precondition is verified; whether it actually crashes Mutter needs hardware and scanout.

### F3 — initial-fullscreen exit decorations

**Repro:** after exit the window had `Decorated=1` and no decoration, and four 1x1-shm fallback edges were created after `unset_fullscreen`.

**Cause:** `setupWaylandDecorations` deferred the title bar while fullscreen, and nothing installed it later.

**Hidden-exit variant, also reproduced:** if Show followed a hidden exit in the same turn, the window restored a 640x360 outer size with a smaller content area. The visible exit and the reference window give 640x392.

### F4 — negative shadow viewport sizes

**Repro:** a 9x9 window still awaiting its fullscreen configure sent `set_destination(-15, 12)`. Mutter replied with `wl_display.error … all values must be either positive or -1`.

**Static inference:** a CSD window resized below about 24 logical pixels wide could hit the same error.

### F5 — queued toggles restore monitor size

**Repro:** from fullscreen, `SetFullScreen(false/true/false)` in one turn produced these configures:

1. 640x392 windowed;
2. 1024x768 fullscreen;
3. **1024x768 windowed.**

**Cause, in two parts:**

- Mutter saved its restore rectangle on the second request, while its window rectangle was still fullscreen.
- GLFW cleared the requested restore size on the first, stale windowed configure, so it had nothing to apply to the third.

Fyne also recorded the intervening fullscreen-sized configure as its windowed size, because `w.fullScreen` was already false.

### F7 — stale limits for non-resizable windows

**Repro, seen while verifying F6:**

- The window stayed pinned to `set_min_size/set_max_size(400, 300)` after growing to 600x432.
- On exiting fullscreen Mutter suggested 400x300 twice, and fixed-size enforcement pushed back each time.

**Cause:** `glfwSetWindowSizeLimits` ignores non-resizable windows, and the Wayland backend never re-pinned their limits on a client resize.

### Latest changes with no defects found

These were reviewed by trace and, where applicable, at runtime:

- **Monitor removal:**
  - outputs are pruned before their proxy is destroyed;
  - an automatic window moves to a surviving monitor;
  - explicit fullscreen exits without `setWindowPos`.
- **Disconnect callback:** `SetFullScreen(false)` runs re-entrantly inside `PollEvents` without touching the freed monitor.
- **Output bookkeeping:**
  - late output reconciliation;
  - duplicate-enter deduplication;
  - keeping the last integer scale while no output is entered.
- **`RescaleContext`:** no longer overwrites the windowed bounds.
- **Explicit retargeting:** no intermediate unset.
- **Idempotent automatic fullscreen.**
- **Platform guards.**

One hypothesis remains, not a confirmed defect. If the compositor leaves fullscreen on its own, a later `SetFullScreen(true)` from the app is a no-op. Upstream GLFW has the same blind spot for explicit monitors.

## 2. Changes

Vendored C. The hash was regenerated after every edit and `upstreamTreeSHA` is now `cd172af7c513e5d3d9470de00ff9148f9d516a86`. `internal/glfw/VENDORING.md` documents each change.

- **`wl_window.c:3238` `setWindowedSizeWayland`** (F1) — when the windowed size changes with no xdg/libdecor role, it reports the size through `_glfwInputWindowSize`, as a configure would.
- **`wl_window.c:1199` `destroyShellObjects`** (F2) — destroys the decoration object, toplevel and xdg surface first, then the shadow and fallback subsurfaces. Hide and Close therefore unmap the window before any subsurface is destroyed.
- **`wl_refyne.c`** (F4) — clamps the shadow's top-width and side-height viewport destinations to at least 1, in both shadow creation and shadow resize.
- **F5 restore correlation:**
  - `wl_platform.h:396` adds `restore.sync`.
  - `wl_window.c:3306-3308`: each exit request with a live toplevel stores the requested size and issues a `wl_display.sync`, replacing any older one.
  - `wl_window.c:756`: windowed (neither fullscreen nor maximized) configures use the requested restore size. glfwSetWindowMonitor(NULL, …, width, height) explicitly requests it, and a floating configure size is only a suggestion. Maximized and fullscreen configures keep the compositor's size.
  - `wl_window.c:828`: a windowed configure clears the restore size only after the sync is done, so stale answers to earlier queued requests can't consume it.
  - `wl_window.c:698` `restoreSyncHandleDone`: if a windowed configure has already been applied when the sync completes, the restore is complete and is cleared. Otherwise the next windowed configure completes it. This also clears a restore the compositor never answered, for example when it refused fullscreen.
  - `wl_window.c:2919`: the callback is destroyed with the window.
  - No EGL or window size changes happen before the exit configure.
- **`wl_window.c:3006`** (F7) — `_glfwSetWindowSizeWayland` re-pins the min/max size of a non-resizable window after a client resize.

Go:

- **`window.go:326`, `window_desktop.go:82-84, 345-347`** (F6) — `fitContent` sets `fittingFixedSize` while it reports the fixed size through `processResized`, and `processResized` skips its nested `fitContent` while that flag is set.
  - Every other path keeps its behavior: configure-driven enforcement, content-minimum growth, canvas updates, and recording the windowed bounds.
  - `window_wasm.go` declares the shared field.
- **F3, `decorations_wayland.go:134` and `:169`:**
  - When a window is shown fullscreen, `setupWaylandDecorations` records that the title bar was deferred and the negotiated decoration mode.
  - `setupWaylandDecorationsForFullScreenExit` runs from `SetFullScreen(false)` before the exit request, whether the window is visible or hidden. For CSD it disables GLFW's decorated flag, so no fallback bars appear. It then installs the title bar without resizing and grows the restore height by the title bar's height.
  - Shared helpers `newWaylandDecoration`/`finishWaylandDecoration`, and a no-op stub in `decorations_other.go`.

Tests. All Wayland tests are gated on `REFYNE_WAYLAND_FULLSCREEN_TEST=1`. Values are captured inside `runOnMain` and asserted on the test goroutine, and monitors are nil-checked before use.

- **`TestWindow_FixedSizeShowResizeAndFullScreen`** (`window_notdarwin_notwasm_test.go:127`) — a fixed-size window through `Show`, `Resize`, `SetContent` with a larger minimum, then fullscreen enter and exit.
  - It asserts the native size, canvas, content size and windowed restore bounds at each step.
  - The fullscreen part runs on X11, and on Wayland only when opted in.
  - It hits the stack overflow without the guard.
- **`TestWaylandFullScreenQueuedToggles`** (`fullscreen_wayland_test.go:272`) — three cycles of exit/enter/exit and enter/exit/enter, each queued within one main-thread turn, plus plain toggles.
  - It asserts the final GLFW and Fyne state, the native size, the canvas and the restore bounds.
  - It fails without the F5 fix: windowed at `[1024 768]`.
- **`TestWaylandInitialFullScreenExitDecorations`** (`:183`, rewritten) — three exits: visible; hidden, then Show in a later turn; hidden, then Show in the same turn.
  - Each must end with Fyne's title bar, `Decorated=false`, and the outer size and content area of an equivalent window shown windowed.
  - The same-turn hidden case fails with the previous visible-only fix (640x360 outer size).
- **`TestWaylandFullScreenHideTransitions`** (`:111`, round 1) — fullscreen+Hide and exit+Hide in the same turn, and exit while hidden. It fails without the F1 fix.

## 3. Tests run

| Check | Result |
|---|---|
| Isolated two-monitor GNOME: all fullscreen tests, fixed-size, `TestWindow_MinSize_Fixed`, the resize/presentation/decoration-bounds tests and `TestWindow_SetFullScreen` (log `/tmp/refyne-fullscreen-opus-residual-suite.log`) | 24/24 PASS, no protocol errors |
| Isolated GNOME, broad regex `(Decoration\|Wayland\|Shadow\|Hide\|Show\|Close\|Fixed\|Resize\|Hoverable\|MinSize\|FullScreen)` (log `/tmp/refyne-fullscreen-opus-residual-broad.log`) | 56 PASS, plus one markup test that failed only because the run started in `/tmp` and couldn't find `testdata`. Rerun from the package directory (log `/tmp/refyne-fullscreen-opus-hover.log`): PASS |
| Xvfb default build: fixed-size, `TestWindow_MinSize_Fixed`, `TestWindow_HandleOutsideHoverableObject`, `TestWindow_SetFullScreen`, `TestApplyFullScreenWithoutMonitor` (log `/tmp/refyne-opus-xvfb-residual.log`) | PASS. Wayland-only tests and the two-monitor test skip |
| Negative controls (logs `…-opus-queued-before`, `…-opus-deco3-nofix`, `/tmp/refyne-opus-xvfb-fixed-nofix.log`, plus round 1's `…-opus-hide-nofix`, `…-opus-deco-nofix`) | Each fails as expected: F5 restores monitor size, F3 hidden case gives 640x360, F6 overflows the stack |
| F5 protocol trace (`/tmp/refyne-fullscreen-opus-queued-after.log`) | `unset`/`set`/`unset`, then configures `(640,392)`, `(1024,768)` fullscreen and a stale `(1024,768)` windowed. The client applies geometry 640x392 at the last one. The exit sync's `done` follows that configure |
| `go build` of `./cmd/fullscreen` with `-tags wayland`, `-tags x11` and default; `GOOS=js GOARCH=wasm go build ./internal/driver/glfw` | OK |
| `go vet` of the driver with default, `x11` and `wayland` tags, and of `./internal/glfw` | OK |
| `gofumpt -l -w .` and `git diff --check` | Clean |

Round 1 logs remain under `/tmp/refyne-fullscreen-opus-*.log`. In those, the hidden fullscreen-exit probe (`/tmp/refyne-fullscreen-opus-probe4.log`), the queued-toggle probe (`/tmp/refyne-fullscreen-opus-probe3.log`) and the first stack overflow (`/tmp/refyne-fullscreen-opus-probe2.log`) are the original reproductions.

`go build -tags test_web_driver ./internal/driver/glfw` fails in `internal/cache` (`undefined: textureInfo`). That predates these changes and has nothing to do with the driver.

## 4. Harness

`/tmp/refyne-run-two-monitors.py` now:

- re-executes itself under its own `dbus-run-session`, with a private outer `XDG_RUNTIME_DIR` and with `DBUS_SESSION_BUS_ADDRESS`/display variables removed, whatever the caller does;
- sets `GIO_USE_VOLUME_MONITOR=unix` and the memory GSettings backend;
- deletes the private runtime directory afterwards, retrying while a private document-portal mount releases;
- accepts `REFYNE_HARNESS_TEST_BINARY` to run alternative test binaries.

After the change, keyring used the private runtime directory and no USB device was enumerated. The original file was saved as `/tmp/refyne-run-two-monitors.py.orig`. Usage:

```
timeout 120s python3 /tmp/refyne-run-two-monitors.py LABEL 'REGEX'
```

Run it from `internal/driver/glfw` so markup tests find `testdata`.

## 5. Remaining limitations

- **F2:** only the protocol precondition is verified. The crash itself needs real GPU and scanout hardware.
- **F5 relies on two assumptions:**
  - Mutter sends exit configures while processing the request, so they arrive before the sync's `done`. The trace confirms this for Mutter. Delayed or coalesced configure ordering on other compositors was not tested, so the review does not establish behavior for those orderings.
  - Tiled windows on xdg_wm_base v1 receive a maximized state. GLFW binds version 1, which has no tiled states. If a compositor reports a v1 tile as a plain floating configure, a fullscreen exit returns to the requested size instead of the tile.
- **F3:** while the exit configure is pending, an initially fullscreen window briefly shows its new title bar inside the fullscreen canvas. If such a window has no recorded windowed size (`w.width == 0`), the existing `doApplyFullScreen` fallback derives the restore size from the canvas, as before.
- **Static inference only:**
  - removing the monitor of an initially fullscreen window while it is explicitly fullscreen;
  - the sub-24-pixel CSD resize case of F4.
- **Not validated:** physical unplug, removal of the last output, mixed or fractional DPI, direct scanout, libdecor (disabled in this driver), and non-Mutter compositors.
- **Content minimums:** a root `canvas.Rectangle`'s `SetMinSize` followed by `Refresh` did not grow either fixed or non-fixed windows in this session. That isn't covered by this change; the regression test grows the minimum through `SetContent` instead.

## 6. Independent parent verification

After Opus returned its final update, the parent reviewed the changes and independently:

- rebuilt the driver tests and passed all 24 targeted tests in a fresh private headless GNOME session (`/tmp/refyne-fullscreen-parent-opus-verified.log`);
- rebuilt the default driver tests and passed five Xvfb tests, including fixed-size Show/Resize/fullscreen, fixed minimum size, hover, and fullscreen (`/tmp/refyne-parent-xvfb.log`);
- built the proof of concept with default, Wayland and X11 tags;
- recomputed the vendored tree hash without writing files or the index and confirmed `cd172af7c513e5d3d9470de00ff9148f9d516a86`;
- ran `gofumpt -l -w .` and `git diff --check`, both clean;
- confirmed the unrelated accessibility plan's SHA-256 is unchanged.

The parent verification uses software rendering and does not extend the hardware or compositor coverage described above.
