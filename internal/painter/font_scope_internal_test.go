package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/theme"
)

// wrapTheme stands in for the theme container.ThemeOverride hands each scope:
// a new wrapper around the same theme on every Refresh.
type wrapTheme struct{ fyne.Theme }

var refreshedStyles = []fyne.TextStyle{{}, {Bold: true}, {Italic: true}, {Monospace: true}, {Symbol: true}}

// TestCachedFontFace_RepeatedOverrideDoesNotGrow refreshes an overridden object
// many times, as ThemeOverride.Refresh does, measuring its text after each one.
// Each refresh mints a new scope id; none of them may add font faces, or shaped
// strings and spaces keyed by those faces, and the scopes they leave behind
// must not pile up.
func TestCachedFontFace_RepeatedOverrideDoesNotGrow(t *testing.T) {
	ClearFontCache()
	text := canvas.NewText("refreshed", nil)

	measure := func() {
		for _, style := range refreshedStyles {
			face := CachedFontFace(style, nil, text)
			MeasureString(face.Fonts, text.Text, 14, style)
		}
	}

	cache.OverrideTheme(text, &wrapTheme{theme.DefaultTheme()})
	measure()
	faces, shapes, spaces := fontFacesLen(), shapeCacheLen(), spaceCacheLen()
	assert.NotZero(t, faces)

	// enough refreshes to pass fontCacheMax several times over
	refreshes := 2 * fontCacheMax
	for i := 0; i < refreshes; i++ {
		cache.OverrideTheme(text, &wrapTheme{theme.DefaultTheme()})
		measure()
	}
	assert.Equal(t, faces, fontFacesLen())
	assert.LessOrEqual(t, fontCacheLen(), fontCacheMax)
	assert.Equal(t, shapes, shapeCacheLen())
	assert.Equal(t, spaces, spaceCacheLen())
}

// freshTheme returns a new resource from every Font call, as a JSON theme with
// font URIs did by reading the file each time.
type freshTheme struct{ fyne.Theme }

func (f *freshTheme) Font(s fyne.TextStyle) fyne.Resource {
	r := f.Theme.Font(s)
	return fyne.NewStaticResource(r.Name(), r.Content())
}

// TestCachedFontFace_FreshFontOverrideIsBounded refreshes an override whose
// theme never returns the same font twice, so no two scopes can share an item.
// The scopes and fonts left behind must still not pile up.
func TestCachedFontFace_FreshFontOverrideIsBounded(t *testing.T) {
	ClearFontCache()
	t.Cleanup(ClearFontCache) // drop the fonts parsed for each fresh resource
	text := canvas.NewText("refreshed", nil)

	for i := 0; i < fontCacheMax+fontFacesMax; i++ {
		cache.OverrideTheme(text, &freshTheme{theme.DefaultTheme()})
		face := CachedFontFace(fyne.TextStyle{}, nil, text)
		assert.Same(t, face, CachedFontFace(fyne.TextStyle{}, nil, text))
	}
	assert.LessOrEqual(t, fontCacheLen(), fontCacheMax)
	assert.LessOrEqual(t, fontFacesLen(), fontFacesMax)
}

// TestCachedFontFace_DroppedScopesKeepAppFaces fills fontCache with override
// scopes until it drops them, and checks the app's own item survives that.
func TestCachedFontFace_DroppedScopesKeepAppFaces(t *testing.T) {
	ClearFontCache()
	app := CachedFontFace(fyne.TextStyle{}, nil, nil)
	text := canvas.NewText("refreshed", nil)
	cache.OverrideTheme(text, &wrapTheme{theme.DefaultTheme()})
	over := CachedFontFace(fyne.TextStyle{Bold: true}, nil, text)

	for i := 0; i < fontCacheMax; i++ {
		cache.OverrideTheme(text, &wrapTheme{theme.DefaultTheme()})
		CachedFontFace(fyne.TextStyle{Bold: true}, nil, text)
	}

	fontCacheLock.RLock()
	_, kept := fontCache[cacheID{}]
	fontCacheLock.RUnlock()
	assert.True(t, kept)
	assert.Same(t, app, CachedFontFace(fyne.TextStyle{}, nil, nil))
	assert.Same(t, over, CachedFontFace(fyne.TextStyle{Bold: true}, nil, text))
}

func fontCacheLen() int {
	fontCacheLock.RLock()
	defer fontCacheLock.RUnlock()
	return len(fontCache)
}

func fontFacesLen() int {
	fontCacheLock.RLock()
	defer fontCacheLock.RUnlock()
	return len(fontFaces)
}

func shapeCacheLen() int {
	fontUseLock.Lock()
	defer fontUseLock.Unlock()
	return len(shapeCache)
}

func spaceCacheLen() int {
	fontUseLock.Lock()
	defer fontUseLock.Unlock()
	return len(spaceCache)
}

type sliceRes struct{ b []byte }

func (sliceRes) Name() string      { return "slice" }
func (r sliceRes) Content() []byte { return r.b }

type wrappedRes struct{ fyne.Resource }

type arrayRes struct{ a [1]fyne.Resource }

func (arrayRes) Name() string    { return "array" }
func (arrayRes) Content() []byte { return nil }

type funcRes struct {
	fyne.Resource
	f func()
}

// hashableCases are resources a theme might return, and whether each can be a
// map key without panicking.
func hashableCases() map[string]struct {
	res  fyne.Resource
	want bool
} {
	static := theme.DefaultTextFont()
	return map[string]struct {
		res  fyne.Resource
		want bool
	}{
		"nil":                {nil, false},
		"static":             {static, true},
		"slice field":        {sliceRes{}, false},
		"wrapped static":     {wrappedRes{static}, true},
		"wrapped nil":        {wrappedRes{}, true},
		"wrapped slice":      {wrappedRes{sliceRes{}}, false},
		"wrapped wrapped":    {wrappedRes{wrappedRes{sliceRes{}}}, false},
		"array of static":    {arrayRes{[1]fyne.Resource{static}}, true},
		"array of slice":     {arrayRes{[1]fyne.Resource{sliceRes{}}}, false},
		"func field":         {funcRes{Resource: static}, false},
		"pointer to slicey":  {&sliceRes{}, true},
		"wrapped to pointer": {wrappedRes{&sliceRes{}}, true},
	}
}

func TestHashable(t *testing.T) {
	for name, tt := range hashableCases() {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, hashable(tt.res))
			if tt.res != nil {
				assert.Equal(t, usableAsMapKey(tt.res), hashable(tt.res), "hashable should agree with a map")
			}
		})
	}
}

// usableAsMapKey is whether storing r as a map key does not panic.
func usableAsMapKey(r fyne.Resource) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	m := map[fyne.Resource]struct{}{}
	m[r] = struct{}{}
	return true
}
