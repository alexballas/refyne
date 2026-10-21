//go:build !no_glfw && !mobile && !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import (
	"os"
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/alexballas/refyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run only on a private Mutter session with its first output to the right of
// a differently sized second output, and the new window placed on the left.
func TestWaylandFullScreenFollowsCurrentOutput(t *testing.T) {
	if !runningWayland() || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_MONITOR_TEST") != "1" {
		t.Skip("requires an opted-in isolated two-monitor Mutter session")
	}

	var right, left *glfw.Monitor
	var rightX, leftX, leftY int
	var rightMode, leftMode *glfw.VidMode
	runOnMain(func() {
		right = glfw.GetPrimaryMonitor()
		rightX, _ = right.GetPos()
		rightMode = right.GetVideoMode()
		for _, monitor := range glfw.GetMonitors() {
			x, y := monitor.GetPos()
			if x < rightX {
				left, leftX, leftY = monitor, x, y
				leftMode = left.GetVideoMode()
				break
			}
		}
	})
	require.NotNil(t, left, "place the secondary output to the left of GLFW's first output")
	require.NotEqual(t, rightMode.Width, leftMode.Width, "use differently sized virtual outputs")

	var w *window
	runOnMain(func() {
		w = d.CreateWindow("Fullscreen output test").(*window)
		w.SetContent(widget.NewLabel("Fullscreen stays on this output"))
		w.Resize(fyne.NewSize(640, 360))
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })
	require.Eventually(t, func() bool {
		var visible bool
		runOnMain(func() { visible = w.viewport.GetAttrib(glfw.Visible) == glfw.True && !w.canvas.Dirty() })
		return visible
	}, 5*time.Second, 20*time.Millisecond)

	waitForSize := func(width, height int) {
		t.Helper()
		require.Eventually(t, func() bool {
			var matches bool
			runOnMain(func() {
				x, y := w.viewport.GetSize()
				matches = x == width && y == height && w.canvas.Size() == w.computeCanvasSize(width, height) && !w.canvas.Dirty()
			})
			return matches
		}, 5*time.Second, 20*time.Millisecond)
	}
	var width, height int
	runOnMain(func() { width, height = w.viewport.GetSize() })

	for i := 0; i < 5; i++ {
		runOnMain(func() { w.SetFullScreen(true) })
		waitForSize(leftMode.Width, leftMode.Height)
		var x, y int
		runOnMain(func() { x, y = w.viewport.GetMonitor().GetPos() })
		require.Equal(t, leftX, x)
		require.Equal(t, leftY, y)
		if i == 0 {
			// The explicit target is the same monitor. It must still replace
			// the compositor-selected policy and send an explicit request.
			runOnMain(w.RequestFullScreenSecondary)
			waitForSize(leftMode.Width, leftMode.Height)
			runOnMain(func() { w.SetFullScreen(true) })
		}
		runOnMain(func() { w.SetFullScreen(false) })
		waitForSize(width, height)
	}

	// With no mapped surface, GLFW initially needs a placeholder monitor.
	// Mapping must still let Mutter choose the left output and reconcile it.
	runOnMain(func() {
		w.Close()
		w = d.CreateWindow("Initially fullscreen output test").(*window)
		w.SetContent(widget.NewLabel("Initially fullscreen follows Mutter"))
		w.SetFullScreen(true)
		w.Show()
	})
	waitForSize(leftMode.Width, leftMode.Height)
	var x, y int
	runOnMain(func() { x, y = w.viewport.GetMonitor().GetPos() })
	require.Equal(t, leftX, x)
	require.Equal(t, leftY, y)
}

