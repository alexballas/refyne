package cache

import (
	"image"
	"image/color"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/async"
	"github.com/alexballas/refyne/v2/internal/svg"
)

var (
	svgs          async.Map[svgCacheKey, *svgInfo]
	colorizedSvgs async.Map[svgSourceKey, *svgSourceInfo]
	svgDecoders   async.Map[svgSourceKey, *svgDecoderInfo]
)

// svgCacheKey identifies a rasterized SVG. The size is part of the key so the
// same icon rendered at multiple sizes (toolbar vs list row, mixed DPI) keeps
// one cache entry per size instead of evicting each other on every lookup.
type svgCacheKey struct {
	name string
	w, h int
}

// svgSourceKey identifies SVG source content derived from a named resource,
// optionally colorized. The resolved color is part of the key so theme or
// variant changes produce new entries rather than stale hits.
type svgSourceKey struct {
	name      string
	colorized bool
	color     [4]uint32
}

func newSvgSourceKey(name string, c color.Color) svgSourceKey {
	key := svgSourceKey{name: name}
	if c != nil {
		key.colorized = true
		r, g, b, a := c.RGBA()
		key.color = [4]uint32{r, g, b, a}
	}
	return key
}

// GetSvg gets svg image from cache if it exists.
func GetSvg(name string, o fyne.CanvasObject, w int, h int) *image.NRGBA {
	svginfo, ok := svgs.Load(svgCacheKey{name: overriddenName(name, o), w: w, h: h})
	if !ok || svginfo == nil {
		return nil
	}

	svginfo.setAlive()
	return svginfo.pix
}

// SetSvg sets a svg into the cache map.
func SetSvg(name string, o fyne.CanvasObject, pix *image.NRGBA, w int, h int) {
	sinfo := &svgInfo{pix: pix}
	sinfo.setAlive()
	svgs.Store(svgCacheKey{name: overriddenName(name, o), w: w, h: h}, sinfo)
}

// GetColorizedSvg returns the cached result of colorizing the named SVG source
// with the given color, if present.
func GetColorizedSvg(name string, c color.Color) ([]byte, bool) {
	info, ok := colorizedSvgs.Load(newSvgSourceKey(name, c))
	if !ok || info == nil {
		return nil, false
	}
	info.setAlive()
	return info.content, true
}

// SetColorizedSvg stores the result of colorizing the named SVG source with the
// given color.
func SetColorizedSvg(name string, c color.Color, content []byte) {
	info := &svgSourceInfo{content: content}
	info.setAlive()
	colorizedSvgs.Store(newSvgSourceKey(name, c), info)
}

// GetSvgDecoder returns the cached parsed decoder for the named SVG source and
// colorize color, if present. The decoder mutates internal state while drawing,
// so callers must only use it from the main (render) goroutine.
func GetSvgDecoder(name string, c color.Color) (*svg.Decoder, bool) {
	info, ok := svgDecoders.Load(newSvgSourceKey(name, c))
	if !ok || info == nil {
		return nil, false
	}
	info.setAlive()
	return info.decoder, true
}

// SetSvgDecoder stores the parsed decoder for the named SVG source and colorize
// color.
func SetSvgDecoder(name string, c color.Color, decoder *svg.Decoder) {
	info := &svgDecoderInfo{decoder: decoder}
	info.setAlive()
	svgDecoders.Store(newSvgSourceKey(name, c), info)
}

type svgInfo struct {
	expiringCache
	pix *image.NRGBA
}

type svgSourceInfo struct {
	expiringCache
	content []byte
}

type svgDecoderInfo struct {
	expiringCache
	decoder *svg.Decoder
}

// destroyExpiredSvgs destroys expired svgs cache data.
func destroyExpiredSvgs(now time.Time) {
	svgs.Range(func(key svgCacheKey, sinfo *svgInfo) bool {
		if sinfo.isExpired(now) {
			svgs.Delete(key)
		}
		return true
	})
	colorizedSvgs.Range(func(key svgSourceKey, info *svgSourceInfo) bool {
		if info.isExpired(now) {
			colorizedSvgs.Delete(key)
		}
		return true
	})
	svgDecoders.Range(func(key svgSourceKey, info *svgDecoderInfo) bool {
		if info.isExpired(now) {
			svgDecoders.Delete(key)
		}
		return true
	})
}

func overriddenName(name string, o fyne.CanvasObject) string {
	if o != nil { // for overridden themes get the cache key right
		if over, ok := overrides.Load(o); ok {
			return over.cacheID + name
		}
	}

	return name
}
