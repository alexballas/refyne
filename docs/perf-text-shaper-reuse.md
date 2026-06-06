# Perf analysis: reuse the text shaper & segmenter

Status: analysis only — no code changed.
Scope: `internal/painter/font.go`.

## Summary

`walkString` allocates a brand-new HarfBuzz shaper and segmenter on **every
call**:

```go
// internal/painter/font.go:270-271
shaper := &shaping.HarfbuzzShaper{}
segmenter := &shaping.Segmenter{}
```

`walkString` is the core of both `MeasureString` (text layout / min-size) and
`DrawString` (text texture rasterization), so it runs on every uncached text
measurement and every text-texture build. `shaping.HarfbuzzShaper` is explicitly
designed to be *reused*: it owns an internal HarfBuzz buffer that is recycled
across `Shape()` calls. Allocating a fresh one each call throws that buffer away
every time and adds avoidable GC pressure on the text hot path. Reusing a pooled
shaper/segmenter is the same class of win as `Fyne.Do` — large impact, contained
change.

## Where it runs (call graph)

```
RenderedTextSize ──(cache miss)──► measureText ─► MeasureString ─► walkString ─► shapeCallback ─► shaper.Shape()
newGlTextTexture ────────────────────────────────► DrawString    ─► walkString ─► shapeCallback ─► shaper.Shape()
```

- `RenderedTextSize` is memoized in `fontSizeCache`, so steady-state repeats are
  cheap — but **every first measure of a string/size/style** (cache miss) pays
  full shaping. Lists, tables, editors, and any dynamic text churn the cache
  constantly (new strings, scrolling, resize re-layout, theme/scale changes that
  call `ResetThemeCaches`).
- `newGlTextTexture` runs on each text-texture (re)creation — i.e. whenever a
  label's text/colour/size/style changes or its texture is freed.

## The cost, precisely

Per `walkString` call the following are allocated and then discarded:

1. `&shaping.HarfbuzzShaper{}` — and, critically, its internal HarfBuzz buffer is
   re-created and re-grown on first `Shape()` instead of being reused.
2. `&shaping.Segmenter{}` — also holds reusable internal slices.
3. `runes := []rune(s)` — one heap slice per call.
4. `strings.ReplaceAll(s, "\r", "")` — allocates a new string (even when there is
   no `\r`, on older Go versions / depending on content).

Items 1–2 are the expensive ones: the shaper buffer reuse is the whole point of
the go-text API, and segmentation reuse avoids repeated allocation of run slices.
Items 3–4 are smaller but on the same hot path.

This is allocation/GC pressure, not raw GL work — it shows up as higher
`allocs/op`, more frequent GC, and CPU spent in `mallocgc` / HarfBuzz buffer init
rather than in actual shaping.

## Fix direction (for context — not applied here)

Pool the shaper and segmenter so the internal buffers survive across calls.
`walkString` is invoked only on the main/render goroutine in the common path, but
`MeasureString` can also be reached from background goroutines, so a
`sync.Pool` (not a single shared instance) is the safe choice:

```go
var shaperPool = sync.Pool{New: func() any { return &shaping.HarfbuzzShaper{} }}
var segmenterPool = sync.Pool{New: func() any { return &shaping.Segmenter{} }}

// in walkString:
shaper := shaperPool.Get().(*shaping.HarfbuzzShaper)
segmenter := segmenterPool.Get().(*shaping.Segmenter)
defer func() { shaperPool.Put(shaper); segmenterPool.Put(segmenter) }()
```

This is what upstream Fyne does. Optional follow-ups: keep a reusable
`[]rune` scratch buffer, and skip `ReplaceAll` when `strings.IndexByte(s,'\r')<0`.

Correctness note when measuring the fix: shaped output may reference the shaper's
internal buffer. `walkString` consumes each `shaping.Output` synchronously inside
the `cb` before the next `Shape()` and before returning the shaper to the pool, so
pooling is safe — but this invariant is exactly what the visual regression tests
(below) must confirm.

---

## How to measure the improvement

Take a baseline first, apply the change, then re-measure with identical inputs.

