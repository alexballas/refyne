package cache

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/async"
)

var canvases async.Map[fyne.CanvasObject, *canvasInfo]

// GetCanvasForObject returns the canvas for the specified object.
func GetCanvasForObject(obj fyne.CanvasObject) fyne.Canvas {
	cinfo, ok := canvases.Load(obj)
	if cinfo == nil || !ok {
		return nil
	}
	cinfo.setAlive()
	return cinfo.canvas
}

// SetCanvasForObject sets the canvas for the specified object.
// The passed function will be called if the item was not previously attached to this canvas
func SetCanvasForObject(obj fyne.CanvasObject, c fyne.Canvas, setup func()) {
	if AttachCanvas(obj, c) && setup != nil {
		setup()
	}
}

// AttachCanvas sets the canvas for the specified object and reports whether the
// object was newly attached (or moved from a different canvas), i.e. whether
// any per-attachment setup work is needed. The already-attached path performs
// no allocation, so callers on hot paths can gate setup work on the result
// instead of passing a closure to SetCanvasForObject.
func AttachCanvas(obj fyne.CanvasObject, c fyne.Canvas) bool {
	if old, ok := canvases.Load(obj); ok && old.canvas == c {
		old.setAlive()
		return false
	}

	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()

	old, found := canvases.LoadOrStore(obj, cinfo)
	return !found || old.canvas != c
}

type canvasInfo struct {
	expiringCache
	canvas fyne.Canvas
}
