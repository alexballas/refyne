// Package main loads a very basic Hello World graphical application.
package main

import (
	"github.com/alexballas/refyne/v2/app"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Hello")

	hello := widget.NewLabel("Hello Fyne!")
	w.SetContent(container.NewVBox(
		hello,
		widget.NewButton("Hi!", func() {
			hello.SetText("Welcome 😀")
		}),
	))

	w.ShowAndRun()
}
