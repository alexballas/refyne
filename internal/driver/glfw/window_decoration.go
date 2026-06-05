//go:build !wasm && !test_web_driver

package glfw

import (
	"image/color"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/driver/desktop"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/alexballas/refyne/v2/widget"
)

// titleBarHeight is the height of the custom client-side title bar.
const titleBarHeight = float32(32)

// windowDecorationButtonInset keeps the visible button highlight away from the
// edges of the full-size title-bar hit area.
const windowDecorationButtonInset = float32(4)

// windowDecorationButtonIconSize is the size of the minimize / maximize /
// close glyphs inside the title-bar buttons.
const windowDecorationButtonIconSize = float32(16)

// windowDecoration is a themed client-side title bar: app icon, title text,
// and minimize / maximize-restore / close buttons. Move/resize/maximize wiring
// is supplied by the controller via the on* callbacks.
type windowDecoration struct {
	widget.BaseWidget

	icon       *canvas.Image
	titleLabel *widget.Label

	minimizeButton *windowDecorationButton
	maximizeButton *windowDecorationButton
	closeButton    *windowDecorationButton

	onMinimize       func()
	onMaximizeToggle func()
	onClose          func()
	onDragStart      func() // called when a drag begins on the title bar
	onDoubleTap      func() // double-click title bar -> toggle maximize
	squareCorners    bool
}

func newWindowDecoration(title string, iconRes fyne.Resource) *windowDecoration {
	d := &windowDecoration{}
	// The title is shown in full or not at all: it is hidden once it no longer
	// fits (see the renderer's Layout) rather than being truncated to an
	// ellipsis, so leave Truncation off.
	d.titleLabel = widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	d.icon = canvas.NewImageFromResource(iconRes)
	d.icon.FillMode = canvas.ImageFillContain
	d.icon.SetMinSize(fyne.NewSquareSize(titleBarHeight - theme.Padding()*2))

	d.minimizeButton = newWindowDecorationButton(theme.WindowMinimizeIcon(), func() {
		if d.onMinimize != nil {
			d.onMinimize()
		}
	})
	d.maximizeButton = newWindowDecorationButton(theme.WindowMaximizeIcon(), func() {
		if d.onMaximizeToggle != nil {
			d.onMaximizeToggle()
		}
	})
	d.closeButton = newWindowDecorationButton(theme.WindowCloseIcon(), func() {
		if d.onClose != nil {
			d.onClose()
		}
	})

	d.ExtendBaseWidget(d)
	return d
}

// windowDecorationButton keeps the full title-bar control hit area while
// centering a slightly smaller standard button inside it.
type windowDecorationButton struct {
	widget.BaseWidget
	button *widget.Button
}

func newWindowDecorationButton(icon fyne.Resource, tapped func()) *windowDecorationButton {
	button := widget.NewButtonWithIcon("", icon, tapped)
	button.Importance = widget.LowImportance
	cache.OverrideTheme(button, &windowDecorationButtonTheme{})

	b := &windowDecorationButton{button: button}
	b.ExtendBaseWidget(b)
	return b
}

func (b *windowDecorationButton) CreateRenderer() fyne.WidgetRenderer {
	return &windowDecorationButtonRenderer{button: b.button}
}

func (b *windowDecorationButton) MouseIn(event *desktop.MouseEvent) {
	b.button.MouseIn(event)
}

func (b *windowDecorationButton) MouseMoved(event *desktop.MouseEvent) {
	b.button.MouseMoved(event)
}

func (b *windowDecorationButton) MouseOut() {
	b.button.MouseOut()
}

func (b *windowDecorationButton) SetIcon(icon fyne.Resource) {
	b.button.SetIcon(icon)
}

func (b *windowDecorationButton) Tapped(event *fyne.PointEvent) {
	b.button.Tapped(event)
}

type windowDecorationButtonRenderer struct {
	button *widget.Button
}

func (r *windowDecorationButtonRenderer) Layout(size fyne.Size) {
	highlightSize := fyne.Max(0, fyne.Min(size.Width, size.Height)-windowDecorationButtonInset*2)
	r.button.Resize(fyne.NewSquareSize(highlightSize))
	r.button.Move(fyne.NewPos((size.Width-highlightSize)/2, (size.Height-highlightSize)/2))
}

func (*windowDecorationButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSquareSize(titleBarHeight)
}

func (r *windowDecorationButtonRenderer) Refresh() {
	r.button.Refresh()
}

func (r *windowDecorationButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.button}
}

func (*windowDecorationButtonRenderer) Destroy() {}

// windowDecorationButtonTheme makes the inset hover background circular.
type windowDecorationButtonTheme struct{}

var (
	_ fyne.Tappable     = (*windowDecorationButton)(nil)
	_ desktop.Hoverable = (*windowDecorationButton)(nil)
)

