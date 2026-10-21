//go:build !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

func (w *window) fullScreenOnCurrentOutput() bool {
	if !runningWayland() {
		return false
	}
	w.viewport.FullScreenOnCurrentOutputWayland()
	return true
}
