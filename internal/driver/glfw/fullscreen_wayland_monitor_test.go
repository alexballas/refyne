//go:build !no_glfw && !mobile && !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import (
	"image/color"
	"os"
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/alexballas/refyne/v2/widget"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

type mutterOutputSpec struct{ Connector, Vendor, Product, Serial string }

type mutterOutputMode struct {
	ID             string
	Width, Height  int32
	Refresh, Scale float64
	Scales         []float64
	Properties     map[string]dbus.Variant
}

type mutterOutput struct {
	Spec       mutterOutputSpec
	Modes      []mutterOutputMode
	Properties map[string]dbus.Variant
}

type mutterLogicalOutput struct {
	X, Y       int32
	Scale      float64
	Transform  uint32
	Primary    bool
	Outputs    []mutterOutputSpec
	Properties map[string]dbus.Variant
}

type mutterOutputConfig struct {
	Connector, Mode string
	Properties      map[string]dbus.Variant
}

type mutterLogicalConfig struct {
	X, Y      int32
	Scale     float64
	Transform uint32
	Primary   bool
	Outputs   []mutterOutputConfig
}

// These helpers change the display configuration. Never opt in on a real
// desktop: use a private D-Bus session and a disposable headless Mutter.
func fullscreenMutterConfig(t *testing.T) (dbus.BusObject, []mutterLogicalConfig) {
	t.Helper()
	if !runningWayland() || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_TEST") != "1" || os.Getenv("REFYNE_WAYLAND_FULLSCREEN_MONITOR_TEST") != "1" {
		t.Skip("requires an opted-in isolated two-monitor Mutter session")
	}
	bus, err := dbus.ConnectSessionBus()
	require.NoError(t, err)
	t.Cleanup(func() { _ = bus.Close() })
	obj := bus.Object("org.gnome.Mutter.DisplayConfig", "/org/gnome/Mutter/DisplayConfig")
	var serial uint32
	var outputs []mutterOutput
	var logicals []mutterLogicalOutput
	var props map[string]dbus.Variant
	require.NoError(t, obj.Call("org.gnome.Mutter.DisplayConfig.GetCurrentState", 0).Store(&serial, &outputs, &logicals, &props))
	require.Len(t, logicals, 2)
	var configs []mutterLogicalConfig
	for _, logical := range logicals {
		config := mutterLogicalConfig{X: logical.X, Y: logical.Y, Scale: logical.Scale, Transform: logical.Transform, Primary: logical.Primary}
		for _, spec := range logical.Outputs {
			for _, output := range outputs {
				if output.Spec.Connector != spec.Connector {
					continue
				}
				for _, mode := range output.Modes {
					if current, ok := mode.Properties["is-current"].Value().(bool); ok && current {
						config.Outputs = append(config.Outputs, mutterOutputConfig{Connector: spec.Connector, Mode: mode.ID, Properties: map[string]dbus.Variant{}})
					}
				}
			}
		}
		require.NotEmpty(t, config.Outputs)
		configs = append(configs, config)
	}
	var firstX, firstY int
	runOnMain(func() { firstX, firstY = glfw.GetPrimaryMonitor().GetPos() })
	t.Cleanup(func() {
		applyFullscreenMutterConfig(t, obj, configs)
		waitFullscreenMonitorCount(t, 2)
		var x, y int
		runOnMain(func() { x, y = glfw.GetPrimaryMonitor().GetPos() })
		if x == firstX && y == firstY {
			return
		}
		// Wayland's "primary" is the first registered output. If that one
		// was removed, restore its registration order for subsequent tests.
		for _, config := range configs {
			if config.X == int32(firstX) && config.Y == int32(firstY) {
				config.X, config.Y, config.Primary = 0, 0, true
				applyFullscreenMutterConfig(t, obj, []mutterLogicalConfig{config})
				waitFullscreenMonitorCount(t, 1)
				break
			}
		}
		applyFullscreenMutterConfig(t, obj, configs)
		waitFullscreenMonitorCount(t, 2)
	})
	return obj, configs
}

func waitFullscreenMonitorCount(t *testing.T, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var count int
		runOnMain(func() { count = len(glfw.GetMonitors()) })
		return count == want
	}, 5*time.Second, 20*time.Millisecond)
}

func applyFullscreenMutterConfig(t *testing.T, obj dbus.BusObject, configs []mutterLogicalConfig) {
	t.Helper()
	state := obj.Call("org.gnome.Mutter.DisplayConfig.GetCurrentState", 0)
	require.NoError(t, state.Err)
	serial, ok := state.Body[0].(uint32)
	require.True(t, ok)
	// Method 1 is temporary, so no persistent desktop settings are changed.
	require.NoError(t, obj.Call("org.gnome.Mutter.DisplayConfig.ApplyMonitorsConfig", 0, serial, uint32(1), configs, map[string]dbus.Variant{}).Err)
}

