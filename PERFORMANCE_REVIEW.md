# Refyne Performance Review

**Date:** 2026-06-11
**Scope:** full codebase review of the Refyne GUI framework (Fyne v2 fork) targeting runtime performance, memory efficiency, allocation reduction, scalability, and responsiveness — with zero observable change in appearance, behavior, compatibility, or developer-facing functionality.

**Method note:** findings are from systematic code reading of the render loop, caches, text stack, and widget layer — not from profiles of a running app. Where magnitudes are stated they are estimates unless marked "measured"; each item lists how to validate.

**Status update (2026-06-11):** findings **1.1 and 1.2 are implemented** (`internal/cache/svg.go`, `internal/cache/base.go`, `theme/icons.go`, `canvas/image.go`). Measured: themed icon `Content()` ~31× faster, 93 → 3 allocs/op.

**Status update (2026-06-12):** the **§4 quick-win batch (items 1–7) is implemented**, covering findings 1.3, 1.4, 2.2, 2.3 (segmenter pooling part), and 3.1. Measured: `MeasureString` ~13–23% faster with fewer allocations; the per-node canvas-attach fast path is 0 allocs/op (~44 ns). Remaining proposals: 2.1, 2.3 (space-run cache), 2.4, §3 (except 3.1), §5.

The engine's frame model: any `Refresh`/`Move`/`Resize` sets a canvas dirty flag; a ticker at display refresh rate triggers `drawSingleFrame`, which re-walks and re-draws **the entire window** (`internal/driver/glfw/loop.go:67`, `internal/driver/glfw/canvas.go:305`). That makes two things dominant: **per-node cost in the walk** (paid thousands of times per frame) and **work triggered per `Refresh()` call** (paid on every interaction). Most findings below fall into one of those two buckets.

---

## 1. Highest-impact opportunities

### 1.1 SVG XML is re-parsed (twice) and re-colorized on every widget refresh — ✅ IMPLEMENTED

**Code:** `canvas/image.go:133` (`Image.Refresh`) → `updateReader` (`canvas/image.go:302`) → `svg.Colorize` (`internal/svg/svg.go:23`) and `imageDetailsFromReader` → `svg.NewDecoder`; plus `theme/icons.go` resource wrappers.

**Why it's a bottleneck:** `Image.Refresh()` unconditionally rebuilds the image source. Implementation review found it to be *worse* than initially reported: `theme.ThemedResource.Content()` (and the Inverted/Error/Primary/Disabled wrappers) run `svg.Colorize` — a full XML unmarshal **and** re-marshal — on **every `Content()` call** (`theme/icons.go:713` et al.). `Image.Refresh` calls `Content()` up to three times (once via `svg.IsResourceSVG`, which itself calls it twice, once for the data), colorizes **again** in `updateReader` with the widget-override-aware color, then `svg.NewDecoder` does yet another full oksvg parse. Only the final rasterization was cached. A single button hover (`buttonRenderer.Refresh` → `r.icon.Refresh()` twice, `widget/button.go:346/:406`) cost on the order of **ten** XML parses + several marshals. It scales with icon count in toolbars, lists, and trees.

**Implemented fix (three layers, all expiring via the existing cache mechanism):**

1. **Colorized-source cache** (`internal/cache/svg.go`): `GetColorizedSvg`/`SetColorizedSvg` keyed by `(source name, colorized?, RGBA)` — the resolved color is part of the key, so theme/variant changes self-invalidate.
2. **Theme layer** (`theme/icons.go`): `colorizeLogError` now takes the unwrapped source resource and consults the cache before colorizing. This single choke point fixes all five wrapper `Content()` methods and every consumer, including `IsResourceSVG`. (Wrapper `Name()`s are color-name-prefixed, theme entries key by the *unwrapped* source name — the two layers cannot collide.)
3. **Canvas layer** (`canvas/image.go`): `updateReader` caches its second (widget-theme-aware) colorize pass keyed by the wrapper resource name + resolved color; `imageDetailsFromReader` caches the parsed `*svg.Decoder` keyed by `(i.name(), colorize color)` — **for `Resource`-backed images only**. File-backed SVGs still re-parse on `Refresh()`, since re-reading edited files is part of that path's contract.

All three caches are cleared by `ResetThemeCaches` (called on settings change by both drivers and the test app) and expired by `destroyExpiredSvgs` in the periodic `Clean`.

**Measured impact:** `ThemedResource.Content()` for the standard cancel icon: **8,506 ns → 269 ns per call (~31×), 93 → 3 allocs/op, 10,552 B → 228 B/op** (Ryzen 9 7900). A hovered icon button previously did this ~10× per state change plus an oksvg parse; all of it is now map lookups.

