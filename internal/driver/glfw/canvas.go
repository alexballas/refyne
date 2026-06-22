package glfw

import (
	"image"
	"image/color"
	"math"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/internal"
	"github.com/alexballas/refyne/v2/internal/app"
	"github.com/alexballas/refyne/v2/internal/build"
	"github.com/alexballas/refyne/v2/internal/driver"
	"github.com/alexballas/refyne/v2/internal/driver/common"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/alexballas/refyne/v2/widget"
)

// Declare conformity with Canvas interface
var _ fyne.Canvas = (*glCanvas)(nil)

// windowCornerRadius is the radius (logical px) of the rounded top corners drawn
// for Wayland client-side decorations.
const windowCornerRadius = float32(12)

// windowBackgroundOverlap fills the anti-aliased boundary between the rounded
// title bar and the opaque window body.
const windowBackgroundOverlap = float32(1)

type glCanvas struct {
	common.Canvas

	content    fyne.CanvasObject
	menu       fyne.CanvasObject
	decoration fyne.CanvasObject // client-side title bar, Wayland CSD only
	background *canvas.Rectangle // opaque body below the rounded title bar, Wayland CSD only
	outline    *canvas.Rectangle // one-pixel inner border, Wayland CSD only
	// squareCorners flattens the rounded top corners while maximized/fullscreen.
	// Wayland CSD only.
	squareCorners bool
	// transparentSurface marks a window presented on an alpha-capable (ARGB)
	// Wayland surface (any decorated window: CSD draws rounded corners, SSD
	// stays opaque). It keeps the painter's framebuffer alpha opaque where
	// content is drawn so semi-transparent widgets are not rendered see-through.
	transparentSurface bool
	padded             bool
	size               fyne.Size

	onTypedRune func(rune)
	onTypedKey  func(*fyne.KeyEvent)
	onKeyDown   func(*fyne.KeyEvent)
	onKeyUp     func(*fyne.KeyEvent)
	// shortcut    fyne.ShortcutHandler

	scale, detectedScale, texScale float32

	context         driver.WithContext
	webExtraWindows *container.MultipleWindows
}

func (c *glCanvas) Capture() image.Image {
	var img image.Image
	c.context.(*window).RunWithContext(func() {
		img = c.Painter().Capture(c)
	})
	return img
}

func (c *glCanvas) Content() fyne.CanvasObject {
	return c.content
}

func (c *glCanvas) DismissMenu() bool {
	if c.menu != nil && c.menu.(*MenuBar).IsActive() {
		c.menu.(*MenuBar).Toggle()
		return true
	}
	return false
}

func (c *glCanvas) InteractiveArea() (fyne.Position, fyne.Size) {
	offset := c.decorationHeight()
	return fyne.NewPos(0, offset), c.Size().SubtractWidthHeight(0, offset)
}

func (c *glCanvas) MinSize() fyne.Size {
	return c.canvasSize(c.content.MinSize())
}

func (c *glCanvas) OnKeyDown() func(*fyne.KeyEvent) {
	return c.onKeyDown
}

func (c *glCanvas) OnKeyUp() func(*fyne.KeyEvent) {
	return c.onKeyUp
}

func (c *glCanvas) OnTypedKey() func(*fyne.KeyEvent) {
	return c.onTypedKey
}

func (c *glCanvas) OnTypedRune() func(rune) {
	return c.onTypedRune
}

func (c *glCanvas) Padded() bool {
	return c.padded
}

func (c *glCanvas) PixelCoordinateForPosition(pos fyne.Position) (int, int) {
	multiple := c.scale * c.texScale
	scaleInt := func(x float32) int {
		return int(math.Round(float64(x * multiple)))
	}

	return scaleInt(pos.X), scaleInt(pos.Y)
}

// PopUpArea returns the part of the GLFW window available to pop-ups. The
// method is consumed by widget.PopUp without changing pop-up behavior on
// canvases that do not provide explicit bounds.
func (c *glCanvas) PopUpArea() (fyne.Position, fyne.Size) {
	return c.InteractiveArea()
}

