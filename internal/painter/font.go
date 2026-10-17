package painter

import (
	"bytes"
	"image/color"
	"image/draw"
	"math"
	"strings"
	"sync"

	"github.com/go-text/render"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/async"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/lang"
	"github.com/alexballas/refyne/v2/theme"
)

const (
	// DefaultTabWidth is the default width in spaces
	DefaultTabWidth               = 4
	UnderlineOffsetFromBaseline   = 2
	StrikethroughToBaselineFactor = 0.75

	fontTabSpaceSize = 10
	replacementChar  = 0xfffd

	emojiVariationSelector = '\uFE0F'
	keycapMark             = '\u20E3'
	maxPooledShapedRuns    = 256
)

var (
	fm           *fontscan.FontMap
	fontScanLock sync.Mutex
	// fontUseLock protects go-text's lazy caches embedded in shared font.Face
	// values. Shapers and run buffers are per-call, but the cached faces are not
	// safe for concurrent shaping.
	fontUseLock sync.Mutex
	loaded      bool
)

// HarfbuzzShaper reuses internal buffers and is not safe for concurrent use,
// so each goroutine takes its own instance from the pool.
var shaperPool = async.Pool[*shaping.HarfbuzzShaper]{New: func() *shaping.HarfbuzzShaper { return &shaping.HarfbuzzShaper{} }}

// Segmenter also carries reusable internal buffers; pool it for the same
// reason. The slice returned by Split is owned by the Segmenter, so it must
// only be returned to the pool after the caller is done with that slice.
var segmenterPool = async.Pool[*shaping.Segmenter]{New: func() *shaping.Segmenter { return &shaping.Segmenter{} }}

type shapedRun struct {
	out shaping.Output
	x   float32
}

type shapedRunBuffer []shapedRun

var shapedRunPool = async.Pool[*shapedRunBuffer]{New: func() *shapedRunBuffer {
	buffer := make(shapedRunBuffer, 0, 8)
	return &buffer
}}

// GetShaper borrows a HarfBuzz shaper from the shared pool.
// Return it with PutShaper when done.
func GetShaper() *shaping.HarfbuzzShaper { return shaperPool.Get() }

// PutShaper returns a shaper borrowed with GetShaper to the pool.
func PutShaper(s *shaping.HarfbuzzShaper) { shaperPool.Put(s) }

// GetSegmenter borrows a text segmenter from the shared pool. Return it with
// PutSegmenter when done — only after any slice obtained from Split is no
// longer in use, as Split's result is owned by the Segmenter.
func GetSegmenter() *shaping.Segmenter { return segmenterPool.Get() }

// PutSegmenter returns a segmenter borrowed with GetSegmenter to the pool.
func PutSegmenter(s *shaping.Segmenter) { segmenterPool.Put(s) }

func loadMap() {
	loaded = true

	fm = fontscan.NewFontMap(noopLogger{})
	err := loadSystemFonts(fm)
	if err != nil {
		fm = nil // just don't fallback
	}
}

func lookupLangFont(family string, aspect font.Aspect) *font.Face {
	fontScanLock.Lock()
	defer fontScanLock.Unlock()

	if !loaded {
		loadMap()
	}
	if fm == nil {
		return nil
	}

	fm.SetQuery(fontscan.Query{Families: []string{family}, Aspect: aspect})
	l, _ := language.NewLangID(language.Language(lang.SystemLocale().LanguageString()))
	return fm.ResolveFaceForLang(l)
}

func lookupRuneFont(r rune, family string, aspect font.Aspect) *font.Face {
	fontScanLock.Lock()
	defer fontScanLock.Unlock()

	if !loaded {
		loadMap()
	}
	if fm == nil {
		return nil
	}

	fm.SetQuery(fontscan.Query{Families: []string{family}, Aspect: aspect})
	fm.SetScript(language.LookupScript(r))
	return fm.ResolveFace(r)
}

