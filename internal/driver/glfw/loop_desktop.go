//go:build !wasm && !test_web_driver

package glfw

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/glfw"
)

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

func (d *gLDriver) pollEvents() {
	glfw.PollEvents() // This call blocks while window is being resized, which prevents freeDirtyTextures from being called
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}
