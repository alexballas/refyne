package cache

import (
	"sync/atomic"

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
	_, attached := canvasEntry(obj, c)
	return attached
}

// CanvasRef holds an object's canvas registration between repaints.
type CanvasRef struct {
	info *canvasInfo
}

// AttachCanvasRef is AttachCanvas skipping the cache lookup while ref still
// holds the current registration of obj with c.
func AttachCanvasRef(ref *CanvasRef, obj fyne.CanvasObject, c fyne.Canvas) bool {
	if info := ref.info; info != nil && info.canvas == c && !info.replaced.Load() {
		info.setAlive()
		return false
	}
	info, attached := canvasEntry(obj, c)
	ref.info = info
	return attached
}

func canvasEntry(obj fyne.CanvasObject, c fyne.Canvas) (*canvasInfo, bool) {
	old, found := canvases.Load(obj)
	if found && old.canvas == c {
		old.setAlive()
		return old, false
	}

	// not cached, or moved to a different canvas - an object may only be on one
	// canvas at a time, so replace the entry and tell any ref holding the old one.
	if found {
		old.replaced.Store(true)
	}
	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()

	canvases.Store(obj, cinfo)
	return cinfo, true
}

type canvasInfo struct {
	expiringCache
	canvas   fyne.Canvas
	replaced atomic.Bool
}