func lookupFaces(theme, fallback, emoji fyne.Resource, family string, style fyne.TextStyle) (faces *dynamicFontMap) {
	f1 := loadMeasureFont(theme)
	if theme == fallback {
		faces = &dynamicFontMap{family: family, faces: []*font.Face{f1}}
	} else {
		f2 := loadMeasureFont(fallback)
		faces = &dynamicFontMap{family: family, faces: []*font.Face{f1, f2}}
	}

	aspect := font.Aspect{Style: font.StyleNormal}
	if style.Italic {
		aspect.Style = font.StyleItalic
	}
	if style.Bold {
		aspect.Weight = font.WeightBold
	}

	if emoji != nil {
		faces.addFace(loadMeasureFont(emoji))
	}

	local := lookupLangFont(family, aspect)
	if local != nil {
		faces.addFace(local)
	}

	return faces
}

// CachedFontFace returns a Font face held in memory. These are loaded from the current theme.
func CachedFontFace(style fyne.TextStyle, source fyne.Resource, o fyne.CanvasObject) *FontCacheItem {
	if source != nil {
		val, ok := fontCustomCache.Load(source)
		if !ok {
			face := loadMeasureFont(source)
			if face == nil {
				face = loadMeasureFont(theme.TextFont())
			}
			faces := &dynamicFontMap{family: source.Name(), faces: []*font.Face{face}}

			val = &FontCacheItem{Fonts: faces}
			fontCustomCache.Store(source, val)
		}
		return val
	}

	scope := ""
	if o != nil { // for overridden themes get the cache key right
		scope = cache.WidgetScopeID(o)
	}

	val, ok := fontCache.Load(cacheID{style: style, scope: scope})
	if !ok {
		var faces *dynamicFontMap

		th := theme.CurrentForWidget(o)
		font1 := th.Font(style)

		emoji := theme.DefaultEmojiFont() // TODO only one emoji - maybe others too
		switch {
		case style.Monospace:
			faces = lookupFaces(font1, theme.DefaultTextMonospaceFont(), emoji, fontscan.Monospace, style)
		case style.Bold:
			if style.Italic {
				faces = lookupFaces(font1, theme.DefaultTextBoldItalicFont(), emoji, fontscan.SansSerif, style)
			} else {
				faces = lookupFaces(font1, theme.DefaultTextBoldFont(), emoji, fontscan.SansSerif, style)
			}
		case style.Italic:
			faces = lookupFaces(font1, theme.DefaultTextItalicFont(), emoji, fontscan.SansSerif, style)
		case style.Symbol:
			th := theme.SymbolFont()
			fallback := theme.DefaultSymbolFont()
			f1 := loadMeasureFont(th)

			if th == fallback {
				faces = &dynamicFontMap{family: fontscan.SansSerif, faces: []*font.Face{f1}}
			} else {
				f2 := loadMeasureFont(fallback)
				faces = &dynamicFontMap{family: fontscan.SansSerif, faces: []*font.Face{f1, f2}}
			}
		default:
			faces = lookupFaces(font1, theme.DefaultTextFont(), emoji, fontscan.SansSerif, style)
		}

		val = &FontCacheItem{Fonts: faces}
		fontCache.Store(cacheID{style: style, scope: scope}, val)
	}

	return val
}

// ClearFontCache is used to remove cached fonts in the case that we wish to re-load Font faces
func ClearFontCache() {
	fontCache.Clear()
	fontCustomCache.Clear()
	parsedFonts.Clear()
	clearShapeCache()
}

// DrawString draws a string into an image.
func DrawString(dst draw.Image, s string, color color.Color, f shaping.Fontmap, fontSize, scale float32, style fyne.TextStyle) {
	DrawStringOffset(dst, s, color, f, fontSize, scale, style, 0)
}

// DrawStringOffset draws a string shifted left by the specified pixel offset.
func DrawStringOffset(dst draw.Image, s string, color color.Color, f shaping.Fontmap, fontSize, scale float32, style fyne.TextStyle, offset int) {
	r := render.Renderer{
		FontSize: fontSize,
		PixScale: scale,
		Color:    color,
	}

	advance := float32(0)
	walkString(f, s, float32ToFixed266(fontSize), style, &advance, scale, func(run shaping.Output, x, y float32) {
		yPixel := int(math.Ceil(float64(y)))
		if len(run.Glyphs) == 1 && run.Glyphs[0].GlyphID == 0 {
			r.DrawStringAt(string([]rune{replacementChar}), dst, int(x)-offset, yPixel, f.ResolveFace(replacementChar))
			return
		}

		r.DrawShapedRunAt(run, dst, int(x)-offset, yPixel)
	})
}

