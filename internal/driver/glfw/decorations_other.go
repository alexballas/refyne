//go:build wasm || test_web_driver || !((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

// applyWaylandWindowHints is a no-op outside Wayland desktop builds (Windows,
// macOS, X11, wasm). The real implementation lives in decorations_wayland.go.
func applyWaylandWindowHints(bool) {}

// initWaylandDecorationCursors is a no-op outside Wayland builds.
func initWaylandDecorationCursors() {}

// setupWaylandDecorations is a no-op outside Wayland builds.
func (w *window) setupWaylandDecorations() {}

// pushWaylandIcon is a no-op outside Wayland builds.
func (w *window) pushWaylandIcon() {}

// updateWaylandResizeCursor is a no-op outside Wayland builds.
func (w *window) updateWaylandResizeCursor() {}

// handleWaylandEdgeResize is a no-op outside Wayland builds; it never starts a
// resize, so the click is processed normally.
func (w *window) handleWaylandEdgeResize() bool { return false }

// handleWaylandWindowMenu is a no-op outside Wayland builds; the secondary
// click is processed normally.
func (w *window) handleWaylandWindowMenu() bool { return false }