func waitFullscreenWindow(t *testing.T, w *window, width, height int, full bool) {
	t.Helper()
	var got struct {
		Width, Height, RestoreWidth, RestoreHeight int
		FullScreen, NativeFullScreen, Dirty        bool
		Canvas                                     fyne.Size
	}
	require.Eventually(t, func() bool {
		var matches bool
		runOnMain(func() {
			x, y := w.viewport.GetSize()
			got.Width, got.Height = x, y
			got.RestoreWidth, got.RestoreHeight = w.width, w.height
			got.FullScreen, got.NativeFullScreen, got.Dirty = w.FullScreen(), w.viewport.GetMonitor() != nil, w.canvas.Dirty()
			got.Canvas = w.canvas.Size()
			matches = w.FullScreen() == full && (w.viewport.GetMonitor() != nil) == full && x == width && y == height && w.canvas.Size() == w.computeCanvasSize(width, height) && !w.canvas.Dirty()
		})
		return matches
	}, 5*time.Second, 20*time.Millisecond, "wanted %dx%d fullscreen=%v; got %+v", width, height, full, &got)
}

func TestWaylandFullScreenOutputDisconnect(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "compositor-selected"
		if explicit {
			name = "explicit"
		}
		t.Run(name, func(t *testing.T) {
			obj, configs := fullscreenMutterConfig(t)
			var w *window
			var targetWidth, targetHeight, survivorWidth, survivorHeight int
			runOnMain(func() {
				for _, m := range glfw.GetMonitors() {
					x, _ := m.GetPos()
					mode := m.GetVideoMode()
					if x == 0 {
						targetWidth, targetHeight = mode.Width, mode.Height
					} else {
						survivorWidth, survivorHeight = mode.Width, mode.Height
					}
				}
				w = d.CreateWindow("Fullscreen disconnect test").(*window)
				w.SetContent(widget.NewLabel("Disconnect this output"))
				w.Resize(fyne.NewSize(640, 360))
				w.Show()
			})
			t.Cleanup(func() { runOnMain(w.Close) })
			var width, height int
			runOnMain(func() { width, height = w.viewport.GetSize() })
			if explicit {
				runOnMain(w.RequestFullScreenSecondary)
			} else {
				runOnMain(func() { w.SetFullScreen(true) })
			}
			waitFullscreenWindow(t, w, targetWidth, targetHeight, true)
			var surviving []mutterLogicalConfig
			for _, config := range configs {
				if config.X != 0 {
					config.X, config.Y, config.Primary = 0, 0, true
					surviving = append(surviving, config)
				}
			}
			require.Len(t, surviving, 1)
			applyFullscreenMutterConfig(t, obj, surviving)
			if explicit {
				waitFullscreenWindow(t, w, width, height, false)
			} else {
				waitFullscreenWindow(t, w, survivorWidth, survivorHeight, true)
				runOnMain(func() { w.SetFullScreen(false) })
				waitFullscreenWindow(t, w, width, height, false)
			}
		})
	}
}

func TestWaylandFullScreenPlaceholderDisconnect(t *testing.T) {
	obj, configs := fullscreenMutterConfig(t)
	var w *window
	var leftWidth, leftHeight int
	runOnMain(func() {
		left := (&window{}).getSecondaryMonitor().GetVideoMode()
		leftWidth, leftHeight = left.Width, left.Height
		w = d.CreateWindow("Hidden fullscreen disconnect test").(*window)
		w.SetContent(widget.NewLabel("Initially fullscreen"))
		w.Resize(fyne.NewSize(640, 360))
		// Allocate the native window without mapping it. There are no entered
		// outputs yet, so automatic fullscreen uses the first as a placeholder.
		w.created = true
		w.prepareFramebufferPresentation()
		w.create()
		w.SetFullScreen(true)
	})
	t.Cleanup(func() { runOnMain(w.Close) })
	var placeholderX int
	runOnMain(func() { placeholderX, _ = w.viewport.GetMonitor().GetPos() })
	require.NotZero(t, placeholderX)
	var surviving []mutterLogicalConfig
	for _, config := range configs {
		if config.X == 0 {
			config.Primary = true
			surviving = append(surviving, config)
		}
	}
	require.Len(t, surviving, 1)
	applyFullscreenMutterConfig(t, obj, surviving)
	require.Eventually(t, func() bool {
		var survived bool
		runOnMain(func() {
			survived = len(glfw.GetMonitors()) == 1 && w.FullScreen() && w.viewport.GetMonitor() != nil
		})
		return survived
	}, 5*time.Second, 20*time.Millisecond)
	runOnMain(w.Show)
	waitFullscreenWindow(t, w, leftWidth, leftHeight, true)
}

