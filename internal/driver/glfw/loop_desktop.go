//go:build !wasm && !test_web_driver

package glfw

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/go-gl/glfw/v3.4/glfw"
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

	initCursors()
}

func (d *gLDriver) pollEvents() {
	glfw.PollEvents() // This call blocks while window is being resized, which prevents freeDirtyTextures from being called
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}
