//go:build !no_glfw && !mobile && (linux || freebsd || openbsd || netbsd)

package glfw

import (
	"math"
	"testing"
	"unsafe"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/widget"
)

type resizeReadyGate struct {
	presentable bool
}

func (g *resizeReadyGate) ready() bool        { return g.presentable }
func (g *resizeReadyGate) arm(unsafe.Pointer) { g.presentable = false }
func (g *resizeReadyGate) markReady()         { g.presentable = true }
func (g *resizeReadyGate) free()              {}

// TestResizeCoalescing verifies that a burst of configure events delivered to the
// resized callback does NOT resize synchronously (it is coalesced), and that a
// single applyPendingResize then applies the latest size - so N configure events
// cost one canvas.Resize per frame instead of N.
func TestResizeCoalescing(t *testing.T) {
	w := createWindow("Test")
	defer destroyTestWindow(w)
	w.SetPadded(false)
	w.SetContent(widget.NewLabel("content"))
	base := fyne.NewSize(1000, 800)

	var sizeBefore, sizeMid, sizeAfter, expected fyne.Size
	var pendingAfterBurst bool

	// One atomic main-thread block so the live draw loop cannot interleave and
	// apply the pending resize before we observe it.
	runOnMain(func() {
		win := w.window
		win.canvas.Resize(base)
		win.visible = true
		win.fixedSize = false
		win.fullScreen = false
		win.pendingResize = false // clear any stale state from setup
		win.shouldExpand = false

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

func TestResizeCoalescingSchedulesFrameWhenGateNotReady(t *testing.T) {
	canvas := newCanvas()
	canvas.CheckDirtyAndClear()

	gate := &resizeReadyGate{}
	w := &window{canvas: canvas, frame: gate}

	w.resized(nil, 640, 480)

	if !w.pendingResize {
		t.Fatal("resize callback did not leave a pending coalesced resize")
	}
	if !gate.ready() {
		t.Fatal("resize callback did not mark the present gate ready")
	}
	if !canvas.Dirty() {
		t.Fatal("resize callback did not schedule a repaint")
	}
}

func TestPendingConfigureWinsOverQueuedExpansion(t *testing.T) {
	w := createWindow("Test")
	defer destroyTestWindow(w)
	w.SetPadded(false)
	w.SetContent(widget.NewLabel("content"))

	const iterations = 100
	for iteration := 0; iteration < iterations; iteration++ {
		base := fyne.NewSize(float32(900+iteration%5*25), float32(650+iteration%7*20))
		configuredWidth := 640 + iteration%11*17
		configuredHeight := 720 + iteration%13*19

		var got fyne.Size
		var width, height int
		var pending, expand bool
		runOnMain(func() {
			win := w.window
			win.canvas.Resize(base)
			win.visible = true
			win.fixedSize = false
			win.fullScreen = false
			win.pendingResize = false
			win.shouldExpand = false
			win.width, win.height = int(base.Width), int(base.Height)
			win.shouldWidth, win.shouldHeight = win.width, win.height

			queueExpansion := func() {
				win.shouldWidth, win.shouldHeight = int(base.Width), int(base.Height)
				win.shouldExpand = true
			}
			configure := func(width, height int) {
				win.resized(nil, width, height)
			}

			// Exercise the realistic orderings between a stale client expansion,
			// compositor configures, event processing, and frame processing.
			switch iteration % 5 {
			case 0: // expansion, configure, events
				queueExpansion()
				configure(configuredWidth, configuredHeight)
				d.processWindowEvents()
			case 1: // expansion handled before configure
				queueExpansion()
				d.processWindowEvents()
				configure(configuredWidth, configuredHeight)
			case 2: // configure queued before expansion
				configure(configuredWidth, configuredHeight)
				queueExpansion()
				d.processWindowEvents()
			case 3: // configure applied before a late stale expansion
				configure(configuredWidth, configuredHeight)
				win.applyPendingResize()
				queueExpansion()
				d.processWindowEvents()
			case 4: // configure burst around event processing
				queueExpansion()
				configure(configuredWidth-23, configuredHeight-31)
				d.processWindowEvents()
				configure(configuredWidth, configuredHeight)
			}

			// Settle the same work a complete event-loop frame would settle.
			d.processWindowEvents()
			win.applyPendingResize()

			got = win.canvas.Size()
			width, height = win.width, win.height
			pending = win.pendingResize
			expand = win.shouldExpand
		})

		want := w.window.computeCanvasSize(configuredWidth, configuredHeight)
		want = fyne.NewSize(
			float32(math.Ceil(float64(want.Width))),
			float32(math.Ceil(float64(want.Height))),
		)
		if got != want || width != configuredWidth || height != configuredHeight {
			t.Fatalf("iteration %d: size=%v framebuffer=%dx%d, want %v/%dx%d", iteration, got, width, height, want, configuredWidth, configuredHeight)
		}
		if pending {
			t.Fatalf("iteration %d: compositor configure remained pending", iteration)
		}
		if expand {
			t.Fatalf("iteration %d: superseded client expansion remained queued", iteration)
		}
	}
}

func TestRescaleContextHonorsPendingConfigure(t *testing.T) {
	w := createWindow("Test")
	defer destroyTestWindow(w)
	w.SetPadded(false)
	w.SetContent(widget.NewLabel("content"))

	const iterations = 100
	for iteration := 0; iteration < iterations; iteration++ {
		base := fyne.NewSize(float32(900+iteration%5*25), float32(650+iteration%7*20))
		configuredWidth := 600 + iteration%11*17
		configuredHeight := 700 + iteration%13*19

		var got fyne.Size
		var width, height, viewportWidth, viewportHeight int
		runOnMain(func() {
			win := w.window
			win.canvas.Resize(base)
			win.viewport.SetSize(int(base.Width), int(base.Height))
			win.visible = true
			win.fixedSize = false
			win.fullScreen = false
			win.pendingResize = false
			win.shouldExpand = false
			win.width, win.height = int(base.Width), int(base.Height)
			win.shouldWidth, win.shouldHeight = win.width, win.height

			// A Wayland output/scale event can rescale the context in the same
			// dispatch batch as the compositor configure.
			win.resized(nil, configuredWidth, configuredHeight)
			win.RescaleContext()
			viewportWidth, viewportHeight = win.viewport.GetSize()
			win.applyPendingResize()

			got = win.canvas.Size()
			width, height = win.width, win.height
		})

		want := w.window.computeCanvasSize(configuredWidth, configuredHeight)
		want = fyne.NewSize(
			float32(math.Ceil(float64(want.Width))),
			float32(math.Ceil(float64(want.Height))),
		)
		if got != want || width != configuredWidth || height != configuredHeight || viewportWidth != configuredWidth || viewportHeight != configuredHeight {
			t.Fatalf("iteration %d: size=%v framebuffer=%dx%d viewport=%dx%d, want %v/%dx%d", iteration, got, width, height, viewportWidth, viewportHeight, want, configuredWidth, configuredHeight)
		}
	}
}

func TestRepaintAfterShowAppliesPendingConfigure(t *testing.T) {
	w := createWindow("Test")
	defer destroyTestWindow(w)
	w.SetPadded(false)
	w.SetContent(widget.NewLabel("content"))

	var before, after, expected fyne.Size
	var pendingAfter bool
	runOnMain(func() {
		win := w.window
		win.visible = true
		win.pendingResize = false
		before = win.canvas.Size()

		win.resized(nil, 640, 480)
		canvasSize := win.computeCanvasSize(640, 480)
		expected = fyne.NewSize(
			float32(math.Ceil(float64(canvasSize.Width))),
			float32(math.Ceil(float64(canvasSize.Height))),
		)

		win.repaintAfterShow()
		after = win.canvas.Size()
		pendingAfter = win.pendingResize
	})

	if before == expected {
		t.Fatalf("test setup did not change configured size: %v", before)
	}
	if pendingAfter {
		t.Fatal("show repaint left compositor configure pending")
	}
	if after != expected {
		t.Fatalf("show repainted before applying compositor configure: got=%v want=%v", after, expected)
	}
}
