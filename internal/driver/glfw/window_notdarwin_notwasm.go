//go:build !darwin && !wasm && !test_web_driver

package glfw

import "github.com/alexballas/refyne/v2/internal/glfw"

func (w *window) getSecondaryMonitor() *monitor {
	primary := glfw.GetPrimaryMonitor()
	primaryX, primaryY := primary.GetPos()
	for _, candidate := range glfw.GetMonitors() {
		x, y := candidate.GetPos()
		if x != primaryX || y != primaryY {
			return candidate
		}
	}

	return primary
}
