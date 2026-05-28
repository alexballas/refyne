package software

import (
	"github.com/alexballas/refyne/v2/internal/painter/software"
	"github.com/alexballas/refyne/v2/test"
)

// NewCanvas creates a new canvas in memory that can render without hardware support.
func NewCanvas() test.WindowlessCanvas {
	return test.NewCanvasWithPainter(software.NewPainter())
}

// NewTransparentCanvas creates a new canvas in memory that can render without hardware support without a background color.
//
// Since: 2.2
func NewTransparentCanvas() test.WindowlessCanvas {
	return test.NewTransparentCanvasWithPainter(software.NewPainter())
}
