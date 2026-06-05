//go:build !wasm && !test_web_driver && ((linux && !x11 && wayland) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import (
	"unsafe"

	"github.com/alexballas/refyne/v2/driver"
)

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	context := driver.WaylandWindowContext{}
	if v := w.view(); v != nil {
		context.WaylandSurface = uintptr(unsafe.Pointer(v.GetWaylandWindow()))
	}

	f(context)
}
