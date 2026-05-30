//go:build ((!gles && !arm && !arm64) || darwin) && !wasm && !test_web_driver

package glfw

import "github.com/alexballas/refyne/v2/internal/glfw"

func initWindowHints() {
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

	glfw.WindowHint(glfw.CocoaGraphicsSwitching, glfw.True)
}
