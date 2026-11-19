package painter

import (
	"image"
	"image/color"
	"os"
	"sync"
	"testing"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/go-text/typesetting/shaping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/math/fixed"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/theme"
)

// han_partial.ttf has some Han runes and lacks others that han_full.ttf has,
// the way a Japanese font lacks some simplified Chinese runes. Both were made
// with fontTools, one box glyph for every rune of a format 13 cmap.
// han_other.ttf has only the three that han_partial.ttf lacks and han_full.ttf
// has, so no font of the pair han_partial.ttf and han_other.ttf has them all.
// Those three are 页, 础 and 镜; neither font has kana or Hangul.
func useHanFonts(t *testing.T, names ...string) {
	t.Helper()

	fontScanLock.Lock()
	oldMap, oldLoaded := fm, loaded
	fm, loaded = fontscan.NewFontMap(noopLogger{}), true
	fontScanLock.Unlock()
	t.Cleanup(func() {
		fontScanLock.Lock()
		fm, loaded = oldMap, oldLoaded
		fontScanLock.Unlock()
	})

	if len(names) == 0 {
		names = []string{"han_partial.ttf", "han_full.ttf"}
	}
	for _, name := range names {
		f, err := os.Open("testdata/font/" + name)
		require.NoError(t, err)
		require.NoError(t, fm.AddFont(f, name, ""))
		require.NoError(t, f.Close())
	}
}

// newHanMap returns a map set up the way CachedFontFace does, with the theme's
// text font and setUp, so the faces it looks up come from useHanFonts.
func newHanMap(setUp ...*font.Face) *dynamicFontMap {
	faces := &dynamicFontMap{family: fontscan.SansSerif, faces: []*font.Face{loadMeasureFont(theme.DefaultTextFont())}, fixed: 1}
	faces.faces = append(faces.faces, setUp...)
	faces.fixed += len(setUp)
	return faces
}

func loadTestFace(t *testing.T, name string) *font.Face {
	t.Helper()

	data, err := os.ReadFile("testdata/font/" + name)
	require.NoError(t, err)
	return loadMeasureFont(fyne.NewStaticResource(name, data))
}

func shapedFamilies(t *testing.T, s string, setUp ...*font.Face) []string {
	t.Helper()

	return familiesIn(newHanMap(setUp...), s)
}

func familiesIn(faces *dynamicFontMap, s string) []string {
	fontUseLock.Lock()
	defer fontUseLock.Unlock()
	shaped := shapeString(faces, s, fixed.I(20))

	var families []string
	for _, step := range shaped.steps {
		families = append(families, step.out.Face.Describe().Family)
	}
	return families
}

func TestShapeString_CJKStaysInOneFont(t *testing.T) {
	useHanFonts(t)

	// 页, 础 and 镜 are not in han_partial.ttf
	assert.Equal(t, []string{"Han Full"}, shapedFamilies(t, "首页基础像镜"))
	// 、 is in both fonts, and has to follow the rest
	assert.Equal(t, []string{"Han Full"}, shapedFamilies(t, "首页、基础镜像"))
	// every rune is in han_partial.ttf, which is found first
	assert.Equal(t, []string{"Han Partial"}, shapedFamilies(t, "首基像"))
	// U+3400 is in neither font, and must not undo the choice for the rest
	assert.Equal(t, []string{"Han Full"}, shapedFamilies(t, "首页\u3400基础"))
}

func TestShapeString_CJKFontOfTheMapIsKept(t *testing.T) {
	useHanFonts(t)

	partial := loadTestFace(t, "han_partial.ttf")

	// the font set up with the map draws 首, so the string is not moved to han_full.ttf
	families := shapedFamilies(t, "首页基础", partial)
	assert.Equal(t, "Han Partial", families[0])
}

func TestShapeString_CJKNoFontHasAll(t *testing.T) {
	useHanFonts(t, "han_partial.ttf", "han_other.ttf")

	// 首 and 基 are only in han_partial.ttf, 页 and 础 only in han_other.ttf, so
	// each rune keeps the font it is found in
	assert.Equal(t, []string{"Han Partial", "Han Other", "Han Partial", "Han Other"}, shapedFamilies(t, "首页基础"))
	// a string that one of them has all of is still drawn by it
	assert.Equal(t, []string{"Han Other"}, shapedFamilies(t, "页础镜"))
}

