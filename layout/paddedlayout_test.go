package layout_test

import (
	"image/color"
	"testing"

	"github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/layout"
	"github.com/alexballas/refyne/v2/theme"

	"github.com/stretchr/testify/assert"
)

func TestPaddedLayout(t *testing.T) {
	size := fyne.NewSize(100, 100)

	obj := canvas.NewRectangle(color.Black)
	container := &fyne.Container{
		Objects: []fyne.CanvasObject{obj},
	}
	container.Resize(size)

	layout.NewPaddedLayout().Layout(container.Objects, size)

	assert.Equal(t, obj.Size().Width, size.Width-2*theme.Padding())
	assert.Equal(t, obj.Size().Height, size.Height-2*theme.Padding())
}

func TestPaddedLayout_MinSize(t *testing.T) {
	text := canvas.NewText("FooBar", color.Black)
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	layoutMin := layout.NewPaddedLayout().MinSize(container.Objects)

	assert.Equal(t, minSize.Width+2*theme.Padding(), layoutMin.Width)
	assert.Equal(t, minSize.Height+2*theme.Padding(), layoutMin.Height)
}

func TestContainer_PaddedLayout_MinSize(t *testing.T) {
	text := canvas.NewText("FooBar", color.Black)
	minSize := text.MinSize()

	container := container.NewWithoutLayout(text)
	container.Layout = layout.NewPaddedLayout()
	layoutMin := container.MinSize()

	assert.Equal(t, minSize.Width+2*theme.Padding(), layoutMin.Width)
	assert.Equal(t, minSize.Height+2*theme.Padding(), layoutMin.Height)
}
