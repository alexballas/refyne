//go:build !wasm && !test_web_driver

package glfw

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"

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

// windowDecorationButtonSymbolSize is the logical size of the minimize /
// maximize / close glyph drawn inside each title-bar button. It matches the 8px
// symbol that libdecor's GTK/Adwaita plugin draws, measured against a reference
// window. The value is even so the glyph stays pixel-centered in the 32px cell,
// and the stroke stays one logical pixel (scaled to the device), so it is crisp
// at any display scale.
const windowDecorationButtonSymbolSize = float32(8)

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

	d.minimizeButton = newWindowDecorationButton(decorationMinimize, func() {
		if d.onMinimize != nil {
			d.onMinimize()
		}
	})
	d.maximizeButton = newWindowDecorationButton(decorationMaximize, func() {
		if d.onMaximizeToggle != nil {
			d.onMaximizeToggle()
		}
	})
	d.closeButton = newWindowDecorationButton(decorationClose, func() {
		if d.onClose != nil {
			d.onClose()
		}
	})

	d.ExtendBaseWidget(d)
	return d
}

// windowDecorationButtonKind selects which control glyph a button draws.
type windowDecorationButtonKind uint8

const (
	decorationMinimize windowDecorationButtonKind = iota
	decorationMaximize
	decorationRestore
	decorationClose
)

// windowDecorationButton keeps the full title-bar control hit area while
// centering a slightly smaller standard button inside it. The button supplies
// the circular hover highlight and tap handling; the control glyph itself is
// drawn by a canvas.Raster (see draw) so it is rasterized crisply at the exact
// device-pixel size rather than scaled down from a fixed 24px SVG.
type windowDecorationButton struct {
	widget.BaseWidget
	button *widget.Button
	symbol *canvas.Raster
	kind   windowDecorationButtonKind
}

func newWindowDecorationButton(kind windowDecorationButtonKind, tapped func()) *windowDecorationButton {
	button := widget.NewButton("", tapped)
	button.Importance = widget.LowImportance
	cache.OverrideTheme(button, &windowDecorationButtonTheme{})

	b := &windowDecorationButton{button: button, kind: kind}
	b.symbol = canvas.NewRaster(b.draw)
	// The glyph is generated at the exact device-pixel size, so blit it 1:1 with
	// no smoothing to keep the snapped edges crisp at any display scale.
	b.symbol.ScaleMode = canvas.ImageScalePixels
	b.ExtendBaseWidget(b)
	return b
}

// setKind swaps the drawn glyph (used to toggle maximize <-> restore).
func (b *windowDecorationButton) setKind(kind windowDecorationButtonKind) {
	if b.kind == kind {
		return
	}
	b.kind = kind
	b.symbol.Refresh()
}

func (b *windowDecorationButton) CreateRenderer() fyne.WidgetRenderer {
	return &windowDecorationButtonRenderer{button: b.button, symbol: b.symbol}
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

func (b *windowDecorationButton) Tapped(event *fyne.PointEvent) {
	b.button.Tapped(event)
}

// draw renders the control glyph into a w x h device-pixel image. Axis-aligned
// strokes are filled on whole-pixel boundaries so they stay crisp at any scale;
// only the close cross is anti-aliased, exactly as libdecor's cairo plugin does.
func (b *windowDecorationButton) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}

	col := theme.Color(theme.ColorNameForeground)

	// Stroke weight tracks pixel density: one logical pixel scaled to the device,
	// matching libdecor's 1px * buffer-scale lines.
	t := int(math.Round(float64(w) / float64(windowDecorationButtonSymbolSize)))
	if t < 1 {
		t = 1
	}

	switch b.kind {
	case decorationMinimize:
		// A single horizontal bar across the middle of the glyph box.
		y0 := (h - t) / 2
		fillRect(img, 0, y0, w, y0+t, col)
	case decorationMaximize:
		strokeRect(img, 0, 0, w, h, t, col)
	case decorationRestore:
		gap := 2 * t
		side := w - gap
		// Back window (top-right), then clear where the front window will sit so
		// it reads as overlapping, then the front window (bottom-left).
		strokeRect(img, gap, 0, w, side, t, col)
		clearRect(img, 0, t, w-t, h)
		strokeRect(img, 0, h-side, side, h, t, col)
	case decorationClose:
		strokeLine(img, 0, 0, float64(w), float64(h), float64(t), col)
		strokeLine(img, float64(w), 0, 0, float64(h), float64(t), col)
	}
	return img
}