// TestShapeString_CJKGo2TVStrings shapes strings of go2tv's zh_Hans and
// zh_Hant translations, which mix Latin text, CJK punctuation that neither
// test font has, and runes han_partial.ttf lacks.
func TestShapeString_CJKGo2TVStrings(t *testing.T) {
	useHanFonts(t)
	latin := loadMeasureFont(theme.DefaultTextFont()).Describe().Family

	assert.Equal(t, []string{latin, "Han Full"}, shapedFamilies(t, "Github 页面"))
	assert.Equal(t, []string{"Han Full"}, shapedFamilies(t, "远程网页会话"))
	// zh_Hant has 頁 where zh_Hans has 页, and han_partial.ttf has all of it
	assert.Equal(t, []string{"Han Partial"}, shapedFamilies(t, "選擇媒體檔案"))
	assert.Equal(t, []string{latin, "Han Partial"}, shapedFamilies(t, "Github 頁面"))

	long := "启动一个独立的 Web UI 会话，拥有自己的播放列表、设备选择和播放状态。会话运行期间，Go2TV 主标签页将替换为会话状态视图。"
	families := shapedFamilies(t, long)
	assert.NotContains(t, families, "Han Partial", "every CJK rune follows 页 to han_full.ttf")
	assert.Contains(t, families, "Han Full")
	assert.Contains(t, families, latin)
}

// TestShapeString_CJKPinIsCachedPerString checks that the shape cache, which
// is keyed by the map a caller passes, keeps pinned and unpinned shaping of
// strings with the same runes apart, and that shaping a string again after the
// cache was emptied, as measuring and then drawing it can, pins the same way.
func TestShapeString_CJKPinIsCachedPerString(t *testing.T) {
	useHanFonts(t)
	faces := newHanMap()

	const pinned, unpinned = "首页基础", "首基"
	assert.Equal(t, []string{"Han Full"}, familiesIn(faces, pinned))
	assert.Equal(t, []string{"Han Partial"}, familiesIn(faces, unpinned))

	fontUseLock.Lock()
	cachedPinned := shapeCache[shapeKey{text: pinned, faces: faces, size: fixed.I(20)}]
	cachedUnpinned := shapeCache[shapeKey{text: unpinned, faces: faces, size: fixed.I(20)}]
	fontUseLock.Unlock()
	require.NotNil(t, cachedPinned)
	require.NotNil(t, cachedUnpinned)
	assert.NotSame(t, cachedPinned.steps[0].out.Face, cachedUnpinned.steps[0].out.Face,
		"首 is drawn by han_full.ttf in one string and han_partial.ttf in the other")

	clearShapeCache()
	assert.Equal(t, []string{"Han Partial"}, familiesIn(faces, unpinned))
	assert.Equal(t, []string{"Han Full"}, familiesIn(faces, pinned))
}

// TestWalkString_CJKMeasuredAndDrawnInOneFont measures a string, then walks
// its glyphs for drawing after the shape cache was emptied, and checks that
// both used the face the string is pinned to.
func TestWalkString_CJKMeasuredAndDrawnInOneFont(t *testing.T) {
	useHanFonts(t)
	faces := newHanMap()

	const s = "远程网页会话"
	measured, _ := MeasureString(faces, s, 14, fyne.TextStyle{})
	clearShapeCache()

	var drawnFaces []*font.Face
	var end float32
	WalkGlyphs(faces, s, 14, 1, fyne.TextStyle{}, func(g PlacedGlyph) {
		drawnFaces = append(drawnFaces, g.Face)
		end = g.X + fixed266ToFloat32(g.Glyph.Advance)
	})
	require.Len(t, drawnFaces, 6)
	for _, face := range drawnFaces {
		assert.Equal(t, "Han Full", face.Describe().Family)
	}
	assert.Equal(t, measured.Width, end)

	// and the pixels match a single string drawn in han_full.ttf alone
	full := drawnFaces[0]
	img := image.NewRGBA(image.Rect(0, 0, 120, 30))
	DrawString(img, s, color.White, faces, 14, 1, fyne.TextStyle{})
	want := image.NewRGBA(img.Bounds())
	DrawString(want, s, color.White, fixedFontMap{face: full}, 14, 1, fyne.TextStyle{})
	assert.Equal(t, want.Pix, img.Pix)
}

type cjkAtlasTexture struct{}