// Hide can follow a fullscreen request before the compositor configures it,
// while the CSD shadow subsurfaces are still mapped. The native teardown
// must unmap the toplevel before destroying them; a protocol trace shows
// xdg_toplevel.destroy ahead of the shadow wl_subsurface.destroy requests.
// Hiding while an exit is pending, or exiting while hidden, must also restore
// the windowed size and the canvas laid out for it.
func TestWaylandFullScreenHideTransitions(t *testing.T) {
	if !runningWayland() || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" {
		t.Skip("set REFYNE_WAYLAND_FULLSCREEN_TEST=1 on an isolated compositor")
	}

	var w *window
	runOnMain(func() {
		w = d.CreateWindow("Fullscreen hide in same turn").(*window)
		w.SetContent(widget.NewLabel("Hide during fullscreen transitions"))
		w.Resize(fyne.NewSize(640, 360))
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })

	settled := func(wantFull bool, width, height int) func() bool {
		return func() bool {
			var ok bool
			runOnMain(func() {
				m := w.viewport.GetMonitor()
				x, y := w.viewport.GetSize()
				if wantFull {
					if m == nil {
						return
					}
					mode := m.GetVideoMode()
					width, height = mode.Width, mode.Height
				}
				ok = w.FullScreen() == wantFull && (m != nil) == wantFull && w.viewport.GetAttrib(glfw.Visible) == glfw.True &&
					x == width && y == height && w.canvas.Size() == w.computeCanvasSize(x, y) && !w.canvas.Dirty()
			})
			return ok
		}
	}
	var width, height int
	require.Eventually(t, func() bool {
		var ready bool
		runOnMain(func() {
			width, height = w.viewport.GetSize()
			ready = w.viewport.GetAttrib(glfw.Visible) == glfw.True && w.canvas.decoration != nil && !w.canvas.Dirty()
		})
		return ready
	}, 5*time.Second, 20*time.Millisecond)

	for i := 0; i < 3; i++ {
		runOnMain(func() {
			w.SetFullScreen(true)
			w.Hide()
		})
		runOnMain(w.Show)
		require.Eventually(t, settled(true, 0, 0), 5*time.Second, 20*time.Millisecond)

		runOnMain(func() {
			w.SetFullScreen(false)
			w.Hide()
		})
		runOnMain(w.Show)
		require.Eventually(t, settled(false, width, height), 5*time.Second, 20*time.Millisecond)

		// No configure follows an exit while hidden. The canvas must
		// still leave its fullscreen layout before the window is shown.
		runOnMain(func() { w.SetFullScreen(true) })
		require.Eventually(t, settled(true, 0, 0), 5*time.Second, 20*time.Millisecond)
		runOnMain(w.Hide)
		runOnMain(func() { w.SetFullScreen(false) })
		runOnMain(w.Show)
		require.Eventually(t, settled(false, width, height), 5*time.Second, 20*time.Millisecond)
	}
}

// A window first shown fullscreen has no custom title bar. Leaving fullscreen,
// whether visible or hidden and shown again, must install it instead of
// GLFW's fallback bars and restore the size of an equivalent windowed window.
func TestWaylandInitialFullScreenExitDecorations(t *testing.T) {
	if !runningWayland() || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" {
		t.Skip("set REFYNE_WAYLAND_FULLSCREEN_TEST=1 on an isolated compositor")
	}

	type state struct {
		Native, Mode                [2]int
		Visible, Full, Decorated    bool
		Decoration, Dirty           bool
		Canvas, Want, ContentRegion fyne.Size
	}
	read := func(w *window) (s state) {
		runOnMain(func() {
			s.Native[0], s.Native[1] = w.viewport.GetSize()
			if m := w.viewport.GetMonitor(); m != nil {
				s.Full = true
				mode := m.GetVideoMode()
				s.Mode = [2]int{mode.Width, mode.Height}
			}
			s.Visible = w.viewport.GetAttrib(glfw.Visible) == glfw.True
			s.Decorated = w.viewport.GetAttrib(glfw.Decorated) == glfw.True
			s.Decoration, s.Dirty = w.canvas.decoration != nil, w.canvas.Dirty()
			s.Canvas, s.Want = w.canvas.Size(), w.computeCanvasSize(s.Native[0], s.Native[1])
			s.ContentRegion = w.canvas.contentSize(s.Canvas)
		})
		return s
	}
	waitFor := func(w *window, want func(state) bool) state {
		t.Helper()
		var got state
		require.Eventually(t, func() bool {
			got = read(w)
			return got.Visible && !got.Dirty && got.Canvas == got.Want && want(got)
		}, 5*time.Second, 20*time.Millisecond, "last state %+v", &got)
		return got
	}
	show := func(title string, full bool) *window {
		var w *window
		runOnMain(func() {
			w = d.CreateWindow(title).(*window)
			w.SetContent(widget.NewLabel("Initially fullscreen decorations"))
			w.Resize(fyne.NewSize(640, 360))
			w.SetFullScreen(full)
			w.Show()
		})
		t.Cleanup(func() { runOnMain(w.Close) })
		return w
	}

	reference := waitFor(show("Windowed reference", false), func(s state) bool { return !s.Full })
	if !reference.Decoration {
		t.Skip("the compositor provides server-side decorations")
	}

	exits := map[string]func(w *window){
		"visible": func(w *window) { runOnMain(func() { w.SetFullScreen(false) }) },
		"hidden": func(w *window) {
			runOnMain(w.Hide)
			runOnMain(func() { w.SetFullScreen(false) })
			runOnMain(w.Show)
		},
		// Show before any frame applies the size reported for the hidden exit.
		"hidden, shown in the same turn": func(w *window) {
			runOnMain(w.Hide)
			runOnMain(func() {
				w.SetFullScreen(false)
				w.Show()
			})
		},
	}
	for _, name := range []string{"visible", "hidden", "hidden, shown in the same turn"} {
		w := show("Initially fullscreen decorations", true)
		full := waitFor(w, func(s state) bool { return s.Full && s.Native == s.Mode })
		assert.False(t, full.Decoration, "the title bar is installed only after fullscreen")

		exits[name](w)
		got := waitFor(w, func(s state) bool { return !s.Full && s.Native == reference.Native })
		assert.True(t, got.Decoration, "%s: custom title bar installed", name)
		assert.False(t, got.Decorated, "%s: GLFW fallback bars disabled", name)
		assert.Equal(t, reference.ContentRegion, got.ContentRegion, "%s: content area", name)
		runOnMain(w.Close)
	}
}

