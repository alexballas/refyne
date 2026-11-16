package painter

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
)

var benchMeasured fyne.Size

// benchmarkMeasure measures s in the app's regular font map. With cached set
// the string is shaped once and then found in the shape cache, which is what
// measuring a label again costs; otherwise its cache entry is dropped before
// each measure, which is what measuring a new string costs.
func benchmarkMeasure(b *testing.B, s string, cached bool) {
	face := CachedFontFace(fyne.TextStyle{}, nil, nil)
	key := shapeKey{text: s, faces: face.Fonts.(*dynamicFontMap), size: float32ToFixed266(14)}
	benchMeasured, _ = MeasureString(face.Fonts, s, 14, fyne.TextStyle{}) // look up any fallback fonts

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !cached {
			fontUseLock.Lock()
			delete(shapeCache, key)
			fontUseLock.Unlock()
		}
		benchMeasured, _ = MeasureString(face.Fonts, s, 14, fyne.TextStyle{})
	}
}

const (
	benchLatin     = "Remember Playback Position"
	benchLatinLong = "Some DLNA devices don't support gapless playback. If 'Auto-Play Next File' isn't working properly, try turning it off."
	benchCJK       = "一些 DLNA 设备不支持无缝播放。如果“自动播放下一个文件”无法正常工作，请尝试关闭它。"
)

func BenchmarkMeasureString_LatinUncached(b *testing.B) { benchmarkMeasure(b, benchLatin, false) }

func BenchmarkMeasureString_LatinLongUncached(b *testing.B) {
	benchmarkMeasure(b, benchLatinLong, false)
}

func BenchmarkMeasureString_LatinCached(b *testing.B) { benchmarkMeasure(b, benchLatin, true) }
func BenchmarkMeasureString_CJKUncached(b *testing.B) { benchmarkMeasure(b, benchCJK, false) }
