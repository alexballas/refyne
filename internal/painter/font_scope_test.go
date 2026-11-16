package painter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/internal/painter"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/theme"
)

type fontTheme struct {
	fyne.Theme
	font fyne.Resource
}

func (f fontTheme) Font(fyne.TextStyle) fyne.Resource { return f.font }

func TestCachedFontFace_OverrideScopesShareFaces(t *testing.T) {
	painter.ClearFontCache()
	app := painter.CachedFontFace(fyne.TextStyle{}, nil, canvas.NewText("app", nil))

	a, b := canvas.NewText("a", nil), canvas.NewText("b", nil)
	cache.OverrideTheme(a, fontTheme{theme.DefaultTheme(), theme.DefaultTextFont()})
	cache.OverrideTheme(b, fontTheme{theme.DefaultTheme(), theme.DefaultTextFont()})
	assert.NotEqual(t, cache.WidgetScopeID(a), cache.WidgetScopeID(b))

	assert.Same(t, painter.CachedFontFace(fyne.TextStyle{}, nil, a), painter.CachedFontFace(fyne.TextStyle{}, nil, b))
	assert.Same(t, app, painter.CachedFontFace(fyne.TextStyle{}, nil, a))
}

func TestCachedFontFace_OverrideScopeWithOtherFont(t *testing.T) {
	painter.ClearFontCache()
	app := painter.CachedFontFace(fyne.TextStyle{}, nil, nil)

	mono := canvas.NewText("mono", nil)
	cache.OverrideTheme(mono, fontTheme{theme.DefaultTheme(), theme.DefaultTextMonospaceFont()})
	assert.NotSame(t, app, painter.CachedFontFace(fyne.TextStyle{}, nil, mono))
}

func TestCachedFontFace_RefreshedOverrideKeepsFaces(t *testing.T) {
	painter.ClearFontCache()
	text := canvas.NewText("text", nil)
	th := fontTheme{theme.DefaultTheme(), theme.DefaultTextFont()}
	cache.OverrideTheme(text, th)
	before := painter.CachedFontFace(fyne.TextStyle{Bold: true}, nil, text)

	cache.OverrideTheme(text, th) // as ThemeOverride.Refresh does
	assert.Same(t, before, painter.CachedFontFace(fyne.TextStyle{Bold: true}, nil, text))
}

// countingTheme returns a new resource from every Font call, as a JSON theme
// with font URIs did by reading the file each time, and counts the calls.
type countingTheme struct {
	fyne.Theme
	calls *int
}

func (c countingTheme) Font(s fyne.TextStyle) fyne.Resource {
	*c.calls++
	r := c.Theme.Font(s)
	return fyne.NewStaticResource(r.Name(), r.Content())
}

func TestCachedFontFace_FreshFontLoadedOncePerScope(t *testing.T) {
	painter.ClearFontCache()
	calls := 0
	text := canvas.NewText("text", nil)
	cache.OverrideTheme(text, countingTheme{theme.DefaultTheme(), &calls})

	face := painter.CachedFontFace(fyne.TextStyle{}, nil, text)
	for i := 0; i < 50; i++ {
		assert.Same(t, face, painter.CachedFontFace(fyne.TextStyle{}, nil, text))
	}
	assert.Equal(t, 1, calls)
}

func TestCachedFontFace_AppThemeWithFreshFont(t *testing.T) {
	a := test.NewTempApp(t)
	calls := 0
	a.Settings().SetTheme(countingTheme{theme.DefaultTheme(), &calls})
	painter.ClearFontCache()
	calls = 0

	text := canvas.NewText("text", nil)
	w := test.NewWindow(text)
	defer w.Close()
	face := painter.CachedFontFace(fyne.TextStyle{}, nil, text)
	for i := 0; i < 10; i++ {
		w.Canvas().Capture()
	}
	assert.Same(t, face, painter.CachedFontFace(fyne.TextStyle{}, nil, text))
	assert.Equal(t, 1, calls)
}

// BenchmarkCachedFontFace_Hit is the lookup every text draw makes, for text
// with and without an overridden theme.
func BenchmarkCachedFontFace_Hit(b *testing.B) {
	app := canvas.NewText("app", nil)
	over := canvas.NewText("over", nil)
	cache.OverrideTheme(over, fontTheme{theme.DefaultTheme(), theme.DefaultTextFont()})
	painter.CachedFontFace(fyne.TextStyle{}, nil, app)
	painter.CachedFontFace(fyne.TextStyle{}, nil, over)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		painter.CachedFontFace(fyne.TextStyle{}, nil, app)
		painter.CachedFontFace(fyne.TextStyle{}, nil, over)
	}
}

// A theme may return a Resource whose concrete type cannot be a map key.
type sliceResource struct{ content []byte }

func (sliceResource) Name() string      { return "slice.ttf" }
func (r sliceResource) Content() []byte { return r.content }

func TestCachedFontFace_NonComparableFontKeepsScope(t *testing.T) {
	painter.ClearFontCache()
	res := sliceResource{theme.DefaultTextFont().Content()}
	a, b := canvas.NewText("a", nil), canvas.NewText("b", nil)
	cache.OverrideTheme(a, fontTheme{theme.DefaultTheme(), res})
	cache.OverrideTheme(b, fontTheme{theme.DefaultTheme(), res})

	fa := painter.CachedFontFace(fyne.TextStyle{}, nil, a)
	assert.NotNil(t, fa)
	assert.Same(t, fa, painter.CachedFontFace(fyne.TextStyle{}, nil, a))
	assert.NotSame(t, fa, painter.CachedFontFace(fyne.TextStyle{}, nil, b))
}

// A struct type is comparable even when the value held in its interface field is not.
type wrappedResource struct{ fyne.Resource }

func TestCachedFontFace_WrappedNonComparableFontKeepsScope(t *testing.T) {
	painter.ClearFontCache()
	res := wrappedResource{sliceResource{theme.DefaultTextFont().Content()}}
	a, b := canvas.NewText("a", nil), canvas.NewText("b", nil)
	cache.OverrideTheme(a, fontTheme{theme.DefaultTheme(), res})
	cache.OverrideTheme(b, fontTheme{theme.DefaultTheme(), res})

	fa := painter.CachedFontFace(fyne.TextStyle{}, nil, a)
	assert.NotNil(t, fa)
	assert.Same(t, fa, painter.CachedFontFace(fyne.TextStyle{}, nil, a))
	assert.NotSame(t, fa, painter.CachedFontFace(fyne.TextStyle{}, nil, b))
}
