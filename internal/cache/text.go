package cache

import (
	"image/color"
	"sync"
	"time"

	fyne "github.com/alexballas/refyne/v2"
)

// fontSizeCache is read by every canvas.Text on every frame the canvas is
// walked, so it is a typed map: async.Map keys by interface, which boxes the
// struct (an allocation) and hashes it through its type on each lookup.
var (
	fontSizeLock  sync.Mutex
	fontSizeCache = map[fontSizeEntry]*fontMetric{}
)

type fontMetric struct {
	expiringCache
	size     fyne.Size
	baseLine float32
}

type fontSizeEntry struct {
	Text   string
	Size   float32
	Style  fyne.TextStyle
	Source string
}

type FontCacheEntry struct {
	fontSizeEntry

	Canvas fyne.Canvas
	Color  color.Color
}

// GetFontMetrics looks up a calculated size and baseline required for the specified text parameters.
func GetFontMetrics(text string, fontSize float32, style fyne.TextStyle, source fyne.Resource) (size fyne.Size, base float32) {
	name := ""
	if source != nil {
		name = source.Name()
	}
	fontSizeLock.Lock()
	defer fontSizeLock.Unlock()
	ret, ok := fontSizeCache[fontSizeEntry{text, fontSize, style, name}]
	if !ok {
		return fyne.Size{Width: 0, Height: 0}, 0
	}
	ret.setAlive()
	return ret.size, ret.baseLine
}

// SetFontMetrics stores a calculated font size and baseline for parameters that were missing from the cache.
func SetFontMetrics(text string, fontSize float32, style fyne.TextStyle, source fyne.Resource, size fyne.Size, base float32) {
	name := ""
	if source != nil {
		name = source.Name()
	}
	ent := fontSizeEntry{text, fontSize, style, name}
	metric := &fontMetric{size: size, baseLine: base}
	metric.setAlive()
	fontSizeLock.Lock()
	fontSizeCache[ent] = metric
	fontSizeLock.Unlock()
}

// ClearFontMetrics clears cached font metrics for tests and benchmarks.
func ClearFontMetrics() {
	fontSizeLock.Lock()
	fontSizeCache = map[fontSizeEntry]*fontMetric{}
	fontSizeLock.Unlock()
}

// destroyExpiredFontMetrics destroys expired fontSizeCache entries
func destroyExpiredFontMetrics(now time.Time) {
	fontSizeLock.Lock()
	for k, v := range fontSizeCache {
		if v.isExpired(now) {
			delete(fontSizeCache, k)
		}
	}
	fontSizeLock.Unlock()
}