### 1. Micro-benchmark (primary signal: allocs/op + ns/op)

Add `internal/painter/font_bench_test.go`:

```go
package painter_test

import (
	"image"
	"image/color"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/painter"
)

func BenchmarkMeasureString(b *testing.B) {
	fonts := painter.CachedFontFace(fyne.TextStyle{}, nil, nil).Fonts
	const s = "The quick brown fox jumps over the lazy dog 0123456789"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = painter.MeasureString(fonts, s, 14, fyne.TextStyle{})
	}
}

func BenchmarkDrawString(b *testing.B) {
	fonts := painter.CachedFontFace(fyne.TextStyle{}, nil, nil).Fonts
	const s = "The quick brown fox jumps over the lazy dog 0123456789"
	dst := image.NewRGBA(image.Rect(0, 0, 512, 64))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		painter.DrawString(dst, s, color.Black, fonts, 14, 1, fyne.TextStyle{})
	}
}
```

> Note: `RenderedTextSize` is cached, so benchmark `MeasureString`/`DrawString`
> directly to measure the *shaping* path (a cache miss), not the cache hit.

Run before and after:

```bash
go test -run=^$ -bench='MeasureString|DrawString' -benchmem -count=10 \
  ./internal/painter/ | tee new.txt    # (baseline.txt before the change)
```

Compare with benchstat — this gives statistically meaningful deltas:

```bash
go install golang.org/x/perf/cmd/benchstat@latest
benchstat baseline.txt new.txt
```

Success criteria: lower `allocs/op` (expect a clear drop — the shaper/segmenter
plus the per-call rune slice disappear) and lower `ns/op`. `B/op` should also
fall.

### 2. Allocation profile (where the allocs go)

```bash
go test -run=^$ -bench=MeasureString -benchmem \
  -memprofile=mem.out ./internal/painter/
go tool pprof -alloc_objects -top mem.out
```

Before: `shaping.(*HarfbuzzShaper)` buffer init and `[]rune` conversion appear
near the top. After: they should be gone or far down the list. Use
`-alloc_space` for bytes and `list walkString` to see line-level attribution.

### 3. CPU profile (confirm time moves off mallocgc)

```bash
go test -run=^$ -bench=MeasureString -cpuprofile=cpu.out ./internal/painter/
go tool pprof -top cpu.out
```

Look for reduced time in `runtime.mallocgc` / GC and HarfBuzz buffer setup.

### 4. GC pressure under a realistic workload

Scroll/repaint a text-heavy screen (e.g. `cmd/fyne_demo`, the List/Table or a
long Label) and watch GC frequency:

```bash
GODEBUG=gctrace=1 go run ./cmd/fyne_demo 2>&1 | grep gc
```

Fewer / less frequent `gc N` lines while interacting with text indicates lower
churn. For a scriptable number, wrap a known text workload with
`runtime.ReadMemStats` and report the delta in `Mallocs` and `NumGC` before vs
after.

### 5. End-to-end frame timing (optional)

Instrument `repaintWindow` (`internal/driver/glfw/loop.go`) or `glCanvas.paint`
with a timer (or a one-off `pprof` capture) while continuously invalidating text,
and compare mean/95p frame time. This validates the micro-benchmark win
translates to real frames; the effect is largest right after cache invalidation
(theme/scale change, large text updates) when many strings re-shape in one frame.

### 6. Regression safety

The shaper buffer is reused, so verify output is byte-identical:

```bash
go test ./internal/painter/...   # includes the DrawString golden-image tests
```

The existing `TestDrawString` image-diff tests are the guard that pooling did not
corrupt shaped glyph runs.

## Expected outcome

- Measurable drop in `allocs/op` and `B/op` for `MeasureString`/`DrawString`.
- Lower CPU in `mallocgc`/GC; lower GC frequency under text-heavy interaction.
- Largest real-world benefit on cache-miss-heavy scenarios: scrolling lists/tables
  with changing text, editors, and the re-shape storm after theme/scale changes.
- No visual change (golden-image tests unchanged).
