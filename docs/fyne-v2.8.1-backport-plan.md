# Fyne v2.8.1 Selective Backport Plan

**Status:** implemented; automated verification complete, with physical-device
and desktop-hardware smoke tests pending

**Created:** 2026-08-26

**Refyne baseline:** `c2b8de6ec8ae1943fe4a9f550438645180f98665` (`v2.8.103`)

**Fyne source baseline:** `v2.8.1`, commit `3dc06f47`

**Go2TV baseline:** `17d00723286c4b977bc26e063e8375adc745af41`

**Primary consumer:** `/home/alex/test/go2tv`

**Related consumer:** `/home/alex/test/xfilepicker`

Upstream release: [Fyne v2.8.1](https://github.com/fyne-io/fyne/releases/tag/v2.8.1)

## Decision

Selectively adapt the v2.8.1 changes that improve Refyne's frame hot paths,
desktop robustness, URI handling, Android input, mobile interaction, and mixed-font
rendering. Do not merge or cherry-pick the release wholesale. Refyne has its own
module path, Go version, renderer/cache work, mobile changes, and vendored GLFW
patches, so upstream diffs are design references rather than directly applicable
commits.

The work should land in independently testable phases. The first releasable batch
is performance plus low-risk desktop robustness. URI and font changes are
deliberately isolated because they alter public serialization or rendered output.

## Expected Value for Go2TV

| Area | Go2TV relevance | Expected result | Risk |
| --- | --- | --- | --- |
| Frame/layout hot paths | High | Faster queue, settings, picker, and label-heavy redraws; much less GC | Medium |
| Desktop input/monitor guards | Medium-high | No crash on unknown Wayland keys or monitor disconnect races | Low |
| Pointer/focus fixes | Medium | More reliable touchscreens and empty collection clicks | Low-medium |
| URI rewrite | High | Correct special-character, Windows, URN, query, and fragment handling | High |
| Android hardware keyboard | High for mobile | No duplicate printable characters; forward Delete works | Medium |
| Mobile Entry/file dialog fixes | Medium | Correct cursor placement and better file-dialog spacing | Low |
| Mixed-font/emoji shaping | Medium | Consistent baselines and intact keycap emoji | High |

## Source Changes to Adapt

The main upstream references are:

- [`b388ca36`](https://github.com/fyne-io/fyne/commit/b388ca362e264908b20a8542ae31cb2e1b9a3341),
  PR [#6465](https://github.com/fyne-io/fyne/pull/6465): per-frame geometry,
  renderer lookup, and benchmark changes.
- [`bbfedde3`](https://github.com/fyne-io/fyne/commit/bbfedde38689146507f5fb12bb0c7e6ab5e70a52):
  canvas attachment allocation reduction.
- [`b62a5975`](https://github.com/fyne-io/fyne/commit/b62a59751bca1dc7668d8f96f1c450654178704c),
  PR [#6453](https://github.com/fyne-io/fyne/pull/6453): cached Label renderer
  object slices.
- [`af8dc998`](https://github.com/fyne-io/fyne/commit/af8dc99891f17fa9a2deb9e7768d35e47c251594),
  PR [#6454](https://github.com/fyne-io/fyne/pull/6454): cached object slices in
  Navigation and mobile renderers.
- [`c2a06f3f`](https://github.com/fyne-io/fyne/commit/c2a06f3fd8958ae023e340ec1f1008917ad8b8c5): no-op
  `Container.Move` early return.
- [`d9cbbe99`](https://github.com/fyne-io/fyne/commit/d9cbbe998afe3d28372a94d2bbba090e62a994d0),
  PR [#6445](https://github.com/fyne-io/fyne/pull/6445): unknown GLFW
  scancode guard.
- [`50a538e5`](https://github.com/fyne-io/fyne/commit/50a538e59312ec586294efb92cb1d8a6d047cc51),
  PR [#6472](https://github.com/fyne-io/fyne/pull/6472): nil monitor video modes.
- [`4595bd57`](https://github.com/fyne-io/fyne/commit/4595bd5704abf217cb12b273ede2039673654002):
  pending pointer processing before touchscreen taps.
- [`a08b7058`](https://github.com/fyne-io/fyne/commit/a08b7058358702e93240f86917064080c9058886),
  PR [#6432](https://github.com/fyne-io/fyne/pull/6432): focus handling over
  empty collection space.
- [`68846c9c`](https://github.com/fyne-io/fyne/commit/68846c9cc3e87698467ae3469e1025e40c1f38d0) and the later
  v2.8.1 URI commits: `net/url`, file paths, URNs, query/fragment, escaping, and
  host validation.
- [`105c372b`](https://github.com/fyne-io/fyne/commit/105c372b62ecd8585ff5995f9218d8e312b68eea):
  Android hardware keyboard handling and forward Delete.
- [`d0068ace`](https://github.com/fyne-io/fyne/commit/d0068ace1c45c01f1556b95c94f14a36aea65997),
  PR [#6307](https://github.com/fyne-io/fyne/pull/6307), and
  [`a02156fa`](https://github.com/fyne-io/fyne/commit/a02156fae612874309c7304518a2046d0f523909):
  mobile Entry touch/selection fixes.
- [`fc099691`](https://github.com/fyne-io/fyne/commit/fc0996919a2650a238489877e6cf301b52ed0654),
  PR [#6451](https://github.com/fyne-io/fyne/pull/6451): padded file dialog.
- [`ddff50ae`](https://github.com/fyne-io/fyne/commit/ddff50ae9fcad96b0c9f29a5b51592a88a34678f),
  PR [#6448](https://github.com/fyne-io/fyne/pull/6448), and
  [`82fca11c`](https://github.com/fyne-io/fyne/commit/82fca11cfac789b040e4fde10fa98d0690adeaec):
  shared mixed-font baseline and emoji-sequence handling.

Use the final `v2.8.1` versions of affected files when reconstructing a change;
several features were spread across many small commits.

## Related Local Design State

Read these before implementation because they describe Refyne behavior that an
upstream-looking patch could accidentally undo:

- [`../PERFORMANCE_REVIEW.md`](../PERFORMANCE_REVIEW.md): implemented Refyne
  optimizations and remaining performance proposals.
- [`go2tv-xfilepicker-idle-blank-list.md`](go2tv-xfilepicker-idle-blank-list.md):
  the frame-start `cache.BeginFrame` fix that keeps a backgrounded xfilepicker
  grid alive. Phase 1 must preserve its ordering and regression test.
- [`perf-text-shaper-reuse.md`](perf-text-shaper-reuse.md): text benchmark and
  profiling guidance. Its original implementation status is stale; Refyne now
  already pools the shaper and segmenter.
- [`../internal/glfw/UPSTREAM_ADOPTION_PLAN.md`](../internal/glfw/UPSTREAM_ADOPTION_PLAN.md)
  and [`../internal/glfw/VENDORING.md`](../internal/glfw/VENDORING.md): vendored
  GLFW decisions and Refyne-specific Wayland behavior.
- [`superpowers/specs/2026-05-28-seekable-storage-reads-design.md`](superpowers/specs/2026-05-28-seekable-storage-reads-design.md):
  seekable URI reader contracts that Phase 3 must retain.
- [`mobile-manifest-plist-go2tv-plan.md`](mobile-manifest-plist-go2tv-plan.md):
  separate mobile capability work. Do not expand this backport effort into that
  plan without an explicit scope decision.

## Constraints and Guardrails

- Keep Refyne's module path `github.com/alexballas/refyne/v2` everywhere.
- Keep the `go 1.19` directive unless a separate compatibility decision is made.
- Do not use Go 1.21's built-in `clear`; upstream commit `766c0e6e` is not
  compatible with Refyne's declared Go version.
- Do not port the `GridWrap` clearing loop from the tag. It had a merge
  inefficiency fixed immediately after v2.8.1 by upstream commit
  [`7b91d865`](https://github.com/fyne-io/fyne/commit/7b91d865a1865635e6a38a75b81f76828b3f1f6a).
- Preserve Refyne's `cache.BeginFrame` coarse clock and existing cache, SVG,
  painter, list, run-loop, mobile intent, seekable storage, and GLFW changes.
- Preserve the vendored GLFW patches documented in
  `internal/glfw/VENDORING.md`; none of this work authorizes a GLFW vendor
  refresh.
- Add behavior tests before or with each fix. Allocation targets belong in
  benchmarks, not brittle unit-test assertions.
- Run `gofumpt -l -w .` after every Refyne code phase, as required by
  `AGENTS.md`.
- Keep commits phase-scoped and revertible. Do not squash the high-risk URI or
  font work into the initial performance commit.
- Do not leave temporary `replace` directives, `go.work` files, benchmark
  outputs, APKs, or built tools in a repository commit.

## Phase 0: Re-establish Baselines

### Tasks

- [ ] Confirm Refyne, Go2TV, and xfilepicker worktrees are clean or record all
  pre-existing user changes before editing.
- [ ] Create `changes/fyne-2.8.1-backports` from the intended Refyne base.
- [ ] Re-read the v2.8.1 release diff against the exact Refyne base; do not
  assume the hashes above remain the current branch heads.
- [ ] Add an adapted version of upstream's roughly 730-node dashboard benchmark
  to `internal/driver/glfw/frame_bench_test.go`.
- [ ] Call `cache.BeginFrame()` during benchmark setup before the timed loop so
  Refyne's cache hits use the same coarse timestamp as a production frame.
- [ ] Capture at least 10 samples of both benchmarks with `-benchmem`; use
  `benchstat` for comparisons if available.
- [ ] Record toolchain, OS, CPU, commit, and whether a display server was used
  next to the benchmark result in the eventual change description.

Suggested benchmark command:

```bash
go test ./internal/driver/glfw -run '^$' \
  -bench '^BenchmarkFrame(EnsureMinSize|WalkTrees)$' -benchmem -count=10
```

### Existing Measurement

Measured on Go `1.26.7`, Linux/amd64, AMD Ryzen 9 7900, Refyne
`c2b8de6ec`:

| Benchmark | Current median | Current allocation rate | Adapted prototype median | Adapted allocation rate |
| --- | ---: | ---: | ---: | ---: |
| `FrameEnsureMinSize` | ~122.4 us/op | 3,560 B, 309 allocs/op | ~108.4 us/op | 144 B, 2 allocs/op |
| `FrameWalkTrees` | ~29.84 us/op | 2,064 B, 122 allocs/op | ~20.53 us/op | 144 B, 2 allocs/op |

These are directional, not release promises. Refyne already has coarse cache
timestamps and other optimizations, so its incremental time gain is smaller
than upstream's headline result.

### Exit Gate

- Benchmark is stable enough to compare before/after samples.
- Baseline package tests pass, or environment-sensitive pre-existing failures
  are written down before implementation.

## Phase 1: Core Frame-Path Improvements

This phase is the highest-value first release candidate.

### 1.1 Allocation-free size comparisons

- [ ] Add `internal.MaxSizes(a, b fyne.Size)` and
  `internal.MinSizes(a, b fyne.Size)` using component-wise `fyne.Max` and
  `fyne.Min`.
- [ ] Add exact equivalence tests, including zero and negative components.
- [ ] Replace `Size.Max`/`Size.Min` in the upstream-identified layout and frame
  paths. Start with container, layout, dialog, collection, popup, scroller,
  window, and test-driver call sites from upstream `b388ca36`.
- [ ] Inline the same component-wise calculation in root `container.go`; the
  root package cannot import `internal` without an import cycle.
- [ ] Re-run the audit after editing. Do not mechanically change unrelated
  public API examples or tests merely for consistency.

Why: the existing methods accept the `Vector2` interface, which can box a
`Size` and escape on hot layout paths. The helper keeps the concrete type.

Caveat: preserve operand order and exact `fyne.Min`/`fyne.Max` semantics. Do not
replace this with `math.Min`/`math.Max`, which use `float64` and differ around
NaN.

### 1.2 Reuse renderer object slices

- [ ] Update the `fyne.WidgetRenderer.Objects` documentation to state that the
  driver does not modify the returned slice.
- [ ] Cache the two possible Label object views in `widget/label.go`; return the
  provider-only suffix for non-selectable labels.
- [ ] Cache stable object slices in `container/navigation.go` and the mobile
  menu renderers.
- [ ] Audit other `Objects()` methods with:

  ```bash
  rg -n 'return \[\]fyne\.CanvasObject' --glob '*.go'
  ```

- [ ] Convert only renderers whose child identity/order is stable, or whose
  refresh path explicitly updates the cached slice.

Caveat: a cached slice is immutable to the driver, not immutable to its owning
renderer. Dynamic renderers must update or replace it when children change.
Returning a stale slice can make objects disappear, retain obsolete children,
or route input to the wrong object. Add a behavior test for every dynamic case.

### 1.3 Reuse renderer lookups in tree walking

- [ ] In `internal/driver/util.go`, obtain a widget renderer once while finding
  children and pass it to an internal clip predicate.
- [ ] Keep public/internal `IsClip` behavior for callers that do not already
  have a renderer.
- [ ] Retain the existing `requireVisible || cache.IsRendered(co)` order so a
  visible walk avoids a redundant cache lookup.
- [ ] Test scrollables, renderer-declared clips, non-widgets, hidden widgets,
  and a widget whose renderer has not yet been cached.

Caveat: do not cache renderers across frames. Reuse is valid only within one
walk because renderer invalidation may occur between frames.

### 1.4 Correct and complete canvas attachment

Refyne already has an allocation-free `AttachCanvas` fast path, but its
different-canvas path uses `LoadOrStore` and does not replace an existing
mapping. Setup runs, yet `GetCanvasForObject` can continue returning the old
canvas. This is a correctness issue beyond the upstream allocation fix.

- [ ] Keep the same-canvas path allocation-free and refresh its expiry.
- [ ] When an object moves to a different canvas, store a new `canvasInfo` for
  the new canvas before reporting that setup is needed.
- [ ] Keep `SetCanvasForObject` as the compatibility wrapper around
  `AttachCanvas`.
- [ ] Add tests proving first attach, same-canvas revisit, different-canvas
  replacement, `GetCanvasForObject`, `CleanCanvas(old)`, `CleanCanvas(new)`,
  expiry refresh, and setup call counts.
- [ ] Add an image case proving per-canvas texture setup is triggered once on a
  real canvas change.
- [ ] Run the cache tests with `-race`.

Caveat: canvas attachment normally runs on the UI/render path, but the map is
concurrent. Tests should make the intended last-store behavior explicit rather
than claiming a stronger multi-writer ordering guarantee than Refyne provides.

### 1.5 Skip no-op container moves

- [ ] Return immediately from `Container.Move` when the position is unchanged.
- [ ] Add a test proving a real move still repaints and a no-op move does not.

### Phase 1 Verification

```bash
gofumpt -l -w .
go test ./internal ./internal/cache ./internal/driver/common ./internal/driver/glfw
go test ./container ./layout ./dialog ./widget
go test -race ./internal/cache ./internal/driver/common
go test ./internal/driver/glfw -run '^$' \
  -bench '^BenchmarkFrame(EnsureMinSize|WalkTrees)$' -benchmem -count=10
```

### Phase 1 Exit Gate

- Target no more than 5 allocs/op in each frame benchmark. The prototype
  reached 2 allocs/op.
- Expect approximately 15% lower `EnsureMinSize` median and 30% lower tree-walk
  median on the recorded machine. Investigate, but do not force, those exact
  percentages on different hardware.
- No statistically meaningful regression above 5% in an affected benchmark.
- No changed screenshots or input behavior.

## Phase 2: Desktop Robustness and Pointer Correctness

### 2.1 Unknown GLFW scancodes

- [ ] Add `safeGetKeyName` in `internal/driver/glfw/window_desktop.go` and use it
  only after the known-key mapping returns `fyne.KeyUnknown`.
- [ ] Recover expected `*glfw.Error` failures, log the concrete error, and
  return an empty name so the final result remains `fyne.KeyUnknown`.
- [ ] Re-panic non-GLFW programmer panics instead of silently hiding them.
- [ ] Add a focused helper test for expected recovery and unexpected re-panic.
- [ ] Manually press Fn/vendor keys under Wayland if hardware is available.

Upstream recovers every panic. Refyne should narrow the recovery so unrelated
logic errors do not become mysterious ignored keystrokes.

### 2.2 Monitor disconnect races

- [ ] Audit every `GetVideoMode()` dereference, not only the upstream-edited
  lines:

  ```bash
  rg -n 'GetVideoMode\(\)' internal/driver/glfw
  ```

- [ ] Use operation-specific fallbacks:
  - centering: abort the reposition;
  - scale detection: return `1.0`;
  - scaled size: return zero size or propagate absence to the caller;
  - fullscreen transition: abort transition;
  - sibling-monitor placement: leave default placement unchanged.
- [ ] Avoid switching silently to an arbitrary monitor during a hotplug race.
- [ ] Add tests around any extracted pure fallback logic.
- [ ] Manually test display disconnect/reconnect, sleep/wake, and fullscreen if
  the environment permits.

### 2.3 Pending pointer before click

- [ ] Process a queued mouse move at the start of `processMouseClicked` so a
  touchscreen tap lands at the latest coordinates.
- [ ] Initialize new windows with `mousePosUpdateProcessed: true`; otherwise the
  first click incorrectly consumes a synthetic pending move at `(0,0)`.
- [ ] Port the upstream two-object regression test.

### 2.4 Empty collection focus handling

- [ ] Change click unfocus logic to ignore unfocus only when the currently
  focused object itself is under the pointer.
- [ ] Port tests for empty areas in `List`, `GridWrap`, `Table`, and `Tree`.
- [ ] Confirm clicks on a focused Entry and its relevant children still preserve
  focus as intended.

### Phase 2 Verification

```bash
gofumpt -l -w .
go test ./internal/driver/glfw ./widget
go test -run 'TestWindow_(TouchScreenTappedWithMouseMovePending|CollectionEmptyAreaUnfocus)' ./internal/driver/glfw
go test -run '^$' -tags wayland ./internal/glfw ./internal/driver/glfw
```

Manual Go2TV smoke tests: open xfilepicker, click empty collection space, use
keyboard navigation, press unusual keys under Wayland, toggle fullscreen, and
move the window between displays.

## Phase 3: URI Parsing and File-Path Semantics

Treat this as a separate commit and release candidate. URI strings are persisted,
compared, handed to custom repositories, and used to reopen media; a superficially
correct parser change can break queues or file access.

### Tasks

- [ ] Adapt the final v2.8.1 `storage/repository` implementation to embed
  `net/url.URL` in the private URI type.
- [ ] Preserve `fyne.URI` method behavior for scheme, authority, decoded path,
  raw query, fragment, name, extension, MIME lookup, equality, and `String()`.
- [ ] Preserve `CustomURIRepository.ParseURI` dispatch for registered schemes.
- [ ] Implement and test the special handling for:
  - POSIX file paths and `file:` URIs;
  - Windows drive paths with slash or backslash separators;
  - Windows paths presented without a `file` scheme;
  - UNC/network paths;
  - percent-encoded separators, spaces, `%`, `#`, `?`, and non-ASCII names;
  - IPv4, bracketed IPv6, userinfo, port, query, and fragment;
  - opaque URNs, including query and fragment;
  - schemes without an authority;
  - Android `content://` URIs;
  - invalid/empty paths and invalid hosts.
- [ ] Exercise `storage.Parent`, `storage.Child`, `storage.EqualURI`,
  `internal/repository`, `storage.Reader`, and `storage.ReaderSeeker`, not just
  parser unit tests.
- [ ] Remove `github.com/fredbi/uri` and generated escape helpers only after all
  references are gone; then run `go mod tidy` and review the complete module
  diff.
- [ ] In Go2TV, replace manual construction at
  `internal/gui/gui.go` (`storage.ParseURI("file://" + s.mediafile)`) with
  `storage.NewFileURI(s.mediafile)`.
- [ ] Add a Go2TV regression test with a local media path containing spaces,
  `#`, `%`, and non-ASCII characters.

### URI Caveats

- `url.URL.String()` escapes paths; compare semantic fields as well as the
  serialized string in tests.
- `RawPath`, `Opaque`, and `RawQuery` are not interchangeable. URNs must not be
  forced into hierarchical path form.
- A file URI's host/authority rules differ from a Windows drive prefix. Include
  explicit UNC tests before claiming Windows support.
- Custom repositories may depend on receiving the untouched original string.
- Android `content://` access has platform-specific URI wrappers and permission
  behavior; parser success alone does not prove the resource can be reopened.
- `EqualURI` changes can affect saved queues even when two URIs refer to the same
  file. Record intentional normalization differences.

### Phase 3 Verification

```bash
gofumpt -l -w .
go test ./storage/... ./internal/repository/... ./internal/driver/mobile/...
go test -race ./storage/... ./internal/repository/...
tmpdir=$(mktemp -d)
GOOS=windows go test -c -o "$tmpdir/repository.test.exe" ./storage/repository
```

The Windows compile-test binary is intentionally written outside the repository.

### Phase 3 Exit Gate

- Full URI matrix passes on Linux and Windows-target compilation succeeds.
- Existing serialized URI fixtures either remain byte-identical or have an
  explicitly reviewed migration impact.
- Go2TV opens and serves local files with reserved and non-ASCII characters.
- Android `content://` selection/reopen still works on a device or emulator.

## Phase 4: Android and Mobile Interaction

### 4.1 Android hardware keyboard duplication

- [ ] Make `processEvent`/`processKey` report whether Android input was consumed
  and pass that value to `AInputQueue_finishEvent`.
- [ ] Add a pure `keyEditsText` helper following upstream semantics:
  printable runes, Return, and Tab are consumed; Backspace and forward Delete
  remain available to the text watcher/input connection.
- [ ] Map `key.CodeDeleteForward` to `fyne.KeyDelete`.
- [ ] Unit-test printable, navigation, Return, Tab, Backspace, Delete, and
  software-keyboard device-ID decisions where they can be separated from cgo.
- [ ] Manually test both a physical keyboard and GBoard. Verify one character
  per press, Backspace, forward Delete, arrows, Return, Tab, key repeat, and
  shortcuts.

Caveat: the handled flag is part of Android event routing, not merely an
optimization. Consuming deletion events can suppress IME/text-watcher behavior;
failing to consume printable hardware events recreates the duplication bug.

### 4.2 Mobile Entry behavior

- [ ] Add the scroll offset to `Entry.TouchDown` pointer coordinates.
- [ ] Move selection reset into the device-specific touch-down path and keep
  `Tapped` from incorrectly preserving a previous double-tap selection.
- [ ] Port the multiline-scroll and tap-after-double/triple-tap tests.

### 4.3 File dialog padding

- [ ] Wrap built-in file-dialog content in `container.NewPadded`.
- [ ] Update tests to traverse the extra container layer without weakening
  assertions.
- [ ] Visually check the Go2TV Android picker at narrow and landscape sizes.

### Already Present; Do Not Re-port

- Android surface recreation when the native window changes.
- The native `query == NULL` check in Android EGL setup.

### Phase 4 Verification

```bash
gofumpt -l -w .
go test ./widget ./dialog ./internal/driver/mobile/...
ANDROID_HOME=/home/alex/Android/Sdk \
ANDROID_NDK_HOME=/home/alex/Downloads/android-ndk-r27d \
make android
```

The final command runs from `/home/alex/test/go2tv` with the candidate Refyne
CLI and module selected as described under Integration Workspaces below.

## Phase 5: Mixed-Font Baselines and Emoji Sequences

This phase fixes visible text defects but touches shaping, measurement, drawing,
fallback fonts, and concurrency. Keep it independent of the frame optimization
release.

### Tasks

- [ ] Change `walkString` to collect shaped runs for one call, calculate the
  maximum ascent, and draw every run on that shared baseline.
- [ ] Update both `DrawString` and offset drawing; the first-run-only baseline in
  current Refyne is the defect.
- [ ] Use a per-call run buffer obtained from `async.Pool`, cleared and returned
  after use. Do not copy upstream's one package-global slice protected by a
  mutex.
- [ ] Ensure the pooled buffer is returned on all normal exits and does not hold
  unexpectedly large glyph slices indefinitely. Drop or cap oversized buffers
  if profiling shows retention.
- [ ] Keep shaping outputs alive until callbacks complete; do not return the
  shaper/segmenter or buffer to pools early.
- [ ] Adapt `splitEmojiSequences`, keycap detection, and single-face sequence
  resolution from the final upstream code.
- [ ] Preserve the existing shaper and segmenter pools.
- [ ] Add tests for:
  - Latin plus CJK/symbol fallback in both orders;
  - text plus emoji in both orders;
  - `#️⃣`, `*️⃣`, `0️⃣` through `9️⃣`, and `🔟`;
  - neighboring text not being pulled into the emoji face;
  - tabs and replacement glyphs;
  - `no_emoji` builds;
  - concurrent measurement and drawing under `-race`.
- [ ] Add golden image comparisons for representative mixed-font strings at
  scale 1 and a HiDPI scale.
- [ ] Benchmark single-font ASCII separately from mixed-font text; the common
  ASCII case must not regress materially.

### Font Caveats

- `sync.Pool` is a cache, not ownership. Reset the slice length before use and
  before return; never expose it after return.
- A shared max ascent intentionally changes pixels. Update golden images only
  after visual review, not automatically.
- `theme.DefaultEmojiFont()` can be nil under `no_emoji`.
- Emoji parsing here recognizes the v2.8.1 variation-selector/keycap case, not
  every Unicode grapheme sequence. Do not describe it as full ZWJ/emoji
  conformance.
- Measurement and drawing must share baseline logic or cursor/selection geometry
  can diverge from rendered text.

### Phase 5 Verification

```bash
gofumpt -l -w .
go test ./internal/painter ./canvas ./widget
go test -race ./internal/painter ./canvas ./widget
go test -tags no_emoji ./internal/painter ./canvas ./widget
```

### Phase 5 Exit Gate

- Mixed-font golden output is visually approved.
- Race detector passes the shaping tests.
- No statistically meaningful regression above 5% for single-font ASCII.
- Mixed-font and emoji regressions are covered without timing-based tests.

## Integration Workspaces

Use temporary Go workspaces to test local Refyne against consumers without
editing their `go.mod` files.

### Go2TV plus Refyne

```bash
workdir=$(mktemp -d)
(cd "$workdir" && go work init /home/alex/test/refyne /home/alex/test/go2tv)
cd /home/alex/test/go2tv
GOWORK="$workdir/go.work" go test ./...
```

Use a task-specific variable such as `workdir`; do not repurpose `HOME`.

### xfilepicker plus Refyne

Create a separate temporary workspace containing
`/home/alex/test/refyne` and `/home/alex/test/xfilepicker`, then run:

```bash
workdir=$(mktemp -d)
(cd "$workdir" && go work init /home/alex/test/refyne /home/alex/test/xfilepicker)
cd /home/alex/test/xfilepicker
GOWORK="$workdir/go.work" go test ./dialog/...
```

Keep the two workspaces separate when validating Go2TV's pinned xfilepicker
version. A three-module workspace intentionally substitutes the xfilepicker
checkout and answers a different question.

### Android candidate CLI

Go2TV's Android packaging must use Refyne's own `cmd/fyne`. A workspace module
does not necessarily populate the version/replace fields expected by Go2TV's
`android-fyne` Make target. Build an explicit candidate executable outside the
repositories and pass its path through `FYNE`:

```bash
go build -o "$workdir/fyne" ./cmd/fyne
GOWORK="$workdir/go.work" FYNE="$workdir/fyne" \
ANDROID_HOME=/home/alex/Android/Sdk \
ANDROID_NDK_HOME=/home/alex/Downloads/android-ndk-r27d \
make android
```

Run the build command from Refyne and the Android command from Go2TV.

## Final Consumer Verification

After all intended phases pass independently:

- [ ] Run `gofumpt -l -w .` and `go test ./...` in Refyne.
- [ ] Run `go vet ./...` where supported by the current platform/cgo setup.
- [ ] Run the frame benchmarks again and archive the before/after `benchstat`
  output in the release notes or PR description, not as an untracked repo file.
- [ ] Test Go2TV with local Refyne through the temporary workspace:

  ```bash
  GOWORK="$workdir/go.work" make test
  GOWORK="$workdir/go.work" make build
  GOWORK="$workdir/go.work" make windows
  GOWORK="$workdir/go.work" go run cmd/fynedo-check/main.go internal/gui/
  ```

- [ ] Run xfilepicker `go test ./dialog/...` against local Refyne.
- [ ] Run the exact Android packaging command from Phase 4 for every phase that
  changes mobile or storage behavior.
- [ ] Smoke-test Go2TV queue scrolling, settings, local and remote file pickers,
  file paths with reserved characters, playback start/stop, keyboard navigation,
  fullscreen, and Android text entry.
- [ ] Tag a Refyne release candidate only after its consumer matrix passes.
- [ ] Update Go2TV's Refyne version only after the Refyne tag is available; run
  `go mod tidy` and review `go.mod`/`go.sum` before committing the pin.

Suggested release grouping:

1. `v2.8.104-rc1`: Phases 1 and 2.
2. A later RC adding Phase 3 after URI/device verification.
3. A later RC adding Phases 4 and 5 after Android and visual text approval.
4. Final `v2.8.104` only when all selected phases and consumers pass.

If URI or font work needs more soak time, ship Phases 1 and 2 as `v2.8.104` and
move later phases to the next patch tag. Do not hold the proven hot-path wins
hostage to unrelated high-risk changes.

## Deferred or Rejected v2.8.1 Changes

Do not include these in this effort without new evidence:

- Upstream `clear()` conversion (`#6449`): incompatible with Refyne's `go 1.19`
  declaration, and the tagged GridWrap form needed an immediate follow-up fix.
- Popup-over-overlay fix (`#6426`): Refyne lacks the upstream redispatch logic
  that caused the regression; xfilepicker's manually created popups do not
  currently need it.
- Delayed large `FyneDo` warning: Refyne never added the startup warning being
  delayed.
- Systray shortcut/menu refresh, macOS menu switching, dynamic Form replacement,
  float binding, capture opacity, and always-on-top changes: low or no current
  Go2TV relevance. Reassess if usage changes.
- Android surface recreation and native NULL check: already present in Refyne.
- Wholesale storage, mobile, or GLFW directory replacement: would overwrite
  Refyne-specific behavior.

## Rollback Strategy

- Keep one commit per numbered subphase where practical.
- Preserve benchmark/test commits with the implementation they validate.
- If integration fails, first revert the most recent high-risk phase rather than
  editing around it in the release branch.
- URI rollback must also revert the Go2TV `NewFileURI` migration and dependency
  removal as one reviewed unit.
- Font rollback must restore both measurement and drawing callbacks together.
- Never revert Refyne's existing surface, cache, SVG, list, seekable-reader, or
  vendored GLFW work merely to make an upstream patch apply.

## Completion Record

Update this section while implementing so the document remains resumable:

| Phase | Status | Commit/tag | Verification notes |
| --- | --- | --- | --- |
| 0. Baseline | Complete | `4e0ef33b5` | Baseline captured on Go 1.26.7, Linux/amd64, Ryzen 9 7900, without a display server. Existing GLFW golden failures recorded before implementation. |
| 1. Frame paths | Complete | `94d121153` | Targeted and race tests pass. Ten-sample medians: 108.4 us/2 allocs for `EnsureMinSize`, 20.53 us/2 allocs for `WalkTrees`. |
| 2. Desktop robustness | Automated checks complete | `94d121153` | Focused pointer/focus/key tests and Wayland-target compilation pass. Fn-key, monitor-hotplug, and touchscreen hardware checks remain manual. |
| 3. URI handling | Automated checks complete | `670a42ba5`; Go2TV `7ba975b` | Storage/repository normal and race tests pass; Windows test binary compiles. Go2TV reserved/non-ASCII path regression passes. Android `content://` reopen remains a device check. |
| 4. Android/mobile | Automated checks complete | `12d46b3fe` | Widget, dialog, and mobile tests pass; Go2TV APK packages and verifies with the local Refyne CLI. Physical keyboard/GBoard and picker-layout checks remain manual. |
| 5. Fonts/emoji | Complete | `82e7f318d` | Normal, race, and `no_emoji` tests pass. Mixed-font scale 1/2 goldens were visually reviewed; ASCII and mixed benchmarks are included. |
| Consumer integration | Automated checks complete | Go2TV `7ba975b` | Go2TV full tests, native build, Windows package, Android package, and Fyne-thread audit pass against local Refyne; xfilepicker dialog tests pass. Interactive playback/UI smoke tests and release tagging remain pending. |

The final `go test ./...` reaches all packages but reports three failures that
also reproduce from the untouched Refyne baseline on this machine:
`internal/driver/glfw.TestMenuBar` font-dependent golden differences,
`internal/driver/mobile.Test_canvas_Dragged` missing a font-dependent hard-coded
tap coordinate, and `theme.TestFromJSON` environment-dependent resource names.
All affected phase packages and focused regressions otherwise pass. `go vet
./...`, `gofumpt -l -w .`, Windows/Wayland compilation, and the selected race
matrix pass.

## Remaining Manual Verification

- Exercise a physical Android keyboard and GBoard, including deletion, arrows,
  repeat, Return, Tab, and shortcuts.
- Reopen an Android `content://` selection on a device or emulator.
- Exercise touchscreen taps and empty collection areas on Linux.
- Exercise monitor hotplug, sleep/wake, fullscreen, and unusual Wayland keys.
- Visually inspect the Android picker at narrow and landscape sizes.
- Smoke-test Go2TV playback, queue/settings scrolling, local/remote pickers,
  reserved-character paths, keyboard navigation, and fullscreen before tagging
  a release candidate.
