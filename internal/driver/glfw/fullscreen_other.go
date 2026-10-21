//go:build wasm || test_web_driver || !((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

func (w *window) fullScreenOnCurrentOutput() bool { return false }
