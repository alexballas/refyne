package canvas_test

import (
	"image/color"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"

	"github.com/stretchr/testify/assert"
)

func TestEllipse_MinSize(t *testing.T) {
	ellipse := canvas.NewEllipse(color.Black)
	min := ellipse.MinSize()

	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestEllipse_FillColor(t *testing.T) {
	c := color.White
	ellipse := canvas.NewEllipse(c)

	assert.Equal(t, c, ellipse.FillColor)
}

func TestEllipse_Resize(t *testing.T) {
	targetWidth := float32(50)
	targetHeight := float32(80)
	ellipse := canvas.NewEllipse(color.White)
	start := ellipse.Size()
	assert.Zero(t, start.Height)
	assert.Zero(t, start.Width)

	ellipse.Resize(fyne.NewSize(targetWidth, targetHeight))
	target := ellipse.Size()
	assert.Equal(t, targetHeight, target.Height)
	assert.Equal(t, targetWidth, target.Width)
}

func TestEllipse_Move(t *testing.T) {
	ellipse := canvas.NewEllipse(color.White)
	ellipse.Resize(fyne.NewSize(80, 50))

	start := fyne.Position{X: 0, Y: 0}
	assert.Equal(t, start, ellipse.Position())

	target := fyne.Position{X: 10, Y: 75}
	ellipse.Move(target)
	assert.Equal(t, target, ellipse.Position())
}
