//go:build wasm || !wayland || (!linux && !freebsd && !openbsd && !netbsd)

package glfw

// applyWaylandWindowHints is a no-op outside Wayland desktop builds (Windows,
// macOS, X11, wasm). The real implementation lives in decorations_wayland.go.
func applyWaylandWindowHints() {}

// setupWaylandDecorations is a no-op outside Wayland builds.
func (w *window) setupWaylandDecorations() {}

// pushWaylandIcon is a no-op outside Wayland builds.
func (w *window) pushWaylandIcon() {}

// handleWaylandEdgeResize is a no-op outside Wayland builds; it never starts a
// resize, so the click is processed normally.
func (w *window) handleWaylandEdgeResize() bool { return false }
