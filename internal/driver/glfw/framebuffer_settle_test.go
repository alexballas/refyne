//go:build !wasm && !test_web_driver

package glfw

import (
	"reflect"
	"testing"
)

func TestFramebufferResizeSettlesStaleEGLBuffer(t *testing.T) {
	type bufferSize struct{ width, height int }

	oldSize := bufferSize{width: 760, height: 420}
	configuredSize := bufferSize{width: 716, height: 873}
	backBuffer := oldSize
	var attached, painted []bufferSize

	w := &window{framebufferWidth: oldSize.width, framebufferHeight: oldSize.height}
	if !w.recordFramebufferResize(configuredSize.width, configuredSize.height, true) {
		t.Fatal("Wayland framebuffer size change did not request settling")
	}
	paint := func() {
		painted = append(painted, backBuffer)
	}
	swap := func() {
		attached = append(attached, backBuffer)
		// Wayland EGL commonly acquires the configured-size back buffer only
		// after it has retired the pre-resize buffer with one swap.
		backBuffer = configuredSize
	}

	if !w.settleFramebufferResize(swap) {
		t.Fatal("framebuffer resize did not request a settling present")
	}
	paint() // repaintWindow paints after the settling swap acquires a fresh buffer
	swap()  // repaintWindow's final, compositor-gated swap

	if got, want := attached, []bufferSize{oldSize, configuredSize}; !reflect.DeepEqual(got, want) {
		t.Fatalf("attached buffers = %v, want stale buffer retired before configured buffer", got)
	}
	if got, want := painted, []bufferSize{configuredSize}; !reflect.DeepEqual(got, want) {
		t.Fatalf("repainted buffers = %v, want configured-size back buffer", got)
	}
	if w.framebufferResizePending {
		t.Fatal("framebuffer resize remained pending after settling present")
	}
}
