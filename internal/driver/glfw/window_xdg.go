//go:build linux || freebsd || openbsd || netbsd

package glfw

import fyne "github.com/alexballas/refyne/v2"

func (w *window) platformResize(canvasSize fyne.Size) {
	w.canvas.Resize(canvasSize)
}
