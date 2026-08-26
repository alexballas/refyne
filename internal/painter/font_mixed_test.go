package painter

import (
	"image"
	"image/color"
	"sync"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type backportSplitFaceMap struct {
	ascii *font.Face
	other *font.Face
}

func (m backportSplitFaceMap) ResolveFace(r rune) *font.Face {
	if r < 128 {
		return m.ascii
	}
	return m.other
}

func TestWalkStringSharedLineAscent(t *testing.T) {
	ascii := loadMeasureFont(theme.DefaultTextFont())
	other := loadMeasureFont(theme.DefaultSymbolFont())
	require.NotNil(t, ascii)
	require.NotNil(t, other)
	faces := backportSplitFaceMap{ascii: ascii, other: other}

	baseline := func(text string) []float32 {
		var baselines []float32
		advance := float32(0)
		walkString(faces, text, float32ToFixed266(24), fyne.TextStyle{}, &advance, 1,
			func(_ shaping.Output, _, y float32) { baselines = append(baselines, y) })
		return baselines
	}

	asciiBaseline := baseline("A")[0]
	otherBaseline := baseline("⌘")[0]
	expected := fyne.Max(asciiBaseline, otherBaseline)
	for _, text := range []string{"A⌘", "⌘A", "A⌘A⌘"} {
		for _, got := range baseline(text) {
			assert.Equal(t, expected, got, "%q should use one shared baseline", text)
		}
	}
}

func TestWalkStringEmojiSequences(t *testing.T) {
	textFace := loadMeasureFont(theme.DefaultTextFont())
	emojiResource := theme.DefaultEmojiFont()
	if emojiResource == nil {
		t.Skip("emoji font disabled")
	}
	emojiFace := loadMeasureFont(emojiResource)
	require.NotNil(t, textFace)
	require.NotNil(t, emojiFace)
	faces := &dynamicFontMap{family: "sans-serif", faces: []*font.Face{textFace, emojiFace}}

	shape := func(text string) []shaping.Output {
		var runs []shaping.Output
		advance := float32(0)
		walkString(faces, text, float32ToFixed266(24), fyne.TextStyle{}, &advance, 1,
			func(run shaping.Output, _, _ float32) { runs = append(runs, run) })
		return runs
	}

	for _, sequence := range []string{"#️⃣", "*️⃣", "0️⃣", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"} {
		runs := shape(sequence)
		require.Len(t, runs, 1, sequence)
		assert.Same(t, emojiFace, runs[0].Face, sequence)
		require.Len(t, runs[0].Glyphs, 1, sequence)
		assert.NotZero(t, runs[0].Glyphs[0].GlyphID, sequence)
	}

	runs := shape("text 1️⃣ text")
	require.Len(t, runs, 3)
	assert.Same(t, textFace, runs[0].Face)
	assert.Same(t, emojiFace, runs[1].Face)
	assert.Same(t, textFace, runs[2].Face)
}

func TestWalkStringTabsAndReplacementGlyph(t *testing.T) {
	face := loadMeasureFont(theme.DefaultTextFont())
	require.NotNil(t, face)
	faces := fixedFontMap{face: face}
	advance := float32(0)
	var glyphs int
	_, baseline := walkString(faces, "A\tस", float32ToFixed266(18), fyne.TextStyle{}, &advance, 1,
		func(run shaping.Output, _, _ float32) { glyphs += len(run.Glyphs) })
	assert.Positive(t, advance)
	assert.Positive(t, baseline)
	assert.Positive(t, glyphs)
}

func TestConcurrentMeasureAndDrawString(t *testing.T) {
	faces := CachedFontFace(fyne.TextStyle{}, nil, nil).Fonts
	texts := []string{"Latin ⌘", "⌘ Latin", "emoji 1️⃣", "tabs\tand replacement स"}

	var wait sync.WaitGroup
	for i := 0; i < 16; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			for iteration := 0; iteration < 20; iteration++ {
				text := texts[(index+iteration)%len(texts)]
				MeasureString(faces, text, 18, fyne.TextStyle{})
				img := image.NewNRGBA(image.Rect(0, 0, 240, 64))
				DrawString(img, text, color.Black, faces, 18, 1, fyne.TextStyle{})
			}
		}(i)
	}
	wait.Wait()
}

func BenchmarkMeasureStringASCII(b *testing.B) {
	faces := CachedFontFace(fyne.TextStyle{}, nil, nil).Fonts
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		MeasureString(faces, "The quick brown fox jumps over the lazy dog", 18, fyne.TextStyle{})
	}
}

func BenchmarkMeasureStringMixedFonts(b *testing.B) {
	faces := CachedFontFace(fyne.TextStyle{}, nil, nil).Fonts
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		MeasureString(faces, "Latin ⌘ emoji 1️⃣", 18, fyne.TextStyle{})
	}
}