// Fullscreen requests queued within one turn must leave the window in the
// last requested state. Mutter answers exit/enter/exit with a stale windowed
// configure, then a fullscreen one, then a windowed one suggesting the
// fullscreen size, because it saved its restore rectangle before this client
// committed the first exit. The size requested on exit must win.
func TestWaylandFullScreenQueuedToggles(t *testing.T) {
	if !runningWayland() || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" {
		t.Skip("set REFYNE_WAYLAND_FULLSCREEN_TEST=1 on an isolated compositor")
	}

	var w *window
	runOnMain(func() {
		w = d.CreateWindow("Queued fullscreen toggles").(*window)
		w.SetContent(widget.NewLabel("Queued fullscreen toggles"))
		w.Resize(fyne.NewSize(640, 360))
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })

	type state struct {
		Native, Restore, Mode [2]int
		Full, FyneFull, Dirty bool
		Canvas, Want          fyne.Size
	}
	read := func() (s state) {
		runOnMain(func() {
			s.Native[0], s.Native[1] = w.viewport.GetSize()
			s.Restore = [2]int{w.width, w.height}
			if m := w.viewport.GetMonitor(); m != nil {
				s.Full = true
				mode := m.GetVideoMode()
				s.Mode = [2]int{mode.Width, mode.Height}
			}
			s.FyneFull, s.Dirty = w.FullScreen(), w.canvas.Dirty()
			s.Canvas, s.Want = w.canvas.Size(), w.computeCanvasSize(s.Native[0], s.Native[1])
		})
		return s
	}
	var windowed [2]int
	waitFor := func(full bool) {
		t.Helper()
		var got state
		require.Eventually(t, func() bool {
			got = read()
			ok := got.Full == full && got.FyneFull == full && !got.Dirty && got.Canvas == got.Want
			if full {
				return ok && got.Native == got.Mode
			}
			return ok && got.Native == windowed && got.Restore == windowed
		}, 5*time.Second, 20*time.Millisecond, "wanted fullscreen=%v windowed=%v; got %+v", full, windowed, &got)
	}
	require.Eventually(t, func() bool {
		s := read()
		windowed = s.Native
		return w.canvas.decoration != nil && !s.Dirty && s.Canvas == s.Want && s.Restore == s.Native
	}, 5*time.Second, 20*time.Millisecond)

	for i := 0; i < 3; i++ {
		runOnMain(func() { w.SetFullScreen(true) })
		waitFor(true)
		runOnMain(func() {
			w.SetFullScreen(false)
			w.SetFullScreen(true)
			w.SetFullScreen(false)
		})
		waitFor(false)
		runOnMain(func() {
			w.SetFullScreen(true)
			w.SetFullScreen(false)
			w.SetFullScreen(true)
		})
		waitFor(true)
		runOnMain(func() { w.SetFullScreen(false) })
		waitFor(false)
	}
}
