//go:build wasm || test_web_driver

package glfw

import (
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/fyne-io/gl-js"
	"github.com/fyne-io/glfw-js"
)

// frameInterval matches the desktop signature. The glfw-js shim does not expose
// monitor video modes, so we keep the historical 60 Hz cadence in the browser.
func (d *gLDriver) frameInterval() time.Duration {
	return time.Second / 60
}

func (d *gLDriver) initGLFW() {
	err := glfw.Init(gl.ContextWatcher)
	if err != nil {
		fyne.LogError("failed to initialise GLFW", err)
		return
	}
}

func (d *gLDriver) pollEvents() {
	glfw.PollEvents() // This call blocks while window is being resized, which prevents freeDirtyTextures from being called
}

func (d *gLDriver) Terminate() {
	glfw.Terminate()
}
