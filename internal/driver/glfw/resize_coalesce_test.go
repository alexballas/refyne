//go:build !no_glfw && !mobile && (linux || freebsd || openbsd || netbsd)

package glfw

import (
	"math"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/widget"
)

// TestResizeCoalescing verifies that a burst of configure events delivered to the
// resized callback does NOT resize synchronously (it is coalesced), and that a
// single applyPendingResize then applies the latest size - so N configure events
// cost one canvas.Resize per frame instead of N.
func TestResizeCoalescing(t *testing.T) {
	w := createWindow("Test")
	w.SetPadded(false)
	w.SetContent(widget.NewLabel("content"))
	base := fyne.NewSize(1000, 800)
	w.Resize(base)
	ensureCanvasSize(t, w, base)
	repaintWindow(w)

	var sizeBefore, sizeMid, sizeAfter, expected fyne.Size
	var pendingAfterBurst bool

	// One atomic main-thread block so the live draw loop cannot interleave and
	// apply the pending resize before we observe it.
	runOnMain(func() {
		win := w.window
		win.visible = true
		win.fixedSize = false
		win.fullScreen = false
		win.pendingResize = false // clear any stale state from setup

		sizeBefore = win.canvas.Size()

		// Simulate a fast drag: several configure events between two frames.
		win.resized(nil, 1100, 700)
		win.resized(nil, 1200, 750)
		win.resized(nil, 1300, 820)

		sizeMid = win.canvas.Size()           // must be unchanged: coalesced
		pendingAfterBurst = win.pendingResize // exactly one pending resize

		cs := win.computeCanvasSize(1300, 820)
		expected = fyne.NewSize(
			float32(math.Ceil(float64(cs.Width))),
			float32(math.Ceil(float64(cs.Height))),
		)

		win.applyPendingResize() // the draw loop does this once per frame
		sizeAfter = win.canvas.Size()
	})

	if sizeMid != sizeBefore {
		t.Errorf("configure burst resized synchronously (not coalesced): before=%v mid=%v", sizeBefore, sizeMid)
	}
	if !pendingAfterBurst {
		t.Errorf("expected a single pending coalesced resize after the burst")
	}
	if sizeAfter != expected {
		t.Errorf("applyPendingResize did not apply the latest size: got=%v want=%v", sizeAfter, expected)
	}
	if sizeAfter == sizeBefore {
		t.Errorf("canvas was not resized after applyPendingResize: still %v", sizeBefore)
	}
}
