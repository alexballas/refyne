package painter_test

import (
	"image"
	"image/color"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/painter"
	intTest "github.com/alexballas/refyne/v2/internal/test"
)

var (
	benchSize    fyne.Size
	benchAdvance float32
	benchImage   *image.RGBA
)

func benchmarkFontMap(style fyne.TextStyle) *intTest.FontMap {
	faces := painter.CachedFontFace(style, nil, nil)
	return &intTest.FontMap{faces.Fonts.ResolveFace(' ')}
}

func BenchmarkMeasureStringASCII(b *testing.B) {
	fonts := benchmarkFontMap(fyne.TextStyle{})
	const text = "The quick brown fox jumps over the lazy dog 0123456789"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSize, benchAdvance = painter.MeasureString(fonts, text, 14, fyne.TextStyle{})
	}
}

func BenchmarkMeasureStringTabs(b *testing.B) {
	fonts := benchmarkFontMap(fyne.TextStyle{})
	const text = "Name\tStatus\tUpdated\tOwner"
	style := fyne.TextStyle{TabWidth: 4}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSize, benchAdvance = painter.MeasureString(fonts, text, 14, style)
	}
}

func BenchmarkMeasureStringLong(b *testing.B) {
	fonts := benchmarkFontMap(fyne.TextStyle{})
	const text = "A longer paragraph of interface text that still fits on one line and represents labels, table cells, and status messages with enough glyphs to grow the shaping buffers."

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSize, benchAdvance = painter.MeasureString(fonts, text, 14, fyne.TextStyle{})
	}
}

func BenchmarkDrawStringASCII(b *testing.B) {
	fonts := benchmarkFontMap(fyne.TextStyle{})
	const text = "The quick brown fox jumps over the lazy dog 0123456789"
	dst := image.NewRGBA(image.Rect(0, 0, 512, 64))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		painter.DrawString(dst, text, color.Black, fonts, 14, 1, fyne.TextStyle{})
	}
	benchImage = dst
}

func BenchmarkDrawStringLong(b *testing.B) {
	fonts := benchmarkFontMap(fyne.TextStyle{})
	const text = "A longer paragraph of interface text that still fits on one line and represents labels, table cells, and status messages with enough glyphs to grow the shaping buffers."
	dst := image.NewRGBA(image.Rect(0, 0, 1400, 80))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		painter.DrawString(dst, text, color.Black, fonts, 14, 1, fyne.TextStyle{})
	}
	benchImage = dst
}
