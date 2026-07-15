//go:build !darwin && !wasm && !test_web_driver

package glfw

import (
	"testing"

	"github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/stretchr/testify/assert"
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