// PlacedGlyph is one shaped glyph together with the position DrawString would
// have rasterised it at, in destination pixels.
type PlacedGlyph struct {
	Glyph shaping.Glyph
	Face  *font.Face
	// X is the pen position, Y the baseline shared by every glyph in the string.
	X, Y float32
}

// WalkGlyphs shapes s exactly as DrawString does and reports each glyph with the
// position it would be drawn at, so a caller can rasterise and cache glyphs
// individually instead of taking a bitmap of the whole run.
//
// Glyph ID 0 is reported as-is. DrawString substitutes the replacement character
// from another face for those, which this deliberately does not do: a caller that
// cannot draw a notdef should fall back to DrawString for the whole string rather
// than have this grow a second shaping path.
func WalkGlyphs(f shaping.Fontmap, s string, fontSize, scale float32, style fyne.TextStyle, cb func(PlacedGlyph)) {
	advance := float32(0)
	walkString(f, s, float32ToFixed266(fontSize), style, &advance, scale, func(run shaping.Output, x, y float32) {
		// The truncation of x and the ceiling of y mirror DrawStringOffset and
		// DrawShapedRunAt; glyphs have to land on the same pixels either way or
		// the two paths would disagree about where a string sits.
		penX := float32(int(x))
		baseline := float32(math.Ceil(float64(y)))
		for _, g := range run.Glyphs {
			cb(PlacedGlyph{
				Glyph: g,
				Face:  run.Face,
				X:     penX + fixed266ToFloat32(g.XOffset)*scale,
				Y:     baseline - fixed266ToFloat32(g.YOffset)*scale,
			})
			penX += fixed266ToFloat32(g.Advance) * scale
		}
	})
}

// RasteriseGlyph draws a single glyph into dst at the given pen position and
// baseline, using the same renderer settings as DrawString. subX shifts the
// glyph a fraction of a pixel right of x, which is how a cached bitmap carries
// the sub-pixel part of a pen position that its quad cannot.
func RasteriseGlyph(dst draw.Image, g PlacedGlyph, c color.Color, fontSize, scale float32, x, y int, subX float32) {
	r := render.Renderer{FontSize: fontSize, PixScale: scale, Color: c}
	run := shaping.Output{Face: g.Face, Glyphs: []shaping.Glyph{g.Glyph}}
	// The shaped XOffset/YOffset are already folded into the caller's x/y, so
	// they are replaced rather than applied a second time. DrawShapedRunAt scales
	// XOffset by the pixel scale, hence the division.
	run.Glyphs[0].XOffset, run.Glyphs[0].YOffset = float32ToFixed266(subX/scale), 0
	fontUseLock.Lock()
	defer fontUseLock.Unlock()
	r.DrawShapedRunAt(run, dst, x, y)
}

// loadMeasureFont returns a new face for the font, which callers may use
// without locking. Faces are not safe for concurrent use, but the parsed Font
// they share is, so the parse is reused.
func loadMeasureFont(data fyne.Resource) *font.Face {
	if ft, ok := parsedFonts.Load(data); ok {
		return font.NewFace(ft)
	}
	loaded, err := font.ParseTTF(bytes.NewReader(data.Content()))
	if err != nil {
		fyne.LogError("font load error", err)
		return nil
	}
	parsedFonts.Store(data, loaded.Font)
	return loaded
}

// MeasureString returns how far dot would advance by drawing s with f.
// Tabs are translated into a dot location change.
func MeasureString(f shaping.Fontmap, s string, textSize float32, style fyne.TextStyle) (size fyne.Size, advance float32) {
	return walkString(f, s, float32ToFixed266(textSize), style, &advance, 1, func(shaping.Output, float32, float32) {})
}

// RenderedTextSize looks up how big a string would be if drawn on screen.
// It also returns the distance from top to the text baseline.
func RenderedTextSize(text string, fontSize float32, style fyne.TextStyle, source fyne.Resource) (size fyne.Size, baseline float32) {
	size, base := cache.GetFontMetrics(text, fontSize, style, source)
	if base != 0 {
		return size, base
	}

	size, base = measureText(text, fontSize, style, source)
	cache.SetFontMetrics(text, fontSize, style, source, size, base)
	return size, base
}

