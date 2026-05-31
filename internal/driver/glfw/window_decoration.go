//go:build !wasm && !test_web_driver

package glfw

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/alexballas/refyne/v2/widget"
)

// titleBarHeight is the height of the custom client-side title bar.
const titleBarHeight = float32(32)

// windowDecoration is a themed client-side title bar: app icon, title text,
// and minimize / maximize-restore / close buttons. Move/resize/maximize wiring
// is supplied by the controller via the on* callbacks.
type windowDecoration struct {
	widget.BaseWidget

	icon       *canvas.Image
	titleLabel *widget.Label

	minimizeButton *widget.Button
	maximizeButton *widget.Button
	closeButton    *widget.Button

	onMinimize       func()
	onMaximizeToggle func()
	onClose          func()
	onDragStart      func() // called when a drag begins on the title bar
	onDoubleTap      func() // double-click title bar -> toggle maximize
}

func newWindowDecoration(title string, iconRes fyne.Resource) *windowDecoration {
	d := &windowDecoration{}
	d.titleLabel = widget.NewLabel(title)
	d.titleLabel.Truncation = fyne.TextTruncateEllipsis

	d.icon = canvas.NewImageFromResource(iconRes)
	d.icon.FillMode = canvas.ImageFillContain
	d.icon.SetMinSize(fyne.NewSquareSize(titleBarHeight - theme.Padding()*2))

	d.minimizeButton = widget.NewButtonWithIcon("", theme.WindowMinimizeIcon(), func() {
		if d.onMinimize != nil {
			d.onMinimize()
		}
	})
	d.maximizeButton = widget.NewButtonWithIcon("", theme.WindowMaximizeIcon(), func() {
		if d.onMaximizeToggle != nil {
			d.onMaximizeToggle()
		}
	})
	d.closeButton = widget.NewButtonWithIcon("", theme.WindowCloseIcon(), func() {
		if d.onClose != nil {
			d.onClose()
		}
	})
	for _, b := range []*widget.Button{d.minimizeButton, d.maximizeButton, d.closeButton} {
		b.Importance = widget.LowImportance
	}

	d.ExtendBaseWidget(d)
	return d
}

func (d *windowDecoration) SetTitle(title string) {
	d.titleLabel.SetText(title)
}

// SetMaximized swaps the maximize/restore glyph. refyne's theme has no dedicated
// WindowRestoreIcon, so ViewRestoreIcon is used for the restore state.
func (d *windowDecoration) SetMaximized(max bool) {
	if max {
		d.maximizeButton.SetIcon(theme.ViewRestoreIcon())
	} else {
		d.maximizeButton.SetIcon(theme.WindowMaximizeIcon())
	}
}

func (d *windowDecoration) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	buttons := []fyne.CanvasObject{d.minimizeButton, d.maximizeButton, d.closeButton}
	return &windowDecorationRenderer{
		d:       d,
		bg:      bg,
		buttons: buttons,
		objects: append([]fyne.CanvasObject{bg, d.icon, d.titleLabel}, buttons...),
	}
}

type windowDecorationRenderer struct {
	d       *windowDecoration
	bg      *canvas.Rectangle
	buttons []fyne.CanvasObject
	objects []fyne.CanvasObject
}

func (r *windowDecorationRenderer) Layout(size fyne.Size) {
	pad := theme.Padding()
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	iconSize := size.Height - pad*2
	r.d.icon.Resize(fyne.NewSquareSize(iconSize))
	r.d.icon.Move(fyne.NewPos(pad, pad))

	btnSize := size.Height
	x := size.Width
	for i := len(r.buttons) - 1; i >= 0; i-- {
		x -= btnSize
		r.buttons[i].Resize(fyne.NewSquareSize(btnSize))
		r.buttons[i].Move(fyne.NewPos(x, 0))
	}

	titleX := pad*2 + iconSize
	r.d.titleLabel.Resize(fyne.NewSize(x-titleX, size.Height))
	r.d.titleLabel.Move(fyne.NewPos(titleX, 0))
}

func (r *windowDecorationRenderer) MinSize() fyne.Size {
	return fyne.NewSize(titleBarHeight*4, titleBarHeight)
}

func (r *windowDecorationRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameBackground)
	r.bg.Refresh()
	canvas.Refresh(r.d)
}

func (r *windowDecorationRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *windowDecorationRenderer) Destroy()                     {}
