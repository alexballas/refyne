package glfw

import "unsafe"

// presentGate reports whether a window's surface is currently presentable and
// lets the render loop register interest in the next presentable moment. On
// Wayland this is backed by wl_surface.frame callbacks; elsewhere it is a
// no-op that always reports ready.
type presentGate interface {
	// ready reports whether the compositor is ready to present this surface.
	ready() bool
	// arm requests notification for the next presentable frame. surface is the
	// platform surface handle (a *wl_surface on Wayland) or nil. After arm,
	// ready reports false until the compositor calls back.
	arm(surface unsafe.Pointer)
	// markReady forces the presentable state to true (focus-regain backstop).
	markReady()
	// free releases any resources held by the gate.
	free()
}

// noGate is the off-Wayland implementation: the surface is always presentable,
// so the render loop behaves exactly as before.
type noGate struct{}

func (noGate) ready() bool        { return true }
func (noGate) arm(unsafe.Pointer) {}
func (noGate) markReady()         {}
func (noGate) free()              {}
