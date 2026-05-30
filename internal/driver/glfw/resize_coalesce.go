//go:build !wasm && !test_web_driver && (linux || freebsd || openbsd || netbsd)

package glfw

import "github.com/alexballas/refyne/v2/internal/glfw"

// resized coalesces interactive-resize configure events. On X11/Wayland
// glfw.PollEvents does not block while the window is being resized: the run loop
// keeps ticking, so a fast drag delivers many configure events between frames.
// Running the full resize (e.g. PopUp.Refresh over an open dialog) for each
// would saturate the main thread, so we stash the latest size and apply it once
// per frame from applyPendingResize.
func (w *window) resized(_ *glfw.Window, width, height int) {
	w.pendingResizeWidth, w.pendingResizeHeight = width, height
	w.pendingResize = true
}

// applyPendingResize applies the most recent coalesced resize, if any. It is
// called on the main thread once per frame from drawSingleFrame, before
// painting, so a burst of configure events costs one canvas.Resize per frame.
func (w *window) applyPendingResize() {
	if !w.pendingResize {
		return
	}
	w.pendingResize = false
	w.processResized(w.pendingResizeWidth, w.pendingResizeHeight)
	w.canvas.SetDirty() // a resize always warrants a repaint
}
