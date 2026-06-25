package canvas_test

import (
	"testing"

	"github.com/alexballas/refyne/v2/canvas"

	"github.com/stretchr/testify/assert"
)

func TestNewBlur(t *testing.T) {
	blur := canvas.NewBlur(12)

	assert.Equal(t, float32(12), blur.Radius)
	assert.True(t, blur.Visible())
}
