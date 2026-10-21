//go:build !darwin && !wasm && !test_web_driver && !no_glfw && !mobile

package glfw

import (
	"image/color"
	"os"
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSecondaryMonitorUsesPosition(t *testing.T) {
	var found bool
	var wantX, wantY, gotX, gotY int
	runOnMain(func() {
		primary := glfw.GetPrimaryMonitor()
		primaryX, primaryY := primary.GetPos()
		for _, candidate := range glfw.GetMonitors() {
			x, y := candidate.GetPos()
			if x == primaryX && y == primaryY {
				continue
			}

			found = true
			wantX, wantY = x, y
			break
		}

		if found {
			gotX, gotY = (&window{}).getSecondaryMonitor().GetPos()
		}
	})

	if !found {
		t.Skip("requires a non-mirrored secondary monitor")
	}
	assert.Equal(t, wantX, gotX)
	assert.Equal(t, wantY, gotY)
}

func TestApplyFullScreenWithoutMonitor(t *testing.T) {
	assert.NotPanics(t, func() {
		(&window{}).doApplyFullScreen(nil, true)
	})
}

func TestWaylandFullScreenRoundTrip(t *testing.T) {
	if !runningWayland() {
		t.Skip("requires Wayland")
	}
	if os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" {
		t.Skip("set REFYNE_WAYLAND_FULLSCREEN_TEST=1 on an isolated compositor")
	}

	var w *window
	var content *canvas.Rectangle
	runOnMain(func() {
		w = d.CreateWindow("Fullscreen round trip").(*window)
		content = canvas.NewRectangle(color.NRGBA{R: 255, A: 255})
		content.SetMinSize(fyne.NewSize(100, 80))
		w.SetContent(container.NewCenter(content))
		w.Resize(fyne.NewSize(640, 360))
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })

	require.Eventually(t, func() bool {
		var visible bool
		runOnMain(func() { visible = w.viewport.GetAttrib(glfw.Visible) == glfw.True })
		return visible
	}, 5*time.Second, 20*time.Millisecond)

	var width, height int
	var windowedCanvasSize fyne.Size
	var windowedContentPosition fyne.Position
	runOnMain(func() {
		width, height = w.viewport.GetSize()
		windowedCanvasSize = w.canvas.Size()
		windowedContentPosition = content.Position()
	})
	waitForSize := func(wantWidth, wantHeight int) {
		t.Helper()
		require.Eventually(t, func() bool {
			var restored bool
			runOnMain(func() {
				gotWidth, gotHeight := w.viewport.GetSize()
				restored = gotWidth == wantWidth && gotHeight == wantHeight && w.canvas.Size() == windowedCanvasSize && !w.canvas.Dirty()
			})
			return restored
		}, 5*time.Second, 20*time.Millisecond)
	}

	// Repeated transitions exercise hiding and restoring the CSD shadow
	// subsurfaces, including the first frame after entering fullscreen.
	for i := 0; i < 5; i++ {
		runOnMain(func() { w.SetFullScreen(true) })
		require.Eventually(t, func() bool {
			var resized bool
			runOnMain(func() { resized = w.canvas.Size() != windowedCanvasSize })
			return resized
		}, 5*time.Second, 20*time.Millisecond)
		var heldUntilConfigure bool
		runOnMain(func() {
			beforeWidth, beforeHeight := w.viewport.GetSize()
			w.SetFullScreen(false)
			gotWidth, gotHeight := w.viewport.GetSize()
			heldUntilConfigure = gotWidth == beforeWidth && gotHeight == beforeHeight
		})
		require.True(t, heldUntilConfigure, "native size changed before the unfullscreen configure")
		waitForSize(width, height)
		var restoredContentPosition fyne.Position
		runOnMain(func() { restoredContentPosition = content.Position() })
		require.Equal(t, windowedContentPosition, restoredContentPosition, "content remained outside the restored viewport")
	}
}

// Fixed-size windows used to recurse between fitContent and processResized
// once visible. Their size, content minimum and fullscreen state must still
// reach both the native window and the canvas.
func TestWindow_FixedSizeShowResizeAndFullScreen(t *testing.T) {
	var w *window
	content := canvas.NewRectangle(color.NRGBA{G: 255, A: 255})
	content.SetMinSize(fyne.NewSize(100, 80))
	runOnMain(func() {
		w = d.CreateWindow("Fixed size").(*window)
		w.SetPadded(false)
		w.SetContent(content)
		w.SetFixedSize(true)
		w.Resize(fyne.NewSize(400, 300))
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })

	type state struct {
		Native, Restore, Mode [2]int
		Canvas, Content       fyne.Size
		Visible, Full         bool
		Dirty                 bool
	}
	read := func() (s state) {
		runOnMain(func() {
			s.Native[0], s.Native[1] = w.viewport.GetSize()
			s.Restore = [2]int{w.width, w.height}
			s.Canvas = w.canvas.Size()
			s.Content = content.Size()
			s.Visible = w.viewport.GetAttrib(glfw.Visible) == glfw.True
			if m := w.viewport.GetMonitor(); m != nil {
				s.Full = true
				mode := m.GetVideoMode()
				s.Mode = [2]int{mode.Width, mode.Height}
			}
			s.Dirty = w.canvas.Dirty()
		})
		return s
	}
	waitFor := func(want func(state) bool) state {
		t.Helper()
		var got state
		require.Eventually(t, func() bool {
			got = read()
			return got.Visible && !got.Dirty && got.Canvas == w.computeCanvasSize(got.Native[0], got.Native[1]) && want(got)
		}, 5*time.Second, 20*time.Millisecond, "last state %+v", &got)
		return got
	}

	waitFor(func(s state) bool { return s.Content == fyne.NewSize(400, 300) && !s.Full })

	// Resize sets the canvas size, which includes any window chrome.
	runOnMain(func() { w.Resize(fyne.NewSize(500, 350)) })
	windowed := waitFor(func(s state) bool { return s.Canvas == fyne.NewSize(500, 350) && !s.Full })

	// Content with a larger minimum grows the fixed size.
	content = canvas.NewRectangle(color.NRGBA{B: 255, A: 255})
	content.SetMinSize(fyne.NewSize(600, 400))
	runOnMain(func() { w.SetContent(content) })
	windowed = waitFor(func(s state) bool { return s.Content == fyne.NewSize(600, 400) && !s.Full })
	assert.Equal(t, windowed.Native, windowed.Restore)

	if runningWayland() && os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" {
		return
	}
	runOnMain(func() { w.SetFullScreen(true) })
	waitFor(func(s state) bool { return s.Full && s.Native == s.Mode })
	runOnMain(func() { w.SetFullScreen(false) })
	waitFor(func(s state) bool {
		return !s.Full && s.Native == windowed.Native && s.Content == fyne.NewSize(600, 400)
	})
}
