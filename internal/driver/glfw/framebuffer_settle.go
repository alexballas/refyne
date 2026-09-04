//go:build !wasm && !test_web_driver

package glfw

// prepareFramebufferPresentation enables the one-off settling swap used while
// mapping a Wayland surface. Live resizes deliberately stay on the normal
// single-swap path: repeatedly swapping an unpainted buffer exposes undefined
// EGL contents on some virtual GPUs.
func (w *window) prepareFramebufferPresentation() {
	w.beginFramebufferPresentation(runningWayland())
}

func (w *window) beginFramebufferPresentation(wayland bool) {
	w.framebufferResizePending = false
	w.framebufferSettleActive = wayland
}

func (w *window) completeFramebufferPresentation() {
	w.framebufferResizePending = false
	w.framebufferSettleActive = false
}

func (w *window) processFramebufferResize(width, height int) bool {
	return w.recordFramebufferResize(width, height, runningWayland())
}

func (w *window) recordFramebufferResize(width, height int, wayland bool) bool {
	changed := width != w.framebufferWidth || height != w.framebufferHeight
	w.framebufferWidth, w.framebufferHeight = width, height
	if !wayland || !changed {
		return false
	}

	if w.framebufferSettleActive {
		w.framebufferResizePending = true
	}
	return true
}

// settleFramebufferResize retires a back buffer acquired before the initial
// mapped size was known. The caller paints and swaps the newly acquired buffer
// after this returns.
func (w *window) settleFramebufferResize(swap func()) bool {
	if !w.framebufferResizePending {
		return false
	}

	w.framebufferResizePending = false
	swap()
	return true
}