func TestWaylandFullScreenPolicyTransitions(t *testing.T) {
	fullscreenMutterConfig(t)
	var w *window
	var rightWidth, rightHeight, leftWidth, leftHeight int
	runOnMain(func() {
		right := glfw.GetPrimaryMonitor().GetVideoMode()
		left := (&window{}).getSecondaryMonitor().GetVideoMode()
		rightWidth, rightHeight, leftWidth, leftHeight = right.Width, right.Height, left.Width, left.Height
		w = d.CreateWindow("Fullscreen policy transitions").(*window)
		w.SetContent(widget.NewLabel("Fullscreen policy transitions"))
		w.Resize(fyne.NewSize(640, 360))
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })
	var width, height int
	runOnMain(func() { width, height = w.viewport.GetSize() })
	runOnMain(func() { w.SetFullScreen(true) })
	waitFullscreenWindow(t, w, leftWidth, leftHeight, true)
	// Move explicitly to the first output, then adopt compositor-selected
	// fullscreen there. Secondary fullscreen must now move to a different one.
	runOnMain(func() { w.doApplyFullScreen(glfw.GetPrimaryMonitor(), true) })
	waitFullscreenWindow(t, w, rightWidth, rightHeight, true)
	runOnMain(func() { w.SetFullScreen(true) })
	waitFullscreenWindow(t, w, rightWidth, rightHeight, true)
	runOnMain(w.RequestFullScreenSecondary)
	waitFullscreenWindow(t, w, leftWidth, leftHeight, true)
	runOnMain(func() { w.SetFullScreen(true) })
	waitFullscreenWindow(t, w, leftWidth, leftHeight, true)

	// A scale callback while fullscreen must not overwrite the saved windowed
	// bounds. Exercise it without relying on a particular monitor DPI.
	runOnMain(w.RescaleContext)
	for i := 0; i < 3; i++ {
		runOnMain(w.Hide)
		var visible bool
		runOnMain(func() { visible = w.viewport.GetAttrib(glfw.Visible) == glfw.True })
		require.False(t, visible, "Hide must unmap the native fullscreen surface")
		runOnMain(w.Show)
		// Mutter may choose either output when mapping a new shell role.
		require.Eventually(t, func() bool {
			var settled bool
			runOnMain(func() {
				m := w.viewport.GetMonitor()
				if m == nil {
					return
				}
				mode := m.GetVideoMode()
				x, y := w.viewport.GetSize()
				settled = w.FullScreen() && w.viewport.GetAttrib(glfw.Visible) == glfw.True && x == mode.Width && y == mode.Height && w.canvas.Size() == w.computeCanvasSize(x, y) && !w.canvas.Dirty()
			})
			return settled
		}, 5*time.Second, 20*time.Millisecond)
	}
	runOnMain(func() { w.SetFullScreen(false) })
	waitFullscreenWindow(t, w, width, height, false)
}

func TestWaylandFullScreenSpanningOutputs(t *testing.T) {
	fullscreenMutterConfig(t)
	var w *window
	runOnMain(func() {
		w = d.CreateWindow("Fullscreen spanning outputs").(*window)
		r := canvas.NewRectangle(color.White)
		// This cannot fit on either virtual output. Mutter's initial left
		// placement makes the mapped content extend across their boundary.
		r.SetMinSize(fyne.NewSize(1500, 300))
		w.SetPadded(false)
		w.SetContent(r)
		w.Resize(r.MinSize())
		w.Show()
	})
	t.Cleanup(func() { runOnMain(w.Close) })
	var stableSince time.Time
	require.Eventually(t, func() bool {
		var ready bool
		runOnMain(func() {
			width, _ := w.viewport.GetSize()
			ready = width >= 1500 && !w.canvas.Dirty()
		})
		if !ready {
			stableSince = time.Time{}
			return false
		}
		if stableSince.IsZero() {
			stableSince = time.Now()
		}
		// Let the spanning buffer be presented on both outputs before toggling.
		return time.Since(stableSince) >= 100*time.Millisecond
	}, 5*time.Second, 20*time.Millisecond)
	// The protocol trace should show two wl_surface.enter events followed
	// by a leave when fullscreen selects one output. Its bookkeeping must
	// converge to the chosen output despite the initial ambiguity.
	runOnMain(func() { w.SetFullScreen(true) })
	require.Eventually(t, func() bool {
		var settled bool
		runOnMain(func() {
			if m := w.viewport.GetMonitor(); m != nil {
				mode := m.GetVideoMode()
				x, y := w.viewport.GetSize()
				settled = x == mode.Width && y == mode.Height && w.canvas.Size() == w.computeCanvasSize(x, y) && !w.canvas.Dirty()
			}
		})
		return settled
	}, 5*time.Second, 20*time.Millisecond)
}
