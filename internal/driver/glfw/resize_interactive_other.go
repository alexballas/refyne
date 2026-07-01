//go:build wasm || test_web_driver || !((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

// interactiveResizing has no meaning off Wayland: no other platform delivers
// compositor-driven resize state, so the size-fight guard never engages.
func (w *window) interactiveResizing() bool { return false }
