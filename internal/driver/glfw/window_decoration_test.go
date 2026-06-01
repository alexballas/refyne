//go:build !wasm && !test_web_driver

package glfw

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
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
	assert.Equal(t, theme.ViewRestoreIcon(), d.maximizeButton.Icon)

	w.maximized(nil, false)
	assert.Equal(t, theme.WindowMaximizeIcon(), d.maximizeButton.Icon)
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
