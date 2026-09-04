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
	w.beginFramebufferPresentation(true)
	if !w.recordFramebufferResize(configuredSize.width, configuredSize.height, true) {
		t.Fatal("initial Wayland framebuffer size change did not request settling")
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

func TestFramebufferResizeDoesNotSettleAfterFirstPresentation(t *testing.T) {
	w := &window{framebufferWidth: 716, framebufferHeight: 873}
	w.beginFramebufferPresentation(true)
	w.completeFramebufferPresentation()

	if !w.recordFramebufferResize(800, 900, true) {
		t.Fatal("live Wayland framebuffer size change was not recorded")
	}
	if w.framebufferResizePending {
		t.Fatal("live Wayland resize requested an unpainted settling present")
	}

	swaps := 0
	if w.settleFramebufferResize(func() { swaps++ }) {
		t.Fatal("live Wayland resize performed a settling present")
	}
	if swaps != 0 {
		t.Fatalf("live Wayland resize swaps = %d, want 0", swaps)
	}
}

func TestFramebufferResizeSettlesAgainAfterRemap(t *testing.T) {
	w := &window{framebufferWidth: 716, framebufferHeight: 873}
	w.beginFramebufferPresentation(true)
	w.completeFramebufferPresentation()

	w.beginFramebufferPresentation(true)
	if !w.recordFramebufferResize(800, 900, true) {
		t.Fatal("remapped Wayland framebuffer size change did not request settling")
	}
}