type windowDecorationButtonRenderer struct {
	button *widget.Button
	symbol *canvas.Raster
}

func (r *windowDecorationButtonRenderer) Layout(size fyne.Size) {
	highlightSize := fyne.Max(0, fyne.Min(size.Width, size.Height)-windowDecorationButtonInset*2)
	r.button.Resize(fyne.NewSquareSize(highlightSize))
	r.button.Move(fyne.NewPos((size.Width-highlightSize)/2, (size.Height-highlightSize)/2))

	sym := fyne.Min(windowDecorationButtonSymbolSize, highlightSize)
	r.symbol.Resize(fyne.NewSquareSize(sym))
	r.symbol.Move(fyne.NewPos((size.Width-sym)/2, (size.Height-sym)/2))
}

func (*windowDecorationButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSquareSize(titleBarHeight)
}

func (r *windowDecorationButtonRenderer) Refresh() {
	r.button.Refresh()
	r.symbol.Refresh()
}

func (r *windowDecorationButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.button, r.symbol}
}

func (*windowDecorationButtonRenderer) Destroy() {}

// fillRect paints the half-open device-pixel rectangle [x0,x1) x [y0,y1) with c.
func fillRect(img *image.NRGBA, x0, y0, x1, y1 int, c color.Color) {
	b := img.Bounds()
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	nc := color.NRGBAModel.Convert(c).(color.NRGBA)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetNRGBA(x, y, nc)
		}
	}
}

// clearRect resets a rectangle to transparent so a stroke drawn over it appears
// to sit in front of whatever was there before.
func clearRect(img *image.NRGBA, x0, y0, x1, y1 int) {
	fillRect(img, x0, y0, x1, y1, color.NRGBA{})
}

// strokeRect outlines [x0,x1) x [y0,y1) with a t-pixel border built from four
// filled rectangles, so every edge lands on whole pixels.
func strokeRect(img *image.NRGBA, x0, y0, x1, y1, t int, c color.Color) {
	fillRect(img, x0, y0, x1, y0+t, c) // top
	fillRect(img, x0, y1-t, x1, y1, c) // bottom
	fillRect(img, x0, y0, x0+t, y1, c) // left
	fillRect(img, x1-t, y0, x1, y1, c) // right
}

// strokeLine fills a t-wide quad from (x0,y0) to (x1,y1) with anti-aliased edges
// via a vector rasterizer - used for the close cross diagonals.
func strokeLine(img *image.NRGBA, x0, y0, x1, y1, t float64, c color.Color) {
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	// Half-thickness offset perpendicular to the line direction.
	nx, ny := -dy/length*t/2, dx/length*t/2
	r := vector.NewRasterizer(img.Bounds().Dx(), img.Bounds().Dy())
	r.MoveTo(float32(x0+nx), float32(y0+ny))
	r.LineTo(float32(x1+nx), float32(y1+ny))
	r.LineTo(float32(x1-nx), float32(y1-ny))
	r.LineTo(float32(x0-nx), float32(y0-ny))
	r.ClosePath()
	r.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{})
}

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
	return theme.Current().Size(name)
}

func (d *windowDecoration) SetTitle(title string) {
	d.titleLabel.SetText(title)
}

// SetMaximized swaps the maximize button between the maximize and restore
// glyphs depending on the current window state.
func (d *windowDecoration) SetMaximized(max bool) {
	if max {
		d.maximizeButton.setKind(decorationRestore)
	} else {
		d.maximizeButton.setKind(decorationMaximize)
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
