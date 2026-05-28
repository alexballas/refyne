package container_test

import (
	"image"
	"image/color"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/widget"
)

func TestClip_Refresh(t *testing.T) {
	hello := widget.NewLabel("Hello Fyne!")
	clip := container.NewClip(hello)
	clip.Resize(fyne.NewSize(28, 20))

	w := test.NewWindow(container.NewWithoutLayout(clip))
	w.Resize(fyne.NewSquareSize(50))

	clip.Content = canvas.NewRectangle(color.White)
	clip.Refresh()
	test.AssertImageMatches(t, "clip/rect.png", w.Canvas().Capture().(*image.NRGBA))

	clip.Content = &widget.Button{Text: "Bye", Importance: widget.HighImportance}
	clip.Refresh()
	test.AssertImageMatches(t, "clip/button.png", w.Canvas().Capture().(*image.NRGBA))
}

func TestClip(t *testing.T) {
	hello := widget.NewLabel("Hello Fyne!")
	clip := container.NewClip(hello)
	clip.Resize(fyne.NewSize(28, 20))

	w := test.NewWindow(container.NewWithoutLayout(clip))
	w.Resize(fyne.NewSquareSize(50))
	test.AssertImageMatches(t, "clip/text.png", w.Canvas().Capture().(*image.NRGBA))
}
