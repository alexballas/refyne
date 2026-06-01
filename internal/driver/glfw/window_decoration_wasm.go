//go:build wasm || test_web_driver

package glfw

import fyne "github.com/alexballas/refyne/v2"

func pointInWindowDecoration(_ *glCanvas, _ fyne.Position) bool {
	return false
}
