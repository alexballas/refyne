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
