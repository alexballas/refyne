package canvas_test

import (
	"image/color"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"

	"github.com/stretchr/testify/assert"
)

func TestArbitraryPolygon_NewAndDefaults(t *testing.T) {
	points := []fyne.Position{
		{X: 0, Y: 0},
		{X: 100, Y: 0},
		{X: 100, Y: 100},
	}
	fill := color.White
	p := canvas.NewArbitraryPolygon(points, fill)

	assert.Equal(t, points, p.Points)
	assert.Equal(t, fill, p.FillColor)
	assert.Nil(t, p.StrokeColor)
	assert.Equal(t, float32(0), p.StrokeWidth)
}

func TestArbitraryPolygon_Properties(t *testing.T) {
	p := canvas.NewArbitraryPolygon(nil, color.Black)
	p.NormalizedPoints = true
	p.StrokeWidth = 2.0
	p.StrokeColor = color.NRGBA{R: 255, G: 0, B: 0, A: 255}
	p.CornerRadii = []float32{5, 10, 5}

	assert.True(t, p.NormalizedPoints)
	assert.Equal(t, float32(2.0), p.StrokeWidth)
	assert.Equal(t, color.NRGBA{R: 255, G: 0, B: 0, A: 255}, p.StrokeColor)
	assert.Equal(t, []float32{5, 10, 5}, p.CornerRadii)
}