**Behavior preservation:** byte-identical output — the cache stores exactly what `svg.Colorize` returned for the same (source, color) input, and the same parsed decoder yields the same rasters. Validated: full `internal/cache`, `theme`, `canvas`, `internal/svg`, `widget` (including screenshot assertions), `container`, and `test` suites pass; `go vet` clean.

**Accepted deltas (flagged):** (a) for resources whose `Content()` changes under an unchanged `Name()`, colorized bytes/decoders can be served stale for up to the cache TTL — the same staleness class the pre-existing raster cache already accepted; (b) the "failed to colorize" error log for non-SVG content wrapped in a themed resource now fires once per cache lifetime instead of on every `Content()` call; (c) decoders are shared across `Image` instances — safe because all drawing is serialized on the main goroutine (documented on `GetSvgDecoder`).

### 1.2 The SVG raster cache holds only one size per icon name → cache thrash at mixed sizes — ✅ IMPLEMENTED

**Code:** `internal/cache/svg.go`; consumer at `canvas/image.go` (`renderSVG`).

**Why it was a bottleneck:** the cache key was the resource *name* (shared across all objects using that icon), but the stored entry held a single `(w, h)`. `GetSvg` returned nil if the requested size differed. If the same icon was displayed at two sizes (toolbar 24px + menu 16px, or two windows at different DPI), every refresh of one size evicted the other: each `Refresh` re-rasterized a full vector image (`i.icon.Draw`) forever, in alternation. Rasterization is by far the most expensive step (rasterx scanline rendering), so this was a silent O(refresh × icon) CPU sink in icon-rich apps.

**Implemented fix:** the cache is now keyed by `svgCacheKey{name, w, h}`, storing one entry per (name, size); the size-mismatch rejection check is gone (a mismatch is simply a different key).

**Impact:** eliminates repeated rasterization wherever an icon appears at >1 size. Combined with 1.1 this makes icon refresh nearly free end-to-end: source colorize, parse, and raster are all cache hits.

**Behavior preservation:** pure cache-keying change; rendered pixels identical. Memory grows by one bitmap per distinct size, bounded by the existing 1-minute expiry (`destroyExpiredSvgs`). `ResetThemeCaches` clears the map on theme change as before. All existing cache tests pass unchanged (they assert exactly the miss-on-different-size behavior, which the new keying preserves).

### 1.3 Per-node allocations and redundant cache lookups in the every-frame tree walks — ✅ IMPLEMENTED

**Code:** `internal/driver/common/canvas.go:85-149` (`EnsureMinSize`), `internal/cache/canvases.go:22` (`SetCanvasForObject`), `internal/driver/util.go:137-202` (`walkObjectTree`), `internal/driver/util.go:204` (`IsClip`), `internal/driver/glfw/canvas.go:323-347` (paint callbacks).

**Why it's a bottleneck:** every repainted frame runs two full tree walks (EnsureMinSize, then paint). Per visible node, per frame, today this does:

1. **One heap allocation + one discarded `time.Now()`** in `SetCanvasForObject`: it unconditionally builds a new `*canvasInfo` and calls `LoadOrStore`, throwing the allocation away on the ~100% common "already attached" path (`internal/cache/canvases.go:23-26`).
2. **One closure allocation** for the `func() { img.Refresh() }` setup callback (`internal/driver/common/canvas.go:105`), heap-allocated because it escapes into `SetCanvasForObject` — even for the vast majority of nodes that aren't images.
3. **A wasted renderer-map lookup** in `walkObjectTree`: `cache.IsRendered(co) || requireVisible` evaluates `IsRendered` (a `sync.Map` load) first even though `requireVisible` is true for every paint/EnsureMinSize walk (`internal/driver/util.go:160`).
4. **`IsClip` evaluated twice per node per paint** (in `paint` and `afterPaint`, `internal/driver/glfw/canvas.go:325/335`), each doing type assertions plus a `CachedRenderer` map load plus `setAlive`/`time.Now()` for widgets.

For a 2,000-node UI at 120 Hz during an animation, that's ~500k map operations, ~480k `time.Now()` calls, and ~480k heap allocations per second of avoidable overhead plus the GC pressure they create.

**Proposed improvements** (all mechanical, all behavior-identical):

```go
// internal/cache/canvases.go — fast path: no allocation when already attached.
func SetCanvasForObject(obj fyne.CanvasObject, c fyne.Canvas, setup func()) {
	if old, ok := canvases.Load(obj); ok && old.canvas == c {
		old.setAlive()
		return
	}
	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()
	old, found := canvases.LoadOrStore(obj, cinfo)
	if (!found || old.canvas != c) && setup != nil {
		setup()
	}
}
```

For the setup closure, note that passing `img.Refresh` as an argument would *not* help: Go evaluates the bound-method value (an allocation) before the callee can take any fast path. The API has to be inverted so the caller only does setup work when attachment actually changed:

