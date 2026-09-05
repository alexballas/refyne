//go:build !no_glfw && !mobile && !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import (
	"image/color"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/stretchr/testify/assert"
)

func TestWaylandPremapPaintPreservesFramebufferSettling(t *testing.T) {
	if !runningWayland() {
		t.Skip("requires Wayland")
	}

	for _, pending := range []bool{false, true} {
		w := createWindow("Premap settling")
		w.SetContent(canvas.NewRectangle(color.Black))
		runOnMain(func() {
			win := w.window
			win.prepareFramebufferPresentation()
			win.framebufferResizePending = pending
			win.visible = true // Show sets this before native mapping completes.
			assert.Equal(t, glfw.False, win.viewport.GetAttrib(glfw.Visible))
			win.RunWithContext(func() { d.repaintWindow(win) })
			assert.True(t, win.framebufferSettleActive)
			assert.Equal(t, pending, win.framebufferResizePending)
			assert.True(t, win.frame.ready())

			// A configure arriving after the skipped swap must still settle.
			win.processFrameSized(win.framebufferWidth+1, win.framebufferHeight+1)
			assert.True(t, win.framebufferResizePending)
			win.visible = false
		})
		destroyTestWindow(w)
	}
}

func TestWaylandInitialDecorationsPreserveConfiguredBounds(t *testing.T) {
	if !runningWayland() {
		t.Skip("requires Wayland")
	}

	for _, configured := range []bool{false, true} {
		w := createWindow("Initial decorations")
		w.SetPadded(false)
		w.SetContent(canvas.NewRectangle(color.Black))
		runOnMain(func() {
			win := w.window
			base := fyne.NewSize(900, 650)
			win.canvas.Resize(base)
			win.width, win.height = win.screenSize(base)
			win.shouldWidth, win.shouldHeight = win.width, win.height
			win.shouldExpand = true // queued before the initial configure
			win.pendingResize = false
			win.visible = true
			want := base.AddWidthHeight(0, titleBarHeight)
			if configured {
				want = fyne.NewSize(640, 720)
			}
			width, height := win.screenSize(want)
			win.viewport.SetSize(width, height)
			if configured {
				win.resized(nil, width, height)
			}

			// Exercise the actual Show ordering, including the chrome resize
			// and any expansion left for the following event-loop frame.
			win.setupWaylandDecorations()
			win.repaintAfterShow()
			d.processWindowEvents()
			win.applyPendingResize()

			gotWidth, gotHeight := win.viewport.GetSize()
			assert.Equal(t, width, gotWidth)
			assert.Equal(t, height, gotHeight)
			assert.Equal(t, want, win.canvas.Size())
			assert.Equal(t, want.Subtract(fyne.NewSize(0, titleBarHeight)), win.canvas.Content().Size())
			assert.False(t, win.pendingResize)
			win.visible = false
		})
		destroyTestWindow(w)
	}
}

func TestWaylandPresentationCompletesOnShowAndRemap(t *testing.T) {
	if !runningWayland() {
		t.Skip("requires Wayland")
	}

	var w *safeWindow
	runOnMain(func() {
		// Use the public lifecycle so the first Show also creates the viewport.
		w = &safeWindow{window: d.CreateWindow("Presentation lifecycle").(*window)}
	})
	defer destroyTestWindow(w)
	w.SetContent(canvas.NewRectangle(color.Black))
	w.Resize(fyne.NewSize(300, 200))

	for mapping := 0; mapping < 2; mapping++ {
		runOnMain(func() {
			win := w.window
			win.Show()
			assert.Equal(t, glfw.True, win.viewport.GetAttrib(glfw.Visible))
			assert.False(t, win.framebufferSettleActive)
			assert.False(t, win.framebufferResizePending)

			// A live resize after presentation must stay on the single-swap path.
			win.Resize(win.canvas.Size().AddWidthHeight(20, 20))
			assert.False(t, win.framebufferResizePending)
			d.runFrame()
			assert.False(t, win.framebufferSettleActive)
			win.Hide()
			assert.Equal(t, glfw.False, win.viewport.GetAttrib(glfw.Visible))
		})
	}
}
