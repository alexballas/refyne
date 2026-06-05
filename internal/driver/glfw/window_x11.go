//go:build !wasm && !test_web_driver && ((linux && x11 && !wayland) || ((freebsd || netbsd || openbsd) && !wayland))

package glfw

import "github.com/alexballas/refyne/v2/driver"

// assert we are implementing driver.NativeWindow
var _ driver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(any)) {
	context := driver.X11WindowContext{}
	if v := w.view(); v != nil {
		context.WindowHandle = uintptr(v.GetX11Window())
	}

	f(context)
}
