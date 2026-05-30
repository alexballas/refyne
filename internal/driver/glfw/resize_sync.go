//go:build !wasm && !test_web_driver && !linux && !freebsd && !openbsd && !netbsd

package glfw

import "github.com/go-gl/glfw/v3.4/glfw"

// resized applies the resize synchronously, inside the GLFW size callback.
//
// On Windows and macOS glfw.PollEvents blocks inside the OS modal resize/move
// loop for the entire duration of a drag (DispatchMessageW on Win32,
// -[NSApp sendEvent:] tracking loop on Cocoa). The size callback still fires
// during that loop, but the run loop never gets to drawSingleFrame, so a
// deferred applyPendingResize would not run until the user releases the mouse -
// freezing the window content mid-drag. Applying the resize here, in-callback,
// is the only way to track the drag live: platformResize on these platforms
// resizes and repaints synchronously (see window_notxdg.go).
func (w *window) resized(_ *glfw.Window, width, height int) {
	w.processResized(width, height)
}

// applyPendingResize is a no-op on Windows/macOS: resizes are applied
// synchronously in resized above. Coalescing them onto the frame loop would
// freeze live resize because PollEvents blocks the frame loop during the drag.
func (w *window) applyPendingResize() {}