func fixed266ToFloat32(i fixed.Int26_6) float32 {
	return float32(float64(i) / (1 << 6))
}

func float32ToFixed266(f float32) fixed.Int26_6 {
	return fixed.Int26_6(float64(f) * (1 << 6))
}

func measureText(text string, fontSize float32, style fyne.TextStyle, source fyne.Resource) (fyne.Size, float32) {
	face := CachedFontFace(style, source, nil)
	return MeasureString(face.Fonts, text, fontSize, style)
}

func tabStop(spacew, x float32, tabWidth int) float32 {
	if tabWidth <= 0 {
		tabWidth = DefaultTabWidth
	}

	tabw := spacew * float32(tabWidth)
	tabs, _ := math.Modf(float64((x + tabw) / tabw))
	return tabw * float32(tabs)
}

func walkString(faces shaping.Fontmap, s string, textSize fixed.Int26_6, style fyne.TextStyle, advance *float32, scale float32,
	cb func(run shaping.Output, x, y float32),
) (size fyne.Size, base float32) {
	fontUseLock.Lock()
	defer fontUseLock.Unlock()

	s = strings.ReplaceAll(s, "\r", "")
	// The string primitive does not support newlines, and go-text cuts the run
	// at one, so it is shaped as a replacement character instead.
	s = strings.ReplaceAll(s, "\n", string([]rune{replacementChar}))
	shaped := shapeString(faces, s, textSize)

	x := float32(0)
	spacew := scale * fontTabSpaceSize
	if style.Monospace {
		spacew = scale * fixed266ToFloat32(shaped.space.Advance)
	}
	buffer := shapedRunPool.Get()
	runs := (*buffer)[:0]
	defer func() {
		clearShapedRuns(runs)
		if cap(runs) <= maxPooledShapedRuns {
			runs = runs[:0]
			*buffer = runs
			shapedRunPool.Put(buffer)
		}
	}()

	maxAscent := fixed.Int26_6(0)
	collect := func(run shaping.Output, runX float32) {
		if run.LineBounds.Ascent > maxAscent {
			maxAscent = run.LineBounds.Ascent
		}
		runs = append(runs, shapedRun{out: run, x: runX})
	}

	for _, step := range shaped.steps {
		if step.tab {
			x = tabStop(spacew, x, style.TabWidth)
		} else {
			x = layoutRun(step.out, x, scale, collect)
		}
	}
	if maxAscent == 0 {
		maxAscent = shaped.space.LineBounds.Ascent
	}
	y := fixed266ToFloat32(maxAscent) * scale
	for _, run := range runs {
		cb(run.out, run.x, y)
	}

	*advance = x
	return fyne.NewSize(*advance, fixed266ToFloat32(shaped.space.LineBounds.LineThickness())),
		fixed266ToFloat32(maxAscent)
}

// shapedString is what the shaper made of one string: a space, which sets the
// tab width and line metrics, then the string's runs in order with the tab
// stops between them. Shaping depends on the font size alone, not the scale, so
// measuring a string and drawing it share one.
type shapedString struct {
	space shaping.Output
	steps []shapeStep
}

// shapeStep is one shaped run, or a tab stop when tab is set.
type shapeStep struct {
	out shaping.Output
	tab bool
}

type shapeKey struct {
	text  string
	faces *dynamicFontMap
	size  fixed.Int26_6
}

// shapeCacheMax caps the shaped string cache. A string is needed only until it
// has been measured and drawn, and labels that churn make new ones without end,
// so past the cap the cache is emptied and refills from the strings in use.
// 4096 strings of a dozen glyphs is about 4MB.
const shapeCacheMax = 4096

// shapeCache holds strings shaped for measuring until they are drawn, and the
// other way round. It is guarded by fontUseLock.
var shapeCache = make(map[shapeKey]*shapedString)

// spaceCacheMax caps the shaped space cache, which grows by one entry per font
// map and size: a handful in a normal app, more while a text size animates.
const spaceCacheMax = 256