```go
// internal/cache/canvases.go — AttachCanvas reports whether the object was newly
// attached (or moved to a different canvas), i.e. whether setup work is needed.
func AttachCanvas(obj fyne.CanvasObject, c fyne.Canvas) bool {
	if old, ok := canvases.Load(obj); ok && old.canvas == c {
		old.setAlive()
		return false
	}
	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()
	old, found := canvases.LoadOrStore(obj, cinfo)
	return !found || old.canvas != c
}
```

```go
// internal/driver/common/canvas.go — allocation-free for every node.
ensureMinSize := func(node *RenderCacheNode, pos fyne.Position) {
	obj := node.obj
	if cache.AttachCanvas(obj, c.impl) {
		if img, ok := obj.(*canvas.Image); ok {
			img.Refresh() // this may now have a different texScale
		}
	}
	...
```
(`SetCanvasForObject` stays for the other callers; `EnsureMinSize` switches to the split form.)

```go
// internal/driver/util.go — skip the redundant IsRendered load on visible walks.
case fyne.Widget:
	if requireVisible || cache.IsRendered(co) {
		children = cache.Renderer(co).Objects()
	}
```

For the double `IsClip`: cache the result on the node for the duration of the walk (add `isClip bool` to `RenderCacheNode`, set it in the `paint` callback, read it in `afterPaint`). The renderer cannot change between the two callbacks of the same single-threaded walk, so this is exactly equivalent.

**Expected impact:** removes ~2 allocations and ~3–5 `sync.Map` operations per node per frame. On large UIs (tables, trees, editors) this is plausibly 10–25% of frame CPU during continuous animation, and meaningfully less GC. Medium-high confidence; trivially profiled with `pprof`.

**Implementation notes (2026-06-12):** all four parts landed as proposed — `AttachCanvas` in `internal/cache/canvases.go` (`SetCanvasForObject` is now a thin wrapper over it; measured 0 allocs/op, ~44 ns steady state), `EnsureMinSize` gating image setup on its result, the `requireVisible` short-circuit reorder in `internal/driver/util.go`, and the per-walk clip flag (`SetClips`/`Clips` on `RenderCacheNode`, used by both the glfw and mobile paint callbacks).

