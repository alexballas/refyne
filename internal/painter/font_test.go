package painter_test

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/go-text/typesetting/font"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/painter"
	intTest "github.com/alexballas/refyne/v2/internal/test"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/theme"
)

type mixedGoldenFontMap struct {
	text, emoji *font.Face
}

func (m mixedGoldenFontMap) ResolveFace(r rune) *font.Face {
	switch r {
	case '1', '\uFE0F', '\u20E3':
		return m.emoji
	default:
		return m.text
	}
}

func loadGoldenFace(t *testing.T, resource fyne.Resource) *font.Face {
	t.Helper()
	face, err := font.ParseTTF(bytes.NewReader(resource.Content()))
	require.NoError(t, err)
	return face
}

func TestCachedFontFace(t *testing.T) {
	for name, tt := range map[string]struct {
		style fyne.TextStyle
		runes string
	}{
		"symbol font": {
			fyne.TextStyle{
				Symbol: true,
			},
			"←↑→↓↖↘↵↵⇞⇟⇥⇧⌃⌘⌥⌦⌫⎋␣⌃⌥⇧⌘",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := painter.CachedFontFace(tt.style, nil, nil)
			for _, r := range tt.runes {
				f := got.Fonts.ResolveFace(r)
				assert.NotNil(t, f, "symbol Font should include: %c", r)
			}
		})
	}

	// check the wide symbol rune
	symbol := canvas.NewText("⌘", color.Black)
	symbol.TextStyle.Symbol = true
	assert.True(t, symbol.MinSize().Width > 10)
}

func TestDrawString(t *testing.T) {
	for name, tt := range map[string]struct {
		color    color.Color
		style    fyne.TextStyle
		size     float32
		string   string
		tabWidth int
		want     string
	}{
		"regular": {
			color:    color.Black,
			style:    fyne.TextStyle{},
			size:     40,
			string:   "Hello\tworld!",
			tabWidth: 7,
			want:     "hello_TAB_world_regular_size_40_height_50_tab_width_7.png",
		},
		"bold italic": {
			color:    color.NRGBA{R: 255, A: 255},
			style:    fyne.TextStyle{Bold: true, Italic: true},
			size:     27.42,
			string:   "Hello\tworld!",
			tabWidth: 3,
			want:     "hello_TAB_world_bold_italic_size_27.42_height_42_tab_width_3.png",
		},
		"missing glyphs": {
			color:    color.Black,
			style:    fyne.TextStyle{},
			size:     40,
			string:   "Missing: स",
			tabWidth: 4,
			want:     "missing_glyph.png",
		},
	} {
		t.Run(name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, 300, 100))
			f := painter.CachedFontFace(tt.style, nil, nil)

			fontMap := &intTest.FontMap{f.Fonts.ResolveFace(' ')} // first (ascii) font
			painter.DrawString(img, tt.string, tt.color, fontMap, tt.size, 1, fyne.TextStyle{TabWidth: tt.tabWidth})
			test.AssertImageMatches(t, "font/"+tt.want, img)
		})
	}
}

func TestDrawStringMixedFontsGoldens(t *testing.T) {
	if theme.DefaultEmojiFont() == nil {
		t.Skip("emoji font disabled")
	}
	faces := mixedGoldenFontMap{
		text:  loadGoldenFace(t, theme.DefaultTextFont()),
		emoji: loadGoldenFace(t, theme.DefaultEmojiFont()),
	}
	for _, scale := range []struct {
		name          string
		value         float32
		width, height int
	}{
		{name: "scale_1", value: 1, width: 360, height: 80},
		{name: "scale_2", value: 2, width: 720, height: 160},
	} {
		t.Run(scale.name, func(t *testing.T) {
			img := image.NewNRGBA(image.Rect(0, 0, scale.width, scale.height))
			draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
			painter.DrawString(img, "Latin emoji 1️⃣ Latin", color.Black, faces, 28, scale.value, fyne.TextStyle{})
			test.AssertImageMatches(t, "font/mixed_fonts_"+scale.name+".png", img)
		})
	}
}

func TestMeasureString(t *testing.T) {
	for name, tt := range map[string]struct {
		style    fyne.TextStyle
		size     float32
		string   string
		tabWidth int
		want     float32
	}{
		"regular": {
			style:    fyne.TextStyle{},
			size:     40,
			string:   "Hello\tworld!",
			tabWidth: 7,
			want:     257.82812,
		},
		"bold italic": {
			style:    fyne.TextStyle{Bold: true, Italic: true},
			size:     27.42,
			string:   "Hello\tworld!",
			tabWidth: 3,
			want:     173.17188,
		},
		"missing glyph": {
			style:    fyne.TextStyle{},
			size:     40,
			string:   "Missing: स",
			tabWidth: 4,
			want:     213.65625,
		},
	} {
		t.Run(name, func(t *testing.T) {
			faces := painter.CachedFontFace(tt.style, nil, nil)
			fontMap := &intTest.FontMap{faces.Fonts.ResolveFace(' ')} // first (ascii) font
			got, _ := painter.MeasureString(fontMap, tt.string, tt.size, fyne.TextStyle{TabWidth: tt.tabWidth})
			assert.Equal(t, tt.want, got.Width)
		})
	}
}

func TestRenderedTextSize(t *testing.T) {
	size1, baseline1 := painter.RenderedTextSize("Hello World!", 20, fyne.TextStyle{}, nil)
	size2, baseline2 := painter.RenderedTextSize("\rH\re\rl\rl\ro\r \rW\ro\rr\rl\rd\r!\r", 20, fyne.TextStyle{}, nil)
	assert.Equal(t, int(size1.Width), int(size2.Width))
	assert.Equal(t, size1.Height, size2.Height)
	assert.Equal(t, baseline1, baseline2)
}

func TestHangul(t *testing.T) {
	got := painter.CachedFontFace(fyne.TextStyle{}, nil, nil)
	f := got.Fonts.ResolveFace('안')
	gid, ok := f.Cmap.Lookup('안')
	assert.True(t, ok)
	assert.NotZero(t, gid)
}
