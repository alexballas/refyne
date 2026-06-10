//go:build !no_glfw && !mobile

package glfw

import (
	"runtime"
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertCanvasSize(t *testing.T, w *safeWindow, size fyne.Size) {
	if runtime.GOOS == "linux" {
		// TODO: find the root cause for these problems and solve them without additional repaint
		// fixes issues where the window does not have the correct size
		waitForCanvasSize(t, w, size, false)
	}
	assert.Equal(t, size, w.Canvas().Size())
}

func ensureCanvasSize(t *testing.T, w *safeWindow, size fyne.Size) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// TODO: find the root cause for these problems and solve them without additional repaint
		// fixes issues where the window does not have the correct size
		waitForCanvasSize(t, w, size, true)
	}
	require.Equal(t, size, w.Canvas().Size())
}

func repaintWindow(w *safeWindow) {
	runOnMain(func() {
		w.RunWithContext(func() {
			d.repaintWindow(w.window)
		})
	})

	time.Sleep(time.Millisecond * 150) // wait for the frames to be rendered... o
}

func waitForCanvasSize(t *testing.T, w *safeWindow, size fyne.Size, resizeIfNecessary bool) {
	// loaded CI runners can take several seconds to deliver resize events
	deadline := time.Now().Add(10 * time.Second)
	lastResize := time.Now()
	for w.Canvas().Size() != size {
		if !assert.False(t, time.Now().After(deadline), "canvas did not get correct size in time") {
			break
		}
		if resizeIfNecessary && time.Since(lastResize) >= 200*time.Millisecond {
			// sometimes the resize does not seem to reach the actual window at all
			w.Resize(size)
			lastResize = time.Now()
		}
		time.Sleep(10 * time.Millisecond)
	}
}
