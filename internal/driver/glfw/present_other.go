//go:build wasm || test_web_driver || !((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import "unsafe"

// newPresentGate returns the no-op gate on every non-Wayland build (X11,
// Windows, macOS, wasm), so the render loop is unchanged off Wayland.
func newPresentGate() presentGate { return noGate{} }

// windowSurface has no meaning off Wayland.
func windowSurface(_ *window) unsafe.Pointer { return nil }