func (cjkAtlasTexture) FlushGlyphs()                      {}
func (cjkAtlasTexture) UploadGlyph(*image.RGBA, int, int) {}
func (cjkAtlasTexture) ResizeAtlas(int)                   {}

// TestGlyphAtlas_CJKGlyphsKeyedByPinnedFace checks that the atlas caches the
// glyphs of a pinned string under the face that drew them.
func TestGlyphAtlas_CJKGlyphsKeyedByPinnedFace(t *testing.T) {
	useHanFonts(t)
	t.Cleanup(ClearFontCache) // the cached map has looked up the test fonts

	// a font source of its own gives the text a new map, set up with the
	// theme's text font only
	source := fyne.NewStaticResource("cjk_atlas.ttf", theme.DefaultTextFont().Content())
	text := canvas.NewText("Github 远程网页会话", color.White)
	text.FontSource = source
	text.TextSize = 14

	var a GlyphAtlas
	_, ok := a.TextQuads(nil, text, fyne.NewPos(0, 0), 1, -1e9, 1e9, cjkAtlasTexture{})
	require.True(t, ok)

	families := map[string]int{}
	for key := range a.entries {
		families[key.face.Describe().Family]++
	}
	// the test fonts draw every rune with one box glyph, which every rune shares
	assert.Equal(t, 1, families["Han Full"], "the string is cached in han_full.ttf, which has 页")
	assert.Zero(t, families["Han Partial"], "the rest is in han_partial.ttf, but not drawn by it")
}

// TestShapeString_CJKConcurrentSharedMap shapes strings in one map from
// several goroutines, the way a map shared by every scope of a style is used,
// while others resolve runes in it outside the painter's lock as text
// truncation does. Run with -race it checks that pinning only reads the map.
func TestShapeString_CJKConcurrentSharedMap(t *testing.T) {
	useHanFonts(t)
	faces := newHanMap()
	faces.ResolveFace('首') // looks up han_partial.ttf before anything else
	require.Equal(t, "Han Partial", faces.faces[1].Describe().Family)
	strs := []string{"首页基础", "Github 页面", "远程网页会话", "首基像", "镜像"}

	var measuring, resolving sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		resolving.Add(1)
		go func(i int) {
			defer resolving.Done()
			// no font has U+3400, so each lookup of it appends a face
			for j := 0; j < 500; j++ {
				select {
				case <-stop:
					return
				default:
				}
				for _, r := range strs[(i+j)%len(strs)] + "\u3400" {
					faces.ResolveFace(r)
				}
			}
		}(i)

		measuring.Add(1)
		go func(i int) {
			defer measuring.Done()
			for j := 0; j < 40; j++ {
				s := strs[(i+j)%len(strs)]
				MeasureString(faces, s, float32(10+j), fyne.TextStyle{})
				faces.pinCJK([]rune(s))
			}
		}(i)
	}
	measuring.Wait()
	close(stop)
	resolving.Wait()

	faces.mu.Lock()
	before := append([]*font.Face(nil), faces.faces...)
	faces.mu.Unlock()
	clearShapeCache()
	assert.Equal(t, []string{"Han Full"}, familiesIn(faces, "首页基础"))
	assert.Equal(t, []string{"Han Partial"}, familiesIn(faces, "首基像"))
	faces.mu.Lock()
	assert.Equal(t, before, faces.faces, "shaping runes already resolved leaves the map as it was")
	faces.mu.Unlock()
}

// requireSystemCJK skips the test unless the system has fonts for the runes
// of samples that han_partial.ttf lacks, and logs which fonts those are.
func requireSystemCJK(t *testing.T, samples ...string) {
	t.Helper()

	found := map[string]bool{}
	for _, s := range samples {
		for _, r := range s {
			if !isCJK(r) || r >= 0x4e00 && r <= 0x9fff && r != '页' {
				continue // han_partial.ttf has the rest of the unified ideographs
			}
			face := lookupRuneFont(r, fontscan.SansSerif, font.Aspect{})
			if face == nil {
				t.Skipf("no system font found for %q", r)
			}
			if _, ok := face.NominalGlyph(r); !ok {
				t.Skipf("no system font has %q, the best match was %s", r, face.Describe().Family)
			}
			found[face.Describe().Family] = true
		}
	}
	t.Log("system CJK fonts used:", found)
}

