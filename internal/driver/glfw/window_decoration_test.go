//go:build !wasm && !test_web_driver

package glfw

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/stretchr/testify/assert"
)

func TestWindowDecoration_ButtonsInvokeCallbacks(t *testing.T) {
	var closed, minimized, maxToggled int
	d := newWindowDecoration("My App", theme.FyneLogo())
	d.onClose = func() { closed++ }
	d.onMinimize = func() { minimized++ }
	d.onMaximizeToggle = func() { maxToggled++ }

	w := test.NewWindow(d)
	defer w.Close()
	w.Resize(fyne.NewSize(400, 200))

	test.Tap(d.closeButton)
	test.Tap(d.minimizeButton)
	test.Tap(d.maximizeButton)

	assert.Equal(t, 1, closed)
	assert.Equal(t, 1, minimized)
	assert.Equal(t, 1, maxToggled)
}

func TestWindowDecoration_SetTitle(t *testing.T) {
	d := newWindowDecoration("Before", theme.FyneLogo())
	d.SetTitle("After")
	assert.Equal(t, "After", d.titleLabel.Text)
}

func TestWindowDecoration_BackgroundRoundedTopCorners(t *testing.T) {
	d := newWindowDecoration("My App", theme.FyneLogo())
	r := d.CreateRenderer().(*windowDecorationRenderer)
	assert.Equal(t, windowCornerRadius, r.bg.TopLeftCornerRadius)
	assert.Equal(t, windowCornerRadius, r.bg.TopRightCornerRadius)
	assert.Equal(t, float32(0), r.bg.BottomLeftCornerRadius)
	assert.Equal(t, float32(0), r.bg.BottomRightCornerRadius)

	d.SetCornersSquare(true)
	r.Refresh()
	assert.Equal(t, float32(0), r.bg.TopLeftCornerRadius)
	assert.Equal(t, float32(0), r.bg.TopRightCornerRadius)
}

func TestWindowDecoration_ButtonsHaveCircularHighlight(t *testing.T) {
	d := newWindowDecoration("My App", theme.FyneLogo())

	assert.Equal(t, canvas.RadiusMaximum, d.minimizeButton.button.Theme().Size(theme.SizeNameInputRadius))
	assert.Equal(t, canvas.RadiusMaximum, d.maximizeButton.button.Theme().Size(theme.SizeNameInputRadius))
	assert.Equal(t, canvas.RadiusMaximum, d.closeButton.button.Theme().Size(theme.SizeNameInputRadius))

	r := d.closeButton.CreateRenderer().(*windowDecorationButtonRenderer)
	r.Layout(fyne.NewSquareSize(titleBarHeight))
	assert.Equal(t, fyne.NewSquareSize(titleBarHeight-windowDecorationButtonInset*2), d.closeButton.button.Size())
	assert.Equal(t, fyne.NewPos(windowDecorationButtonInset, windowDecorationButtonInset), d.closeButton.button.Position())
}

func TestWindowDecoration_TitleCenteredInWindow(t *testing.T) {
	d := newWindowDecoration("Centered", theme.FyneLogo())
	r := test.TempWidgetRenderer(t, d)

	size := fyne.NewSize(400, titleBarHeight)
	r.Layout(size)

	assert.Equal(t, fyne.TextAlignCenter, d.titleLabel.Alignment)
	assert.True(t, d.titleLabel.TextStyle.Bold)
	assert.Equal(t, size.Width/2, d.titleLabel.Position().X+d.titleLabel.Size().Width/2)
	assert.LessOrEqual(t, d.titleLabel.Position().X+d.titleLabel.Size().Width, d.minimizeButton.Position().X)
}

func TestWindowDecoration_MinSizeHasTitleBarHeight(t *testing.T) {
	d := newWindowDecoration("X", theme.FyneLogo())
	assert.Greater(t, d.MinSize().Height, float32(0))
}

func TestWindowDecoration_DragAndDoubleTap(t *testing.T) {
	var dragged, doubled int
	d := newWindowDecoration("X", theme.FyneLogo())
	d.onDragStart = func() { dragged++ }
	d.onDoubleTap = func() { doubled++ }

	d.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(2, 0)})
	d.DoubleTapped(&fyne.PointEvent{})

	assert.Equal(t, 1, dragged)
	assert.Equal(t, 1, doubled)
}

func TestWindow_MaximizedUpdatesDecorationIcon(t *testing.T) {
	d := newWindowDecoration("X", theme.FyneLogo())
	w := &window{canvas: &glCanvas{decoration: d}}

	w.maximized(nil, true)
	assert.Equal(t, theme.ViewRestoreIcon(), d.maximizeButton.button.Icon)

	w.maximized(nil, false)
	assert.Equal(t, theme.WindowMaximizeIcon(), d.maximizeButton.button.Icon)
}

func TestPointInWindowDecoration(t *testing.T) {
	d := newWindowDecoration("X", theme.FyneLogo())
	c := &glCanvas{decoration: d, size: fyne.NewSize(400, 200)}

	assert.True(t, pointInWindowDecoration(c, fyne.NewPos(0, 0)))
	assert.True(t, pointInWindowDecoration(c, fyne.NewPos(399, titleBarHeight-1)))
	assert.False(t, pointInWindowDecoration(c, fyne.NewPos(400, 0)))
	assert.False(t, pointInWindowDecoration(c, fyne.NewPos(0, titleBarHeight)))
	assert.False(t, pointInWindowDecoration(c, fyne.NewPos(0, -1)))
	assert.False(t, pointInWindowDecoration(&glCanvas{}, fyne.NewPos(0, 0)))
}
