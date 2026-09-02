//go:build !wasm && !test_web_driver

package glfw

func (w *window) processFramebufferResize(width, height int) bool {
	return w.recordFramebufferResize(width, height, runningWayland())
}

func (w *window) recordFramebufferResize(width, height int, wayland bool) bool {
	changed := width != w.framebufferWidth || height != w.framebufferHeight
	w.framebufferWidth, w.framebufferHeight = width, height
	if !wayland || !changed {
		return false
	}

	w.framebufferResizePending = true
	return true
}

// settleFramebufferResize retires the back buffer acquired before the latest
// Wayland framebuffer resize. The caller paints and swaps the newly acquired
// buffer after this returns.
func (w *window) settleFramebufferResize(swap func()) bool {
	if !w.framebufferResizePending {
		return false
	}

	w.framebufferResizePending = false
	swap()
	return true
}
