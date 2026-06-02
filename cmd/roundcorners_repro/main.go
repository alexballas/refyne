package main

import (
	"image/color"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/app"
	"github.com/alexballas/refyne/v2/canvas"
)

func main() {
	a := app.NewWithID("org.fyne.roundcorners-repro")

	parent := a.NewWindow("Parent")
	parent.SetContent(canvas.NewRectangle(color.NRGBA{R: 40, G: 90, B: 150, A: 255}))
	parent.Resize(fyne.NewSize(760, 520))
	parent.CenterOnScreen()
	parent.Show()

	child := a.NewWindow("Child")
	child.SetContent(canvas.NewRectangle(color.NRGBA{R: 70, G: 130, B: 80, A: 255}))
	child.Resize(fyne.NewSize(460, 300))
	child.CenterOnScreen()
	child.Show()

	a.Run()
}
