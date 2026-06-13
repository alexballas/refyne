//go:build !no_glfw && !mobile

package glfw

import (
	"testing"
	"time"
	"unsafe"

	fyne "github.com/alexballas/refyne/v2"
	iglfw "github.com/alexballas/refyne/v2/internal/glfw"
)

type notReadyGate struct{}

func (notReadyGate) ready() bool        { return false }
func (notReadyGate) arm(unsafe.Pointer) {}
func (notReadyGate) markReady()         {}
func (notReadyGate) free()              {}

func TestHasReadyDirtyWindow(t *testing.T) {
	c := newCanvas()
	w := &window{canvas: c, frame: noGate{}, viewport: &iglfw.Window{}, visible: true}
	d := &gLDriver{windows: []fyne.Window{w}}

	if d.hasReadyDirtyWindow() {
		t.Fatal("clean window reported as ready dirty")
	}

	c.SetDirty()
	if !d.hasReadyDirtyWindow() {
		t.Fatal("visible ready dirty window was not reported")
	}

	c.CheckDirtyAndClear()
	w.visible = false
	c.SetDirty()
	if d.hasReadyDirtyWindow() {
		t.Fatal("hidden dirty window reported as ready dirty")
	}

	w.visible = true
	w.frame = notReadyGate{}
	if d.hasReadyDirtyWindow() {
		t.Fatal("not-ready dirty window reported as ready dirty")
	}
}

func TestNeedsFrameTickWithAnimation(t *testing.T) {
	d := &gLDriver{}
	if d.needsFrameTick() {
		t.Fatal("idle driver reported frame tick needed")
	}

	a := fyne.NewAnimation(time.Second, func(float32) {})
	d.animation.Start(a)
	if !d.needsFrameTick() {
		t.Fatal("running animation did not request frame tick")
	}
}

// BenchmarkRunOnMain measures the cost of calling a function
// on the main thread.
func BenchmarkRunOnMain(b *testing.B) {
	f := func() {}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runOnMain(f)
	}
}

// BenchmarkRunOnDraw measures the cost of calling a function
// on the draw thread.
func BenchmarkRunOnDraw(b *testing.B) {
	f := func() {}
	w := createWindow("Test")
	w.create()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.RunWithContext(f)
	}
}
