// Command fullscreen demonstrates toggling a window's fullscreen state.
package main

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/app"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/widget"
)

func main() {
	a := app.NewWithID("io.refyne.fullscreen")
	w := a.NewWindow("Fullscreen toggle proof of concept")

	toggle := widget.NewButton("Toggle fullscreen", func() {
		w.SetFullScreen(!w.FullScreen())
	})
	w.SetContent(container.NewCenter(container.NewVBox(
		widget.NewLabel("Move this window to another monitor, then toggle fullscreen."),
		toggle,
		widget.NewLabel("Press Escape to leave fullscreen."),
	)))
	w.Canvas().SetOnTypedKey(func(event *fyne.KeyEvent) {
		if event.Name == fyne.KeyEscape {
			w.SetFullScreen(false)
		}
	})
	w.Resize(fyne.NewSize(640, 360))
	w.ShowAndRun()
}