type spaceKey struct {
	faces *dynamicFontMap
	size  fixed.Int26_6
}

// spaceCache holds the space shaped for each font map and size, which every
// string needs for its line metrics and tab width. A font map only ever gains
// faces, so the face that resolves a space never changes. It is guarded by
// fontUseLock.
var spaceCache = make(map[spaceKey]shaping.Output)

func clearShapeCache() {
	fontUseLock.Lock()
	shapeCache = make(map[shapeKey]*shapedString)
	spaceCache = make(map[spaceKey]shaping.Output)
	fontUseLock.Unlock()
}

// shapeSpace returns a space shaped in faces at textSize. Callers hold
// fontUseLock.
func shapeSpace(shaper *shaping.HarfbuzzShaper, faces shaping.Fontmap, textSize fixed.Int26_6) shaping.Output {
	key := spaceKey{size: textSize}
	key.faces, _ = faces.(*dynamicFontMap)
	if key.faces != nil {
		if out, ok := spaceCache[key]; ok {
			return out
		}
	}

	out := shaper.Shape(shaping.Input{
		Text:      []rune{' '},
		RunStart:  0,
		RunEnd:    1,
		Direction: di.DirectionLTR,
		Face:      faces.ResolveFace(' '),
		Size:      textSize,
	})
	if key.faces != nil {
		if len(spaceCache) >= spaceCacheMax {
			spaceCache = make(map[spaceKey]shaping.Output)
		}
		spaceCache[key] = out
	}
	return out
}

// shapeString returns s shaped in faces at textSize, from the cache when it has
// been shaped already. Only the painter's own font maps are cached: they are
// what every caller but tests pass, and comparable. Callers hold fontUseLock.
func shapeString(faces shaping.Fontmap, s string, textSize fixed.Int26_6) *shapedString {
	key := shapeKey{text: s, size: textSize}
	key.faces, _ = faces.(*dynamicFontMap)
	if key.faces != nil {
		if shaped, ok := shapeCache[key]; ok {
			return shaped
		}
	}

	shaper := shaperPool.Get()
	defer shaperPool.Put(shaper)
	segmenter := segmenterPool.Get()
	defer segmenterPool.Put(segmenter)

	shaped := &shapedString{space: shapeSpace(shaper, faces, textSize)}

	runes := []rune(s)
	in := shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: di.DirectionLTR,
		Face:      faces.ResolveFace(' '),
		Size:      textSize,
	}
	for _, in := range splitEmojiSequences(in, faces, segmenter) {
		inEnd := in.RunEnd

		pending := false
		for i, r := range in.Text[in.RunStart:in.RunEnd] {
			if r == '\t' {
				if pending {
					in.RunEnd = i
					shaped.steps = append(shaped.steps, shapeStep{out: shaper.Shape(in)})
				}
				shaped.steps = append(shaped.steps, shapeStep{tab: true})

				in.RunStart = i + 1
				in.RunEnd = inEnd
				pending = false
			} else {
				pending = true
			}
		}

		shaped.steps = append(shaped.steps, shapeStep{out: shaper.Shape(in)})
	}

	if key.faces != nil {
		if len(shapeCache) >= shapeCacheMax {
			shapeCache = make(map[shapeKey]*shapedString)
		}
		shapeCache[key] = shaped
	}
	return shaped
}

func clearShapedRuns(runs shapedRunBuffer) {
	for i := range runs {
		runs[i] = shapedRun{}
	}
}

// layoutRun places one shaped run with its pen at x, splitting out glyphs the
// font did not have so each can be drawn on its own, and returns the pen
// position after it.
func layoutRun(out shaping.Output, x, scale float32, cb func(shaping.Output, float32)) float32 {
	glyphs := out.Glyphs
	start := 0
	pending := false
	adv := fixed.I(0)
	for i, g := range out.Glyphs {
		if g.GlyphID == 0 {
			if pending {
				out.Glyphs = glyphs[start:i]
				cb(out, x)
				x += fixed266ToFloat32(adv) * scale
				adv = 0
			}

			out.Glyphs = glyphs[i : i+1]
			cb(out, x)
			x += fixed266ToFloat32(glyphs[i].Advance) * scale
			adv = 0

			start = i + 1
			pending = false
		} else {
			pending = true
		}
		adv += g.Advance
	}

	if pending {
		out.Glyphs = glyphs[start:]
		cb(out, x)
		x += fixed266ToFloat32(adv) * scale
		adv = 0
	}
	return x + fixed266ToFloat32(adv)*scale
}

