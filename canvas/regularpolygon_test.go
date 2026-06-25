package canvas_test

import (
	"image/color"
	"testing"

	"github.com/alexballas/refyne/v2/canvas"

	"github.com/stretchr/testify/assert"
)

func TestRegularPolygon_FillColorSides(t *testing.T) {
	c := color.White
	polygon := canvas.NewRegularPolygon(3, c)

	assert.Equal(t, c, polygon.FillColor)
	assert.Equal(t, uint(3), polygon.Sides)
}

func TestRegularPolygon_Properties(t *testing.T) {
	polygon := canvas.NewRegularPolygon(5, color.Black)
	polygon.StrokeColor = color.White
	polygon.StrokeWidth = 2
	polygon.CornerRadius = canvas.RadiusMaximum
	polygon.Angle = -45

	assert.Equal(t, color.White, polygon.StrokeColor)
	assert.Equal(t, float32(2), polygon.StrokeWidth)
	assert.Equal(t, float32(canvas.RadiusMaximum), polygon.CornerRadius)
	assert.Equal(t, float32(-45), polygon.Angle)
}