func (c *glCanvas) Resize(size fyne.Size) {
	// This might not be the ideal solution, but it effectively avoid the first frame to be blurry due to the
	// rounding of the size to the loower integer when scale == 1. It does not affect the other cases as far as we tested.
	// This can easily be seen with fyne/cmd/hello and a scale == 1 as the text will happear blurry without the following line.
	nearestSize := fyne.NewSize(float32(math.Ceil(float64(size.Width))), float32(math.Ceil(float64(size.Height))))

	c.size = nearestSize

	if c.webExtraWindows != nil {
		c.webExtraWindows.Resize(size)
	}
	c.resizeOverlays()

	content := c.content
	contentSize := c.contentSize(nearestSize)
	contentPos := c.contentPos()
	menu := c.menu
	menuHeight := c.menuHeight()
	decoration := c.decoration
	decorationHeight := c.decorationHeight()

	content.Resize(contentSize)
	content.Move(contentPos)

	if decoration != nil {
		decoration.Refresh()
		decoration.Resize(fyne.NewSize(nearestSize.Width, decorationHeight))
		decoration.Move(fyne.NewPos(0, 0))
	}
	if c.background != nil {
		c.background.Resize(c.windowBackgroundSize(nearestSize))
	}
	if c.outline != nil {
		c.outline.Resize(nearestSize)
	}

	if menu != nil {
		menu.Refresh()
		menu.Resize(fyne.NewSize(nearestSize.Width, menuHeight))
		menu.Move(fyne.NewPos(0, decorationHeight))
	}
}

func (c *glCanvas) Scale() float32 {
	return c.scale
}

func (c *glCanvas) SetContent(content fyne.CanvasObject) {
	content.Resize(content.MinSize()) // give it the space it wants then calculate the real min

	// the pass above makes some layouts wide enough to wrap, so we ask again what the true min is.
	newSize := c.size.Max(c.canvasSize(content.MinSize()))

	c.setContent(content)

	c.Resize(newSize)
	c.SetDirty()
}

func (c *glCanvas) SetOnKeyDown(typed func(*fyne.KeyEvent)) {
	c.onKeyDown = typed
}

func (c *glCanvas) SetOnKeyUp(typed func(*fyne.KeyEvent)) {
	c.onKeyUp = typed
}

func (c *glCanvas) SetOnTypedKey(typed func(*fyne.KeyEvent)) {
	c.onTypedKey = typed
}

func (c *glCanvas) SetOnTypedRune(typed func(rune)) {
	c.onTypedRune = typed
}

func (c *glCanvas) SetPadded(padded bool) {
	c.padded = padded

	c.content.Move(c.contentPos())
}

func (c *glCanvas) reloadScale() {
	w := c.context.(*window)
	windowVisible := w.visible
	if !windowVisible {
		return
	}

	c.scale = w.calculatedScale()
	c.SetDirty()

	c.context.RescaleContext()
}

func (c *glCanvas) Size() fyne.Size {
	return c.size
}

func (c *glCanvas) ToggleMenu() {
	if c.menu != nil {
		c.menu.(*MenuBar).Toggle()
	}
}

func (c *glCanvas) buildMenu(w *window, m *fyne.MainMenu) {
	c.setMenuOverlay(nil)
	if m == nil {
		return
	}
	if build.HasNativeMenu {
		setupNativeMenu(w, m)
	} else {
		c.setMenuOverlay(buildMenuOverlay(m, w))
	}
}

// canvasSize computes the needed canvas size for the given content size
func (c *glCanvas) canvasSize(contentSize fyne.Size) fyne.Size {
	canvasSize := contentSize.Add(fyne.NewSize(0, c.menuHeight()+c.decorationHeight()))
	if c.Padded() {
		return canvasSize.Add(fyne.NewSquareSize(theme.Padding() * 2))
	}
	return canvasSize
}

func (c *glCanvas) chromeHeight() float32 {
	return c.menuHeight() + c.decorationHeight()
}

func (c *glCanvas) contentPos() fyne.Position {
	contentPos := fyne.NewPos(0, c.decorationHeight()+c.menuHeight())
	if c.Padded() {
		return contentPos.Add(fyne.NewSquareOffsetPos(theme.Padding()))
	}
	return contentPos
}