**Behavior preservation:** `LoadOrStore` semantics are reproduced (the fast path only triggers when the entry exists with the same canvas — precisely the case where `setup` would not run); boolean short-circuit reorder has no side effects (`IsRendered` is a pure read, and the renderer's expiry is still refreshed by the `cache.Renderer` call that follows); the clip flag is recomputed every walk. **One flagged delta:** the fast path now calls `setAlive()` on the existing entry, which the old code did not — previously an object walked every frame still had its canvas-attachment entry expire once a minute, get dropped by `Clean`, and be re-attached on the next frame (re-running the image-refresh setup spuriously). Attachment entries for actively walked objects now stay alive, eliminating that churn; rendering is identical.

### 1.4 `RangeExpiredTexturesFor` scans the entire global texture map every repainted frame — ✅ IMPLEMENTED

**Code:** `internal/driver/common/canvas.go:226-238` (`FreeDirtyTextures`) → `internal/cache/texture_common.go:42-60`; called from `repaintWindow` (`internal/driver/glfw/loop.go:242`).

**Why it's a bottleneck:** every repainted frame iterates **all** entries of both global texture maps (`textTextures`, `objectTextures`) for every window, checking expiry against a 1-minute TTL. The maps hold one entry per distinct text run and per image/raster/gradient. A text-heavy app (editor, table, log viewer) easily accumulates thousands of entries; during animations or typing you pay an O(total-textures) `sync.Map.Range` (which itself allocates and promotes the read map) at up to 240 Hz — to find entries that, with a 60-second TTL, can only actually expire about once a minute.

**Proposed improvement:** throttle the expiry scan in the spirit of `cache.Clean` (`internal/cache/base.go:29-55`) — but the throttle **must be per-canvas**, not global. `RangeExpiredTexturesFor` filters entries by canvas and must run inside that window's GL context to delete textures; with a single shared timestamp, whichever window repaints first after the interval consumes the scan slot *for its canvas only*, and since `drawSingleFrame` iterates windows in stable order, a later window's expired textures could be deferred indefinitely rather than by one second. The natural home is a field on the canvas that owns the call:

```go
// internal/driver/common/canvas.go
type Canvas struct {
	...
	lastTextureScan time.Time
}

const textureScanInterval = time.Second

func (c *Canvas) FreeDirtyTextures() uint64 {
	... // existing refreshQueue drain, untouched

	if now := time.Now(); now.Sub(c.lastTextureScan) >= textureScanInterval {
		c.lastTextureScan = now
		cache.RangeExpiredTexturesFor(c.impl, c.painter.Free)
	}
	return objectsToFree
}
```

**Expected impact:** removes an O(N) scan from nearly every frame; bounds it to 1 Hz per window. For texture-heavy apps this is one of the larger steady-state frame costs after the walk itself. High confidence in mechanism; magnitude depends on texture population.

**Implementation note (2026-06-12):** landed exactly as proposed (per-canvas `lastTextureScan` on `common.Canvas`, `textureScanInterval = time.Second`, zero value scans on first call). Used by both desktop and mobile drivers since both go through `common.Canvas.FreeDirtyTextures`.

**Behavior preservation:** textures already live ≥60 s past last use; with the per-canvas timestamp each window's expired textures are freed at most one second later than today (each window throttles only its own scans, in its own GL context), so GPU memory release timing shifts by ≤1.7% of the TTL and nothing visible changes. The dirty-object frees (the part that matters for correctness, `refreshQueue` drain) are untouched. Risk: low; validate with the existing driver tests and a VRAM watch during a long multi-window soak.

---

## 2. Significant improvements

### 2.1 List: `RefreshItem` does a full-widget refresh, and every refresh constructs a throwaway template item

**Code:** `widget/list.go:208-218` (`RefreshItem` → `l.BaseWidget.Refresh()`), `widget/list.go:562-576` (`listRenderer.Refresh` → `createItemAndApplyThemeScope` + `updateList(false)`), keyboard nav at `widget/list.go:383-398` (two `RefreshItem` calls per keypress).

**Why it's a bottleneck:** `RefreshItem(id)` triggers `listRenderer.Refresh`, which (a) calls the user's `CreateItem()` to build a complete new widget tree just to measure `itemMin`, and (b) runs `updateList(false)`, re-running the user's `UpdateItem` callback for **every visible row**. Then it sets up the one requested row again. Arrow-key navigation does this twice per keypress. With a 50-row viewport and a non-trivial `UpdateItem`, one keypress costs 100+ row updates plus two full template constructions.

**Proposed improvement:**
- Cache the measuring template: store the created item on the `List`, call `Refresh()` + `MinSize()` on it in `listRenderer.Refresh` instead of `CreateItem()`. (`CreateItem` invocation count is not a documented contract, but flag it in the changelog; the widget already calls it at unpredictable times.)
- Make `RefreshItem` update only the target row **for content-only refreshes**: selection/focus/`UpdateItem` changes need the min-size revalidation and a canvas dirty mark, not `updateList` over all rows. **Important:** `SetItemHeight` also funnels through `RefreshItem` (`widget/list.go:225`) and *does* depend on the full path — a height change moves every row below it, changes the scroller's content min size, and alters which rows are visible. The narrowing therefore needs two paths: a narrow one for content refresh, and a relayout path (geometry + `updateList`) that `SetItemHeight` keeps using. A conservative first cut: keep `BaseWidget.Refresh()` but add a `refreshingItem` flag the layout consults to skip re-running `setupListItem` on rows other than `id`, leaving all geometry work intact. The least invasive first step is to drop the duplicated work in `TypedKey` by batching the two `RefreshItem` calls.

**Expected impact:** keyboard navigation and selection in lists becomes O(1) row updates instead of O(visible) — often the difference between smooth and laggy for lists with image-bearing rows (which, per finding 1.1, currently re-parse SVGs on each update). Medium confidence on exact wins; depends on app callbacks.

**Behavior/risks:** visual output identical (the same rows end up in the same states). The observable difference is fewer invocations of user callbacks (`CreateItem`/`UpdateItem`); the API docs explicitly permit calling them for caching/reuse at any time, but this needs a careful pass over `searchVisible` invariants and the table/tree/gridwrap siblings which share the pattern. Validate with `widget/list_test.go` (it asserts callback behavior in places — adjust only where counts are over-specified).

### 2.2 GL painter: per-draw vertex-slice allocations and string-keyed uniform lookups — ✅ IMPLEMENTED

**Code:** `internal/painter/gl/draw.go:543-551` (`lineCoords` returns a fresh `[]float32`), `:590-596` (`rectCoords`), `:663-668` (`vecRectCoordsWithPad`); uniform lookups `pState.uniforms[name]` in `SetUniform1f/2f/4f` (`internal/painter/gl/painter.go:83-112`) — ~6–12 map lookups per object per frame.

**Why it's inefficient:** every drawn object allocates 1 vertex slice (64–96 B) and performs several `map[string]` hashes per frame. The painter is strictly single-threaded per GL context, and `updateBuffer` consumes the slice synchronously (`BufferData` copies it out, `internal/painter/gl/draw_desktop.go:5`), so neither cost buys anything.

**Proposed improvement:**
- Give the painter a scratch array: `vertexScratch [24]float32`; have the coord helpers fill and return `p.vertexScratch[:n]`. No lifetime extends past the `DrawArrays` call.
- Resolve uniforms once at program-link time into typed fields, e.g. a `rectUniforms struct { frameSize, rectCoords, strokeWidthHalf … *UniformState }` per program, instead of string lookups per draw. Mechanical, fully behavior-preserving.

**Expected impact:** removes ~1 allocation + ~10 map lookups per object per frame; on a 500-object animated scene that's ~60k allocations/s gone plus GC relief. Moderate but cheap and risk-free. The existing `prev`-value uniform dedup (a good optimization) stays intact.

**Implementation notes (2026-06-12):** both parts landed, with two deviations from the sketch. (a) The scratch array cannot be an inline painter field: the slice is passed to C via `glBufferData`, and cgo rejects a pointer into an allocation that also contains Go pointers — so it is a separately allocated pointer-free `*[24]float32` (`painter.scratch()`), one allocation per painter lifetime. This was caught immediately by the cgo pointer check in the glfw driver tests. (b) Uniform *and* attribute handles are resolved into typed per-program structs (`resolvedUniforms`, populated by `resolveUniforms()` at the end of each platform `Init` — core/es/wasm/gomobile), and `SetUniform*`/`UpdateVertexArray` now take the handle directly, eliminating the attribute map lookups too (~12 map hashes per object per frame total).

**Behavior preservation:** identical GL command stream; the wasm and gles variants build, and the painter golden-image tests pass. Risk: only that a future refactor makes the painter concurrent — the scratch invariant is documented on the field.

### 2.3 Text measurement: per-call `Segmenter` allocation and a HarfBuzz shape of `" "` on every measure/draw — ⚠️ PARTIALLY IMPLEMENTED

**Code:** `internal/painter/font.go:260-315` (`walkString`): `segmenter := &shaping.Segmenter{}` per call; `out := shaper.Shape(in)` on a one-space input per call (used only for `LineBounds`/space advance). Also `truncateLimit` (`widget/richtext.go:1209-1210`) creates a fresh `HarfbuzzShaper` **and** `Segmenter` per truncation instead of using the pools.

**Why it's a bottleneck:** `walkString` backs every uncached `MeasureString`/`DrawString`. The `Segmenter` carries internal buffers explicitly designed for reuse; allocating it per call defeats that. The leading space-shape is a full HarfBuzz invocation whose result depends only on (face, size) — it's recomputed for every string measured. Text-heavy flows (RichText wrapping does *many* measurements per layout via `howManyRunesFit`'s search loop) multiply this.

**Proposed improvement:**
- Pool the segmenter exactly like the shaper: `var segmenterPool = async.Pool[*shaping.Segmenter]{New: …}`.
- Cache the space-run results per (face pointer, size) in a small `async.Map`; invalidate alongside `ClearFontCache`. Store only the two read values (`Advance`, `LineBounds`) in a narrow struct, not the whole `shaping.Output`, to avoid aliasing risk.
- Use the pools in `truncateLimit`.

**Expected impact:** measurable cut in allocation and shaping work for first-layout and re-wrap paths (window resize of text-heavy UI, typing in `Entry`). Medium confidence — gate it on the existing `internal/painter/font_bench_test.go` benchmarks before/after.

**Implementation status (2026-06-12):** the pooling parts are done — `segmenterPool` in `internal/painter/font.go` (used by `walkString`), plus exported `GetShaper`/`PutShaper`/`GetSegmenter`/`PutSegmenter` so `truncateLimit` in `widget/richtext.go` uses the pools instead of fresh instances. Note the pooled `Segmenter` must only be returned after the slice from `Split` is done with (it owns that slice); both call sites return it via defer. Measured (`font_bench_test.go`): `MeasureStringASCII` 31.2 → 27.1 µs, `MeasureStringTabs` 19.2 → 14.9 µs, with 3–8 fewer allocs/op. **The space-run cache (per face+size `Advance`/`LineBounds`) is not yet implemented** — that is the remaining piece of this finding.

**Behavior preservation:** same shaping inputs ⇒ same outputs; pooling is already the established pattern for the shaper in this exact function.

### 2.4 `Canvas.Refresh` pays a `runtime.Stack` call per refreshed object

**Code:** `internal/driver/common/canvas.go:287-290` → `async.EnsureMain` → `IsMainGoroutine` → `goroutineID()` which calls `runtime.Stack` (`internal/async/goroutine.go:77-85`).

**Why it's a cost:** every single `canvas.Refresh(obj)` — i.e. every widget refresh, including each child in a `Container.Refresh` cascade — captures and parses a goroutine stack header just to learn the goroutine ID (microsecond-scale). The protected operation is `SetDirty()`, a bool store.

**Proposed improvement:** make the dirty flag an `atomic.Bool` and set it directly:

```go
// internal/driver/common/canvas.go
dirty atomic.Bool

func (c *Canvas) Refresh(obj fyne.CanvasObject) {
	c.refreshQueue.In(obj) // already lock-free
	c.dirty.Store(true)
}

func (c *Canvas) CheckDirtyAndClear() bool { return c.dirty.Swap(false) }
func (c *Canvas) SetDirty()               { c.dirty.Store(true) }
```

**Expected impact:** removes a µs-scale cost from the hottest API in the framework; also fixes a latent data race (the current non-atomic `dirty` is written via `EnsureMain` only on a best-effort basis).

**Behavior preservation:** repaint scheduling is identical; off-main `Refresh` becomes *safer*, not less safe. **Tradeoff:** the developer-facing "called from wrong thread" diagnostic log for this one path disappears. If that diagnostic is valued, keep `EnsureMain` only in debug/hints builds (`internal/build.Mode == fyne.BuildDebug`), preserving release performance.

---

## 3. Moderate improvements

**3.1 `time.Now()` on every cache hit (`setAlive`). — ✅ IMPLEMENTED** `internal/cache/base.go`. Every `CachedRenderer`, `GetTexture`, `GetFontMetrics`, `GetCanvasForObject` hit called `time.Now()` to push expiry. With a 60 s TTL, sub-second precision is pointless. Implemented as a package-level `coarseTimestamp atomic.Int64` (unix nanos) refreshed at the top of `cache.Clean` — which all three drivers (glfw, mobile, test) call per frame — and read by `setAlive`; a zero value (Clean never ran, e.g. headless use) falls back to `timeNow()`. Saves tens of thousands of vDSO calls/s on big UIs; expiry shifts by <1 frame. The cache test mocks (`timeMock.setTime`, `testClearAll`) keep the coarse stamp in sync with the injected `timeNow`.

**3.2 List with custom item heights: O(n) scans per scroll event.** `widget/list.go:512-534` (`calculateVisibleRowHeights`) and `scrollTo` (`:247-263`) iterate from row 0 on every offset change when `itemHeights` is non-empty; `contentMinSize` iterates the whole map per call. For a 100k-row list with a few custom heights, every scroll frame walks 100k iterations. Maintain a sorted slice / Fenwick (prefix-sum) tree of custom heights to make offset→row queries O(log n). Behavior identical (same arithmetic); moderate implementation effort; validate against `list_test.go` custom-height cases.

**3.3 RichText wrapping floods the font-metric cache with probe substrings.** `howManyRunesFit` (`widget/richtext.go:877`) binary-searches by measuring successive prefixes; each probe goes through `RenderedTextSize`, which **stores** every probe string in `fontSizeCache` (`internal/painter/font.go:226-235`) — allocating the key string and an entry per probe, for strings that will never be looked up again. Add an uncached measure path (`measureText` directly) for the search probes, caching only the final accepted line. Reduces map churn and memory during resize of wrapped text. Behavior identical (cache is an optimization, not semantics); confirm no test relies on probe-warming.

**3.4 `Image.MinSize()` can trigger a full disk reload.** `canvas/image.go:112-119` calls `i.Refresh()` (file open + decode) from inside `MinSize` when aspect is unknown — and `MinSize` is called from layout walks. Once an image has loaded, `aspect != 0` so the steady state is fine, but a hidden-then-shown or zero-sized image can re-enter this repeatedly. At minimum, memoize failure (today a missing file is retried on every MinSize call, with a logged error each frame). Low risk; needs care around the documented "Refresh picks up file changes" semantics.

**3.5 `RichText.Refresh`/`textRenderer.Refresh` re-allocate everything.** `widget/richtext.go:699-759`: every refresh rebuilds `objs` slices, re-converts each segment to `[]rune` (per bound), and re-slices strings; `updateRowBounds` re-wraps all rows even when only a style changed. Within current architecture, the cheap win is hoisting the repeated `[]rune(textSeg.Text)` conversions per segment (convert once per segment per refresh, reuse across its bounds) and reusing the `objs` slice across refreshes. Entry typing currently re-wraps the whole document per keystroke (`widget/entry.go:850` → `updateText` → `Refresh`) — see 5.4 for the architectural fix.

**3.6 Per-frame texture lookups for text do double duty.** `drawText` (`internal/painter/gl/draw.go:415`) calls `text.MinSize()` (font-metric cache lookup + `setAlive`) and then `getTexture` builds a `FontCacheEntry` (string-bearing struct key hashed into a `sync.Map`) every frame per text object. Caching the texture handle on the `RenderCacheNode` between frames (invalidated by the refresh queue) would skip both; this is a smaller, riskier version of the damage-tracking idea in §5 — worth doing only after profiling shows the maps high.

---

## 4. Low-risk quick wins

| # | Change | Location | Effect |
|---|--------|----------|--------|
| 1 | ✅ Reorder `cache.IsRendered(co) \|\| requireVisible` → `requireVisible \|\| cache.IsRendered(co)` — **done** | `internal/driver/util.go` | One `sync.Map` load saved per widget per walk (see 1.3) |
| 2 | ✅ Fast-path canvas attach (`AttachCanvas`, load before alloc/`LoadOrStore`) — **done** | `internal/cache/canvases.go` | Removes 1 alloc + closure + `time.Now()` per node per frame (0 allocs/op measured) |
| 3 | ✅ Throttle texture-expiry scan to 1 Hz **per canvas** — **done** | `internal/driver/common/canvas.go` (`FreeDirtyTextures`) | Removes O(textures) scan per frame |
| 4 | ✅ Pool `shaping.Segmenter`; use pools in `truncateLimit` — **done** | `internal/painter/font.go`, `widget/richtext.go` | `MeasureString` ~13–23% faster, fewer allocs (measured) |
| 5 | ✅ Painter scratch buffer for vertex slices — **done** (separate pointer-free allocation; cgo constraint) | `internal/painter/gl/draw.go` | 1 alloc per object per frame removed |
| 6 | ✅ Pre-resolved uniform **and attribute** handles instead of `map[string]` per set — **done** | `internal/painter/gl/painter.go` | ~12 map hashes per object per frame removed |
| 7 | ✅ Coarse per-frame timestamp for `setAlive` — **done** | `internal/cache/base.go` | Thousands of `time.Now()` calls per frame removed |
| 8 | ✅ SVG raster cache keyed by (name, w, h) — **done** | `internal/cache/svg.go` | Stops re-rasterization thrash (see 1.2) |

All eight are implemented (2026-06-12). Items 1, 2, 3, 7 together remove the bulk of the *fixed* per-node frame overhead; 5, 6 remove the per-drawn-object overhead; 4, 8 remove text/icon recomputation. Validation: full build (host, wasm, gles), `go vet`, race-enabled tests for the touched packages, and the cache/driver/painter (incl. software golden-image)/widget/container/test/dialog/layout/binding suites — all green. The only glfw-driver test failure (`TestMenuBar` golden image) reproduces identically on the unmodified tree and is environment-related.

---

## 5. Architectural opportunities

**5.1 Damage-region rendering (partial repaint).** Today one blinking caret repaints the whole window: `paint` clears and re-draws every visible object (`internal/driver/glfw/canvas.go:305`), re-running texture lookups, uniform sets, and draw calls for thousands of unchanged objects. Some infrastructure exists — the refresh queue records objects passed to `Canvas.Refresh(obj)`, and `RenderCacheNode` persists per-object state across frames — but the queue is **not** a complete damage source: `Move`/`Resize` on base objects only mutate fields and set the canvas-level dirty flag with no object identity (`internal/widget/base.go:34`, `canvas/canvas.go:45`), the *old* bounds of a moved object are already gone by paint time, and several invalidations (window resize, overlay changes, theme application) are inherently canvas-wide. A real implementation therefore needs: (a) old/new bounds capture at `Move`/`Resize`/`Refresh` time (likely recorded on the `RenderCacheNode`), (b) a damage union with overlap resolution via the existing clip stack, and (c) a full-redraw fallback whenever the canvas-level dirty flag was set without object attribution. Done right, a scissored partial redraw would cut steady-state animation cost by 10–100× for localized changes. This is the single largest performance opportunity in the codebase — and the riskiest: overlap/z-order correctness, transparent backgrounds (Wayland CSD), and `Capture` semantics all need careful handling. Recommend prototyping behind a build tag with screenshot-diff validation against the software painter.

**5.2 Stop re-validating `MinSize` of the entire tree every frame.** `EnsureMinSize` (`internal/driver/common/canvas.go:85`) walks every visible node per repaint calling `obj.MinSize()`; container layouts recursively call children's `MinSize`, making the per-frame cost superlinear in depth. Since size invalidation originates from known events (refresh, resize, theme change), a dirty-propagation scheme (mark ancestors min-size-dirty when a node refreshes; skip clean subtrees in the walk — the `minSize` field on `RenderCacheNode` is already there) preserves exact layout results while reducing per-frame layout cost to O(changed). Medium risk: the current behavior also *implicitly* catches widgets whose MinSize changes without a Refresh (technically an API misuse, but apps rely on it). Needs a compatibility audit.

**5.3 Idle CPU: the event loop ticks at display refresh rate forever.** `runGL` (`internal/driver/glfw/loop.go:149-216`) wakes at up to 240 Hz to `pollEvents` + iterate windows even when nothing is dirty and no animation runs. For tray apps and background windows this is constant CPU/power burn. Note that `glfw.WaitEventsTimeout(frameInterval)` alone would *not* fix this — it wakes at exactly the same cadence as the current ticker; its only benefit is lower input latency between ticks. Dropping idle wakeups to near zero requires a genuinely adaptive loop: **block indefinitely** (`glfw.WaitEvents`) when there are no running animations, no dirty canvases, and no pending `funcQueue` work, with `glfw.PostEmptyEvent()` posted as the cross-thread wakeup whenever a function is queued or `Refresh` marks a canvas dirty; resume frame-interval pacing only while animations run or repaints are pending. Moderate-to-high risk: the wakeup must be raceless (post-after-enqueue), and the interaction with the `funcQueue` select, the rate ticker, and Wayland frame callbacks needs a careful redesign of the loop structure.

**5.4 Entry/RichText incremental layout.** Each keystroke re-wraps and re-measures the entire document (`Entry.TypedRune` → `updateText` → `RichText.Refresh` → `updateRowBounds` over all segments). Editors stutter beyond a few thousand lines. Incremental re-wrap (re-layout only the edited paragraph and lines whose wrap input changed) keeps identical output with O(edit) cost. Large effort; high payoff for text-centric apps.

**5.5 Draw-call batching.** Every object is one `BufferData` + `DrawArrays` with per-object uniform updates. Batching same-program quads (especially text glyph quads and rectangles) into shared VBOs would cut driver overhead dramatically on object-heavy scenes. Only worth it after 5.1, which reduces the object count per frame first.

---

## 6. Areas that appear already well optimized

- **Refresh queue and dedup** — lock-free MPSC queue with pooled nodes (`internal/async/queue.go`) plus a dedup map; well-designed for the workload.
- **Resize handling** — coalescing interactive resizes to one canvas resize per frame (`resize_coalesce.go`) and the Wayland frame-callback present gate (`present.go`) are both careful, correct latency/throughput tradeoffs.
- **Refresh-rate-driven loop with hotplug re-evaluation** (`loop.go:144-179`) — correct generalization of the old fixed 60 Hz.
- **Uniform redundant-set elimination** — the `prev [4]float32` dedup in `SetUniform*` already avoids most GL chatter (the remaining cost is the map lookup, item 4.6).
- **Render cache trees** — `walkTree`'s incremental node reuse (`common/canvas.go:438`) avoids rebuilding the node graph per frame.
- **List/Grid/Tree virtualization** — item pooling (`async.Pool`), binary search over visible rows, slice-reuse with explicit nil-ing; the issues found (2.1, 3.2) are at the edges, not the core.
- **Text caches** — font-metric, font-face, and per-string text-texture caches give good steady-state typing/render performance; the gaps are upstream of them (1.1, 2.3, 3.3).
- **Theme lookups** — flat switch statements over package-level color values; cheap. Widget-level `themeCache` avoids repeated override lookups.
- **HarfBuzz shaper pooling** — already in place (`shaperPool`); 2.3 just extends the same idea.

---

## Suggested execution order

1. ✅ **SVG source/raster caching (1.1 + 1.2)** — **implemented** (2026-06-11). `ThemedResource.Content()` measured at ~31× faster (8.5 µs → 269 ns, 93 → 3 allocs); icon refresh is now cache-hits end to end. Full test suites green.
2. ✅ **Quick-win batch (§4, items 1–7)** — **implemented** (2026-06-12), covering 1.3, 1.4, 2.2, 2.3 (pooling), 3.1. Measured: `MeasureString` ~13–23% faster; canvas-attach fast path 0 allocs/op; per-object GL draws no longer allocate or hash uniform names. All suites green.
3. **List refresh narrowing (2.1)** and the **space-run cache remainder of 2.3** — next up.
4. **Prototype damage tracking (5.1)** behind a build tag, validated by software-painter screenshot diffs, since it dwarfs everything else if it lands.

Every recommendation above keeps the GL command stream, layout results, event semantics, and public API unchanged; the only externally observable deltas are explicitly flagged (callback invocation counts in 2.1, the thread-misuse log in 2.4, texture-free timing in 1.4, and canvas-attachment expiry churn in 1.3). Before/after validation should combine the package test suite, `test`-package screenshot assertions, and CPU/alloc profiles of an animation-heavy and a text-heavy scenario.

---

## Appendix: relevance to go2tv

go2tv's UI profile (icon-dense toolbar, `widget.List` device/queue rows with icons, 1–2 s tickers driving refreshes — `internal/gui/main.go:414-489`, `main.go:61`, `gui.go:565/578`, `main.go:1010/1046`, `actions.go:1474/1634/2457`) makes the priorities:

1. ✅ **1.1 + 1.2 (SVG source/raster caching)** — **implemented**; eliminates per-refresh XML parsing for all icon buttons and list rows; biggest perceived-smoothness win.
2. **2.1 (List refresh narrowing)** — `DeviceList.Refresh()` currently re-creates the row template and re-runs `UpdateItem` for every visible row on each discovery tick.
3. **5.3 (idle loop wakeups)** — battery/background-CPU win for a long-running utility app sitting mostly idle between ticker updates.

The per-frame walk and GL allocation items matter less for go2tv than for animation-heavy apps: its window repaints in bursts (~1 Hz), so fixed per-refresh cost dominates over per-frame cost.