func (*windowDecorationButtonTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.Current().Color(name, variant)
}

func (*windowDecorationButtonTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.Current().Font(style)
}

func (*windowDecorationButtonTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.Current().Icon(name)
}

func (*windowDecorationButtonTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInputRadius {
		return canvas.RadiusMaximum
	}
	if name == theme.SizeNameInlineIcon {
		return windowDecorationButtonIconSize
	}
	return theme.Current().Size(name)
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

// SetCornersSquare flattens (square=true) or restores the rounded top corners.
// Maximized/fullscreen Wayland CSD windows use square corners.
func (d *windowDecoration) SetCornersSquare(square bool) {
	if d.squareCorners == square {
		return
	}
	d.squareCorners = square
	d.Refresh()
}

func (d *windowDecoration) cornerRadius() float32 {
	if d.squareCorners {
		return 0
	}
	return windowCornerRadius
}

func (d *windowDecoration) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	bg.TopLeftCornerRadius = d.cornerRadius()
	bg.TopRightCornerRadius = d.cornerRadius()
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

	// The window controls are anchored to the right edge and are always shown.
	btnSize := size.Height
	x := size.Width
	for i := len(r.buttons) - 1; i >= 0; i-- {
		x -= btnSize
		r.buttons[i].Resize(fyne.NewSquareSize(btnSize))
		r.buttons[i].Move(fyne.NewPos(x, 0))
	}
	controlsLeft := x
	controlsWidth := size.Width - x

	iconSize := size.Height - pad*2
	iconRight := pad + iconSize

	// The title is the first thing to go. Keep it centered in the window with a
	// padding gap before the controls so it never butts up against them, and
	// hide it as soon as its full text no longer fits the available width. It is
	// shown whole or not at all, never truncated to an ellipsis. The icon may
	// still be shown at this point.
	titleInset := controlsWidth + pad
	if iconInset := iconRight + pad; titleInset < iconInset {
		titleInset = iconInset
	}
	titleWidth := size.Width - titleInset*2
	if setDecorationObjectVisible(r.d.titleLabel, titleWidth >= r.d.titleLabel.MinSize().Width) {
		r.d.titleLabel.Resize(fyne.NewSize(titleWidth, size.Height))
		r.d.titleLabel.Move(fyne.NewPos(titleInset, 0))
	}

	// The icon hides independently, and only once the controls have narrowed far
	// enough to reach it (after the title has already gone), so a narrow window
	// keeps the icon while just the title is dropped.
	if setDecorationObjectVisible(r.d.icon, controlsLeft >= iconRight+pad) {
		r.d.icon.Resize(fyne.NewSquareSize(iconSize))
		r.d.icon.Move(fyne.NewPos(pad, pad))
	}
}

// setDecorationObjectVisible toggles obj's visibility, guarding against
// redundant Show/Hide calls so repeated layouts do not queue extra repaints.
// It returns visible so callers can lay the object out only when it is shown.
func setDecorationObjectVisible(obj fyne.CanvasObject, visible bool) bool {
	if visible != obj.Visible() {
		if visible {
			obj.Show()
		} else {
			obj.Hide()
		}
	}
	return visible
}

func (r *windowDecorationRenderer) MinSize() fyne.Size {
	return fyne.NewSize(titleBarHeight*4, titleBarHeight)
}

func (r *windowDecorationRenderer) Refresh() {
	r.bg.FillColor = theme.Color(theme.ColorNameBackground)
	r.bg.TopLeftCornerRadius = r.d.cornerRadius()
	r.bg.TopRightCornerRadius = r.d.cornerRadius()
	r.bg.Refresh()
	canvas.Refresh(r.d)
}

func (r *windowDecorationRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *windowDecorationRenderer) Destroy()                     {}

// Dragged is the user grabbing the title bar. Hand off to the compositor for an
// interactive move (the very first drag delta triggers it; Wayland then drives
// the move and we receive no further deltas).
func (d *windowDecoration) Dragged(_ *fyne.DragEvent) {
	if d.onDragStart != nil {
		d.onDragStart()
	}
}

func (d *windowDecoration) DragEnd() {}

// DoubleTapped toggles maximize/restore.
func (d *windowDecoration) DoubleTapped(_ *fyne.PointEvent) {
	if d.onDoubleTap != nil {
		d.onDoubleTap()
	}
}

func pointInWindowDecoration(c *glCanvas, pos fyne.Position) bool {
	if c == nil || c.decoration == nil {
		return false
	}

	size := c.Size()
	return pos.X >= 0 && pos.X < size.Width &&
		pos.Y >= 0 && pos.Y < c.decorationHeight()
}

// Ensure the widget satisfies the interaction interfaces.
var (
	_ fyne.Draggable      = (*windowDecoration)(nil)
	_ fyne.DoubleTappable = (*windowDecoration)(nil)
)
