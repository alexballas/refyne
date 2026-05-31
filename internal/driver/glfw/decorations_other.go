//go:build wasm || !wayland || (!linux && !freebsd && !openbsd && !netbsd)

package glfw

// applyWaylandWindowHints is a no-op outside Wayland desktop builds (Windows,
// macOS, X11, wasm). The real implementation lives in decorations_wayland.go.
func applyWaylandWindowHints() {}