func (c *glCanvas) contentSize(canvasSize fyne.Size) fyne.Size {
	contentSize := fyne.NewSize(canvasSize.Width, canvasSize.Height-c.menuHeight()-c.decorationHeight())
	if c.Padded() {
		return contentSize.Subtract(fyne.NewSquareSize(theme.Padding() * 2))
	}
	return contentSize
}

func (c *glCanvas) menuHeight() float32 {
	if c.menu == nil {
		return 0 // no menu or native menu -> does not consume space on the canvas
	}

	return c.menu.MinSize().Height
}

// decorationHeight returns the height reserved at the very top of the canvas for
// the client-side title bar, or 0 when none is shown (the common case).
func (c *glCanvas) decorationHeight() float32 {
	if c.decoration == nil {
		return 0
	}

	return c.decoration.MinSize().Height
}

func (c *glCanvas) overlayChanged() {
	c.resizeOverlays()
	c.SetDirty()
}

func (c *glCanvas) resizeOverlays() {
	areaPos, areaSize := c.InteractiveArea()
	for _, overlay := range c.Overlays().List() {
		if p, ok := overlay.(*widget.PopUp); ok {
			// TODO: remove this when #707 is being addressed.
			// “Notifies” the PopUp of the canvas size change.
			p.Refresh()
		} else {
			overlay.Resize(areaSize)
			overlay.Move(areaPos)
		}
	}
}

func (c *glCanvas) paint(size fyne.Size) {
	clips := &internal.ClipStack{}
	if c.Content() == nil {
		return
	}
	// Rounded Wayland CSD corners require transparent clear pixels outside the
	// title bar shape. The explicit lower body restores opacity behind content.
	c.Painter().SetTransparentBackground(c.background != nil)
	// On an alpha-capable Wayland surface, keep drawn pixels opaque so
	// semi-transparent widgets are not blended away to see-through (the
	// compositor honours the surface alpha). CSD additionally clears the corners
	// transparent above; SSD/fullscreen keep the opaque clear.
	c.Painter().SetPreserveFramebufferAlpha(c.transparentSurface)
	c.Painter().Clear()
	if c.background != nil {
		c.Painter().Paint(c.background, fyne.NewPos(0, c.windowBackgroundTop()), size, nil)
	}

	paint := func(node *common.RenderCacheNode, pos fyne.Position) {
		obj := node.Obj()
		isClip := driver.IsClip(obj)
		node.SetClips(isClip)
		if isClip {
			inner := clips.Push(pos, obj.Size())
			c.Painter().StartClipping(inner.Rect())
		}
		if size.Width <= 0 || size.Height <= 0 { // iconifying on Windows can do bad things
			return
		}
		c.Painter().Paint(obj, pos, size, clips.Top())
	}
	afterPaint := func(node *common.RenderCacheNode, pos fyne.Position) {
		if node.Clips() {
			clips.Pop()
			if top := clips.Top(); top != nil {
				c.Painter().StartClipping(top.Rect())
			} else {
				c.Painter().StopClipping()
			}
		}

		if build.Mode == fyne.BuildDebug {
			c.DrawDebugOverlay(node.Obj(), pos, size, clips.Top())
		}
	}
	c.WalkTrees(paint, afterPaint)
	if c.outline != nil {
		c.Painter().Paint(c.outline, fyne.NewPos(0, 0), size, nil)
	}
}

func (c *glCanvas) setContent(content fyne.CanvasObject) {
	c.content = content
	c.SetContentTreeAndFocusMgr(content)
}

func (c *glCanvas) setMenuOverlay(b fyne.CanvasObject) {
	c.menu = b
	c.SetMenuTreeAndFocusMgr(b)

	if c.menu != nil && !c.size.IsZero() {
		c.content.Resize(c.contentSize(c.size))
		c.content.Move(c.contentPos())

		c.menu.Refresh()
		c.menu.Resize(fyne.NewSize(c.size.Width, c.menu.MinSize().Height))
		c.menu.Move(fyne.NewPos(0, c.decorationHeight()))
	}
}

