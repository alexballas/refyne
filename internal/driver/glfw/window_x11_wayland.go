//go:build !wasm && !test_web_driver && ((linux && !x11 && !wayland) || (linux && x11 && wayland))

package glfw

import (
	"unsafe"

	"github.com/alexballas/refyne/v2/driver"
)

// This file covers the Linux builds that compile both the X11 and Wayland
// backends (the default build with no display tags, and the explicit
// -tags "x11 wayland" build). Which one is live is only known at runtime, so
// RunNative dispatches on the active GLFW platform instead of a build tag.

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	v := w.view()

	if runningWayland() {
		context := driver.WaylandWindowContext{}
		if v != nil {
			context.WaylandSurface = uintptr(unsafe.Pointer(v.GetWaylandWindow()))
		}
		f(context)
		return
	}

	context := driver.X11WindowContext{}
	if v != nil {
		context.WindowHandle = uintptr(v.GetX11Window())
	}
	f(context)
}