// TestShapeString_CJKSystemFonts uses the system's CJK fonts. A map that has
// looked up han_partial.ttf for some Han runes, as a Japanese font that lacks
// simplified forms can be, finds a system font for the rest of a string: 页, or
// the kana, Hangul and CJK punctuation han_partial.ttf has none of. When one of
// the fonts found has every CJK rune of the string, the string is drawn in it.
// Otherwise each rune keeps its own font: Japanese system fonts, such as MS
// Gothic or Hiragino Sans, have the fullwidth colon but not the simplified 项.
func TestShapeString_CJKSystemFonts(t *testing.T) {
	samples := []string{
		"远程网页会话",      // zh_Hans, "Remote Web Session"
		"Github 页面",   // zh_Hans, "Github page"
		"播放列表：%d 项",   // zh_Hans, "Playlist: %d items"
		"設定と再生",       // Japanese, Kanji and kana
		"日本語のテキスト",    // Japanese, Kanji, Hiragana and Katakana
		"大韓民國 한국어 설정", // Korean, Hanja and Hangul
	}
	requireSystemCJK(t, samples...)
	partial := loadTestFace(t, "han_partial.ttf")
	latin := loadMeasureFont(theme.DefaultTextFont()).Describe().Family

	checked := 0
	for _, s := range samples {
		// han_partial.ttf as a face looked up before, not one set up with the map
		faces := newHanMap()
		faces.faces = append(faces.faces, partial)

		fontUseLock.Lock()
		shaped := shapeString(faces, s, fixed.I(20))
		fontUseLock.Unlock()
		var used []*font.Face
		var cjk []string
		for _, step := range shaped.steps {
			family := step.out.Face.Describe().Family
			if family != latin && !containsString(cjk, family) {
				cjk = append(cjk, family)
				used = append(used, step.out.Face)
			}
		}

		var runes []rune
		for _, r := range s {
			if isCJK(r) {
				runes = append(runes, r)
			}
		}
		pinnable := false
		for _, face := range used {
			pinnable = pinnable || coversSequence(face, runes)
		}
		if !pinnable {
			t.Logf("%q: none of %v has all its CJK runes, so each keeps its own font", s, cjk)
			continue
		}
		checked++
		assert.Len(t, cjk, 1, "%q is drawn in one CJK font", s)
		assert.NotContains(t, cjk, "Han Partial", "%q has runes that han_partial.ttf lacks", s)
	}
	require.NotZero(t, checked, "no sample could be drawn in one system font")
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func TestIsCJK(t *testing.T) {
	for _, r := range "首页、。：，ひらカタ한ㄱ" {
		assert.True(t, isCJK(r), string(r))
	}
	for _, r := range "aZ0 é—…“☆→\u00a0" {
		assert.False(t, isCJK(r), string(r))
	}
}

func TestPinCJK_LatinTextKeepsMap(t *testing.T) {
	useHanFonts(t)
	faces := newHanMap()

	assert.Same(t, faces, faces.pinCJK([]rune("Remember Playback Position")).(*dynamicFontMap))
	pinned, ok := faces.pinCJK([]rune("首页")).(cjkFontMap)
	require.True(t, ok)
	assert.Same(t, faces, pinned.dynamicFontMap)
	var _ shaping.Fontmap = pinned
}

// TestPinCJK_UncoveredRuneAddsNoFace checks that pinning adds nothing to a
// shared map for a rune no font has: the lookup gives a fallback face without
// it, and keeping that face would grow the map on every shape cache miss.
func TestPinCJK_UncoveredRuneAddsNoFace(t *testing.T) {
	useHanFonts(t)
	faces := newHanMap()
	for _, r := range "首页" {
		faces.ResolveFace(r) // looks up han_partial.ttf and han_full.ttf
	}
	size := func() int {
		faces.mu.Lock()
		defer faces.mu.Unlock()
		return len(faces.faces)
	}

	before := size()
	for i := 0; i < 3; i++ {
		faces.pinCJK([]rune("首㐀"))
		faces.pinCJK([]rune("首页㐀"))
	}
	assert.Equal(t, before, size(), "U+3400 is in no font")

	// shaping a string grows the map as much as resolving its runes once does
	before = size()
	for _, r := range "首㐀" {
		faces.ResolveFace(r)
	}
	resolved := size() - before
	for i := 0; i < 3; i++ {
		clearShapeCache()
		before = size()
		assert.Equal(t, []string{"Han Partial"}, familiesIn(faces, "首㐀"))
		assert.Equal(t, resolved, size()-before)
	}
}