// setDecoration installs (or clears) the client-side title bar at the top of the
// canvas, mirroring setMenuOverlay. Used for Wayland CSD.
func (c *glCanvas) setDecoration(obj fyne.CanvasObject) {
	c.decoration = obj
	c.SetDecorationTreeAndFocusMgr(obj)
	if obj == nil {
		c.setWindowOutline(false)
		c.setWindowBackground(false)
	}

	if !c.size.IsZero() {
		c.Resize(c.size)
	}
	c.SetDirty()
}

func (c *glCanvas) effectiveCornerRadius() float32 {
	if c.squareCorners {
		return 0
	}
	return windowCornerRadius
}

func (c *glCanvas) windowBackgroundSize(size fyne.Size) fyne.Size {
	return fyne.NewSize(size.Width, fyne.Max(0, size.Height-c.windowBackgroundTop()))
}

func (c *glCanvas) windowBackgroundTop() float32 {
	return fyne.Max(0, c.decorationHeight()-windowBackgroundOverlap)
}

// setWindowOutline enables or clears the inexpensive inner border used to keep
// overlapping Wayland CSD windows visually distinct. The top corners follow the
// rounded title bar; the bottom stays square.
func (c *glCanvas) setWindowOutline(enabled bool) {
	if !enabled {
		c.outline = nil
		c.SetDirty()
		return
	}

	outline := canvas.NewRectangle(color.Transparent)
	outline.StrokeColor = theme.Color(theme.ColorNameShadow)
	outline.StrokeWidth = 1
	outline.TopLeftCornerRadius = c.effectiveCornerRadius()
	outline.TopRightCornerRadius = c.effectiveCornerRadius()
	outline.Resize(c.size)
	c.outline = outline
	c.SetDirty()
}

// setWindowBackground enables or clears the opaque lower window body painted
// behind Wayland CSD content. The title bar paints the rounded top separately.
func (c *glCanvas) setWindowBackground(enabled bool) {
	if !enabled {
		c.background = nil
		c.SetDirty()
		return
	}

	background := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	background.Resize(c.windowBackgroundSize(c.size))
	c.background = background
	c.SetDirty()
}

// setWindowCornersSquare flattens (square=true) or restores the rounded top
// corners of the title bar and outline. Maximized/fullscreen windows are square
// and have no external shadow, matching native decorations.
func (c *glCanvas) setWindowCornersSquare(square bool) {
	decoration, hasRoundedDecoration := c.decoration.(interface{ SetCornersSquare(bool) })
	if c.outline == nil && !hasRoundedDecoration {
		return
	}

	c.squareCorners = square

	radius := c.effectiveCornerRadius()
	if c.outline != nil {
		c.outline.TopLeftCornerRadius = radius
		c.outline.TopRightCornerRadius = radius
		c.outline.Refresh()
	}
	if hasRoundedDecoration {
		decoration.SetCornersSquare(square)
	}
	c.SetDirty()
}

func (c *glCanvas) applyThemeOutOfTreeObjects() {
	if c.menu != nil {
		app.ApplyThemeTo(c.menu, c) // Ensure our menu gets the theme change message as it's out-of-tree
	}
	if c.decoration != nil {
		app.ApplyThemeTo(c.decoration, c) // decoration is out-of-tree too (Wayland CSD)
	}
	if c.background != nil {
		c.background.FillColor = theme.Color(theme.ColorNameBackground)
		c.SetDirty()
	}
	if c.outline != nil {
		c.outline.StrokeColor = theme.Color(theme.ColorNameShadow)
		c.SetDirty()
	}

	c.SetPadded(c.padded) // refresh the padding for potential theme differences
}

func newCanvas() *glCanvas {
	c := &glCanvas{scale: 1.0, texScale: 1.0, padded: true}
	connectKeyboard(c)
	c.Initialize(c, c.overlayChanged)
	c.SetOnDirty(wakeEventLoop)
	c.setContent(&canvas.Rectangle{FillColor: theme.Color(theme.ColorNameBackground)})
	return c
}