// splitEmojiSequences segments input while keeping variation-selector emoji
// sequences together in a face that covers the complete sequence.
func splitEmojiSequences(in shaping.Input, faces shaping.Fontmap, segmenter *shaping.Segmenter) []shaping.Input {
	if !hasEmojiSequence(in) {
		return segmenter.Split(in, faces)
	}

	var output []shaping.Input
	start := in.RunStart
	for i := start; i < in.RunEnd; {
		length := emojiSequenceLen(in.Text, i, in.RunEnd)
		var face *font.Face
		if length > 0 {
			face = resolveSequence(faces, in.Text[i:i+length])
		}
		if face == nil {
			i++
			continue
		}

		if i > start {
			output = appendSplit(output, in, start, i, faces, segmenter)
		}
		output = appendSplit(output, in, i, i+length, fixedFontMap{face: face}, segmenter)
		i += length
		start = i
	}
	if start < in.RunEnd {
		output = appendSplit(output, in, start, in.RunEnd, faces, segmenter)
	}
	return output
}

func appendSplit(output []shaping.Input, in shaping.Input, start, end int, faces shaping.Fontmap, segmenter *shaping.Segmenter) []shaping.Input {
	in.RunStart, in.RunEnd = start, end
	return append(output, segmenter.Split(in, faces)...)
}

func hasEmojiSequence(in shaping.Input) bool {
	for _, r := range in.Text[in.RunStart:in.RunEnd] {
		if r == emojiVariationSelector {
			return true
		}
	}
	return false
}

func emojiSequenceLen(text []rune, start, end int) int {
	if start+1 >= end || text[start+1] != emojiVariationSelector {
		return 0
	}
	if start+2 < end && text[start+2] == keycapMark {
		return 3
	}
	return 2
}

func resolveSequence(faces shaping.Fontmap, sequence []rune) *font.Face {
	var candidates []*font.Face
	torn := false
	for _, r := range sequence {
		if r == emojiVariationSelector {
			continue
		}
		face := faces.ResolveFace(r)
		if len(candidates) > 0 && face != candidates[0] {
			torn = true
		}
		candidates = append(candidates, face)
	}
	if !torn {
		return nil
	}

	for _, face := range candidates {
		if coversSequence(face, sequence) {
			return face
		}
	}
	return nil
}

func coversSequence(face *font.Face, sequence []rune) bool {
	if face == nil {
		return false
	}
	for _, r := range sequence {
		if _, ok := face.NominalGlyph(r); !ok {
			return false
		}
	}
	return true
}

type fixedFontMap struct {
	face *font.Face
}

func (f fixedFontMap) ResolveFace(rune) *font.Face {
	return f.face
}

type FontCacheItem struct {
	Fonts shaping.Fontmap
}

type cacheID struct {
	style fyne.TextStyle
	scope string
}

var (
	fontCache       async.Map[cacheID, *FontCacheItem]
	fontCustomCache async.Map[fyne.Resource, *FontCacheItem] // for custom resources

	// parsedFonts holds each font resource parsed once. Every style's face list
	// includes the fallback and emoji fonts, and parsing the emoji font alone
	// takes several MB, so parsing per style multiplied that.
	parsedFonts async.Map[fyne.Resource, *font.Font]
)

type noopLogger struct{}

func (n noopLogger) Printf(string, ...any) {}

type dynamicFontMap struct {
	faces  []*font.Face
	family string
	mu     sync.Mutex
}

func (d *dynamicFontMap) ResolveFace(r rune) *font.Face {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, f := range d.faces {
		if _, ok := f.NominalGlyph(r); ok {
			return f
		}
	}

	toAdd := lookupRuneFont(r, d.family, font.Aspect{})
	if toAdd != nil {
		d.addFace(toAdd)
		return toAdd
	}

	return d.faces[0]
}

func (d *dynamicFontMap) addFace(f *font.Face) {
	d.faces = append(d.faces, f)
}
