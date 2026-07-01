//go:build !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

// interactiveResizing reports whether the compositor is driving an interactive
// resize of this window (xdg_toplevel RESIZING state). In the default
// both-backends build this file is also compiled on X11, where the Wayland
// window state does not exist; the runtime guard returns false there.
func (w *window) interactiveResizing() bool {
	if !runningWayland() || w.viewport == nil {
		return false
	}
	return w.viewport.InteractiveResizingWayland()
}
