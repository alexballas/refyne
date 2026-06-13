//go:build !wasm && !test_web_driver

package glfw

import (
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/glfw"
)

// frameInterval returns the tick period that drives the event/draw loop. It is
// derived from the fastest connected monitor so drawing, animation and input
// keep pace with high-refresh-rate displays (120/144/240 Hz) instead of the
// historical fixed 60 Hz. On Wayland the per-surface frame callbacks (see the
// presentGate) still throttle each window to the refresh rate of the monitor it
// is actually on, so taking the maximum here is safe for mixed-refresh,
// multi-monitor setups. Must be called on the main thread (after glfw.Init).
func (d *gLDriver) frameInterval() time.Duration {
	return time.Second / time.Duration(d.displayRefreshRate())
}

// displayRefreshRate reports the highest refresh rate (Hz) among the connected
// monitors, clamped to a sane range. It never reports below 60 so existing
// 60 Hz setups keep their previous cadence exactly.
func (d *gLDriver) displayRefreshRate() int {
	rate := 0
	for _, m := range glfw.GetMonitors() {
		if m == nil {
			continue
		}
		if mode := m.GetVideoMode(); mode != nil && mode.RefreshRate > rate {
			rate = mode.RefreshRate
		}
	}
	if rate < 60 {
		rate = 60 // never go below the historical default (also covers 0/unknown)
	}
	if rate > 1000 {
		rate = 1000 // guard against bogus values from a misbehaving backend
	}
	return rate
}

func (d *gLDriver) initGLFW() {
	// TEMPORARY: disable libdecor for Wayland window decorations (GLFW 3.4).
	// Must be called before glfw.Init(); ignored on non-Wayland platforms.
	glfw.InitHint(glfw.WaylandLibdecor, glfw.WaylandDisableLibdecor)

	err := glfw.Init()
	if err != nil {
		fyne.LogError("failed to initialise GLFW", err)
		return
	}

	// Record the backend GLFW actually selected. In the default build both X11
	// and Wayland are compiled in, so this is the only reliable signal of which
	// one is live. Must run before initCursors (it sets up Wayland-only cursors).
	waylandRuntime = glfw.GetPlatform() == glfw.PlatformWayland

	initCursors()
}

func (d *gLDriver) waitEvents() {
	glfw.WaitEvents()
}

func (d *gLDriver) waitEventsTimeout(timeout time.Duration) {
	glfw.WaitEventsTimeout(timeout.Seconds())
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}

func wakeEventLoop() {
	if eventLoopReady.Load() {
		glfw.PostEmptyEvent()
	}
}
