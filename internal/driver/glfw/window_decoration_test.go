//go:build !no_glfw && !mobile

package glfw

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/stretchr/testify/assert"
)

// newWindowDecoration touches global theme caches (via cache.OverrideTheme),
// which under the migrated_fynedo model are not internally locked. Every test
// here therefore builds and drives the decoration inside runOnMain so all of it
// runs on the GLFW main goroutine (TestMain), serialised with the draw loop and
// clean of data races.

func TestWindowDecoration_ButtonsInvokeCallbacks(t *testing.T) {
	var closed, minimized, maxToggled int
	runOnMain(func() {
		d := newWindowDecoration("My App", theme.FileApplicationIcon())
		d.onClose = func() { closed++ }
		d.onMinimize = func() { minimized++ }
		d.onMaximizeToggle = func() { maxToggled++ }

		d.closeButton.Tapped(&fyne.PointEvent{})
		d.minimizeButton.Tapped(&fyne.PointEvent{})
		d.maximizeButton.Tapped(&fyne.PointEvent{})
	})

	assert.Equal(t, 1, closed)
	assert.Equal(t, 1, minimized)
	assert.Equal(t, 1, maxToggled)
}

func TestWindowDecoration_SetTitle(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("Before", theme.FileApplicationIcon())
		d.SetTitle("After")
		assert.Equal(t, "After", d.titleLabel.Text)
	})
}

func TestWindowDecoration_BackgroundRoundedTopCorners(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("My App", theme.FileApplicationIcon())
		r := d.CreateRenderer().(*windowDecorationRenderer)
		assert.Equal(t, windowCornerRadius, r.bg.TopLeftCornerRadius)
		assert.Equal(t, windowCornerRadius, r.bg.TopRightCornerRadius)
		assert.Equal(t, float32(0), r.bg.BottomLeftCornerRadius)
		assert.Equal(t, float32(0), r.bg.BottomRightCornerRadius)

		d.SetCornersSquare(true)
		r.Refresh()
		assert.Equal(t, float32(0), r.bg.TopLeftCornerRadius)
		assert.Equal(t, float32(0), r.bg.TopRightCornerRadius)
	})
}

func TestWindowDecoration_ButtonsHaveCircularHighlight(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("My App", theme.FileApplicationIcon())
		assert.Equal(t, canvas.RadiusMaximum, d.minimizeButton.button.Theme().Size(theme.SizeNameInputRadius))
		assert.Equal(t, canvas.RadiusMaximum, d.maximizeButton.button.Theme().Size(theme.SizeNameInputRadius))
		assert.Equal(t, canvas.RadiusMaximum, d.closeButton.button.Theme().Size(theme.SizeNameInputRadius))

		r := d.closeButton.CreateRenderer().(*windowDecorationButtonRenderer)
		r.Layout(fyne.NewSquareSize(titleBarHeight))
		assert.Equal(t, fyne.NewSquareSize(titleBarHeight-windowDecorationButtonInset*2), d.closeButton.button.Size())
		assert.Equal(t, fyne.NewPos(windowDecorationButtonInset, windowDecorationButtonInset), d.closeButton.button.Position())
	})
}

func TestWindowDecoration_TitleCenteredInWindow(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("Centered", theme.FileApplicationIcon())
		r := d.CreateRenderer()

		size := fyne.NewSize(400, titleBarHeight)
		r.Layout(size)

		assert.Equal(t, fyne.TextAlignCenter, d.titleLabel.Alignment)
		assert.True(t, d.titleLabel.TextStyle.Bold)
		assert.Equal(t, size.Width/2, d.titleLabel.Position().X+d.titleLabel.Size().Width/2)
		assert.LessOrEqual(t, d.titleLabel.Position().X+d.titleLabel.Size().Width, d.minimizeButton.Position().X)
	})
}

