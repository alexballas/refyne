package common

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/cache"
)

// CanvasForObject returns the canvas for the specified object.
func CanvasForObject(obj fyne.CanvasObject) fyne.Canvas {
	return cache.GetCanvasForObject(obj)
}
