package glfw

import "testing"

func TestNoGateAlwaysReady(t *testing.T) {
	var g presentGate = noGate{}
	if !g.ready() {
		t.Fatal("noGate.ready() = false, want true (off-Wayland must always present)")
	}
	// arm/markReady/free must be safe no-ops.
	g.arm(nil)
	g.markReady()
	g.free()
}