func TestWindowDecoration_HidesTitleBeforeIcon(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("My App", theme.FileApplicationIcon())
		r := d.CreateRenderer()

		// The title is never truncated, so it reports the width of its full text.
		assert.Equal(t, fyne.TextTruncateOff, d.titleLabel.Truncation)
		titleMin := d.titleLabel.MinSize().Width
		// Window width at which the centered title exactly fits its full text.
		fitWidth := (titleBarHeight*3+theme.Padding())*2 + titleMin

		// Wider than a full fit: icon and title shown, the title at its full
		// width (no ellipsis) and clear of the left-most control button.
		r.Layout(fyne.NewSize(fitWidth+titleBarHeight, titleBarHeight))
		assert.True(t, d.icon.Visible())
		assert.True(t, d.titleLabel.Visible())
		assert.GreaterOrEqual(t, d.titleLabel.Size().Width, titleMin)
		assert.LessOrEqual(t, d.icon.Position().X+d.icon.Size().Width, d.minimizeButton.Position().X)
		assert.Less(t, d.titleLabel.Position().X+d.titleLabel.Size().Width, d.minimizeButton.Position().X)

		// Too narrow for the full title: it hides rather than truncating to
		// "...", while the icon still fits to the left of the controls and stays
		// visible (no overlap).
		r.Layout(fyne.NewSize(fitWidth-titleBarHeight, titleBarHeight))
		assert.True(t, d.icon.Visible())
		assert.False(t, d.titleLabel.Visible())
		assert.LessOrEqual(t, d.icon.Position().X+d.icon.Size().Width, d.minimizeButton.Position().X)

		// Very narrow: the controls would now reach the icon too, so it hides as
		// well, leaving just the controls laid out against the right edge.
		r.Layout(fyne.NewSize(titleBarHeight*3, titleBarHeight))
		assert.False(t, d.icon.Visible())
		assert.False(t, d.titleLabel.Visible())
		assert.Equal(t, titleBarHeight*3-titleBarHeight, d.closeButton.Position().X)

		// Widen again: the icon comes back first, then the full title once it
		// fits.
		r.Layout(fyne.NewSize(fitWidth-titleBarHeight, titleBarHeight))
		assert.True(t, d.icon.Visible())
		assert.False(t, d.titleLabel.Visible())
		r.Layout(fyne.NewSize(fitWidth+titleBarHeight, titleBarHeight))
		assert.True(t, d.icon.Visible())
		assert.True(t, d.titleLabel.Visible())
	})
}

func TestWindowDecoration_MinSizeHasTitleBarHeight(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("X", theme.FileApplicationIcon())
		assert.Greater(t, d.MinSize().Height, float32(0))
	})
}

func TestWindowDecoration_DragAndDoubleTap(t *testing.T) {
	var dragged, doubled int
	runOnMain(func() {
		d := newWindowDecoration("X", theme.FileApplicationIcon())
		d.onDragStart = func() { dragged++ }
		d.onDoubleTap = func() { doubled++ }

		d.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(2, 0)})
		d.DoubleTapped(&fyne.PointEvent{})
	})

	assert.Equal(t, 1, dragged)
	assert.Equal(t, 1, doubled)
}

func TestWindow_MaximizedUpdatesDecorationIcon(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("X", theme.FileApplicationIcon())
		w := &window{canvas: &glCanvas{decoration: d}}

		w.maximized(nil, true)
		assert.Equal(t, decorationRestore, d.maximizeButton.kind)

		w.maximized(nil, false)
		assert.Equal(t, decorationMaximize, d.maximizeButton.kind)
	})
}

func TestPointInWindowDecoration(t *testing.T) {
	runOnMain(func() {
		d := newWindowDecoration("X", theme.FileApplicationIcon())
		c := &glCanvas{decoration: d, size: fyne.NewSize(400, 200)}

		assert.True(t, pointInWindowDecoration(c, fyne.NewPos(0, 0)))
		assert.True(t, pointInWindowDecoration(c, fyne.NewPos(399, titleBarHeight-1)))
		assert.False(t, pointInWindowDecoration(c, fyne.NewPos(400, 0)))
		assert.False(t, pointInWindowDecoration(c, fyne.NewPos(0, titleBarHeight)))
		assert.False(t, pointInWindowDecoration(c, fyne.NewPos(0, -1)))
		assert.False(t, pointInWindowDecoration(&glCanvas{}, fyne.NewPos(0, 0)))
	})
}
