//go:build !wasm && !test_web_driver && !android && !ios && !mobile

package gl

import "unsafe"

// floatsPtr passes a vertex slice to GL without gl.Ptr, which boxes the slice
// into an interface and so allocates on every draw. GL copies the data before
// the call returns, so no reference outlives it.
func floatsPtr(points []float32) unsafe.Pointer {
	if len(points) == 0 {
		return nil
	}
	return unsafe.Pointer(&points[0])
}
