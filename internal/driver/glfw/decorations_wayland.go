//go:build !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import (
	"bytes"
	"image"
	"image/draw"
	"os"
	"path/filepath"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/alexballas/refyne/v2/internal/painter"
	"github.com/alexballas/refyne/v2/internal/scale"
	"github.com/alexballas/refyne/v2/internal/svg"
)

// waylandAppID returns the app_id to advertise to the Wayland compositor. The
// compositor matches it against an installed <app_id>.desktop file to resolve
// the taskbar / title-bar icon and to group windows. Prefer the application's
// unique ID; fall back to the executable's base name so dev runs still get a
// stable, non-generic id; finally fall back to a constant.
func waylandAppID() string {
	if app := fyne.CurrentApp(); app != nil {
		if id := app.UniqueID(); id != "" {
			return id
		}
	}
	if exe, err := os.Executable(); err == nil {
		if base := filepath.Base(exe); base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return "refyne"
}

// applyWaylandWindowHints sets the pre-create window hints specific to Wayland.
// Must be called before glfw.CreateWindow. Decorated windows request an alpha
// framebuffer up front because Mutter CSD is selected only after creation; SSD
// windows continue to render an opaque clear.
func applyWaylandWindowHints(decorate bool) {
	// In the default both-backends build this file is compiled even on X11, so
	// guard the Wayland-only hints (notably the transparent ARGB framebuffer
	// request) behind the live platform; on X11 the window stays opaque.
	if !runningWayland() {
		return
	}

	glfw.WindowHintString(glfw.WaylandAppID, waylandAppID())
	transparent := glfw.False
	if decorate {
		transparent = glfw.True
	}
	// Window hints persist until explicitly changed, so reset this for every
	// window rather than leaving undecorated windows to inherit a prior value.
	glfw.WindowHint(glfw.TransparentFramebuffer, transparent)
}

// waylandResizeBorder is the distance (in logical pixels) from a window edge
// within which a primary-button press starts an interactive edge/corner resize
// while custom (client-side) decorations are active.
const waylandResizeBorder = float32(8)

var waylandDecorationCursors map[glfw.StandardCursor]*glfw.Cursor

func initWaylandDecorationCursors() {
	if !runningWayland() {
		return
	}

	waylandDecorationCursors = make(map[glfw.StandardCursor]*glfw.Cursor, 4)
	for _, shape := range []glfw.StandardCursor{
		glfw.ResizeEWCursor,
		glfw.ResizeNSCursor,
		glfw.ResizeNWSECursor,
		glfw.ResizeNESWCursor,
	} {
		waylandDecorationCursors[shape] = glfw.CreateStandardCursor(shape)
	}
}

func (w *window) waylandCursorPosition() (float64, float64, fyne.Position) {
	xpos, ypos := w.viewport.GetCursorPos()
	return xpos, ypos, fyne.NewPos(
		scale.ToFyneCoordinate(w.canvas, int(xpos)),
		scale.ToFyneCoordinate(w.canvas, int(ypos)),
	)
}

// decorationIcon resolves the resource to show in the title bar / push to the
// compositor: the window icon if set, otherwise the application icon.
func (w *window) decorationIcon() fyne.Resource {
	if w.icon != nil {
		return w.icon
	}
	if app := fyne.CurrentApp(); app != nil {
		return app.Icon()
	}
	return nil
}

// setupWaylandDecorations runs once after the window has been shown. If the
// compositor granted server-side decorations (KDE/wlroots) we leave them in
// place and rely on app_id + pushWaylandIcon for the title-bar icon. Otherwise
// (GNOME/Mutter, or no decoration manager) we suppress GLFW's minimal fallback
// bars and draw our own themed title bar inside the canvas.
func (w *window) setupWaylandDecorations() {
	if !w.decorate {
		return
	}

	// Decorated windows requested a transparent (ARGB) framebuffer before
	// creation (applyWaylandWindowHints), so the compositor honours the surface
	// alpha and no opaque region is set. Keep drawn pixels opaque in the painter
	// for every decorated window — CSD relies on it for the rounded-corner body,
	// SSD/fullscreen rely on it to avoid see-through semi-transparent widgets.
	w.canvas.transparentSurface = true

	// Hide destroys the xdg shell objects and their shadow subsurfaces. The
	// in-canvas decoration remains installed, so a later Show only needs to
	// restore the shadow request for the recreated shell objects. Keep the
	// request active in fullscreen too: the C layer hides the surfaces while
	// fullscreen and recreates them when the window returns to normal.
	if w.canvas.decoration != nil {
		w.viewport.SetWindowShadowWayland(true)
		return
	}

	if w.fullScreen {
		return
	}

	// The decoration mode is delivered asynchronously by the compositor; pump
	// the event queue once so any reply has been processed before we read it.
	// On GNOME (no decoration manager) the mode stays None and we draw our own.
	glfw.PollEvents()

	if w.viewport.DecorationMode() == glfw.DecorationModeServerSide {
		return // real SSD: nothing for us to draw.
	}

	// Take over: remove GLFW's fallback bars (this destroys them on Wayland).
	w.viewport.SetAttrib(glfw.Decorated, glfw.False)

	d := newWindowDecoration(w.title, w.decorationIcon())
	d.SetMaximized(w.viewport.GetAttrib(glfw.Maximized) == glfw.True)
	d.onClose = w.Close
	d.onMinimize = func() { w.viewport.Iconify() }
	d.onMaximizeToggle = func() {
		if w.viewport.GetAttrib(glfw.Maximized) == glfw.True {
			w.viewport.Restore()
		} else {
			w.viewport.Maximize()
		}
	}
	d.onDragStart = func() { w.viewport.StartWindowMove() }
	d.onDoubleTap = d.onMaximizeToggle

	w.updateChrome(func() {
		w.canvas.setDecoration(d)
	})
	w.canvas.setWindowBackground(true)
	w.canvas.setWindowOutline(true)
	w.canvas.setWindowCornersSquare(w.viewport.GetAttrib(glfw.Maximized) == glfw.True || w.fullScreen)
	w.viewport.SetWindowShadowWayland(true)
}

func waylandResizeEdgeAt(pos fyne.Position, size fyne.Size) glfw.ResizeEdge {
	border := waylandResizeBorder
	left := pos.X <= border
	right := pos.X >= size.Width-border
	top := pos.Y <= border
	bottom := pos.Y >= size.Height-border

	switch {
	case top && left:
		return glfw.ResizeEdgeTopLeft
	case top && right:
		return glfw.ResizeEdgeTopRight
	case bottom && left:
		return glfw.ResizeEdgeBottomLeft
	case bottom && right:
		return glfw.ResizeEdgeBottomRight
	case left:
		return glfw.ResizeEdgeLeft
	case right:
		return glfw.ResizeEdgeRight
	case top:
		return glfw.ResizeEdgeTop
	case bottom:
		return glfw.ResizeEdgeBottom
	default:
		return glfw.ResizeEdgeNone
	}
}

func waylandResizeCursorShape(edge glfw.ResizeEdge) (glfw.StandardCursor, bool) {
	switch edge {
	case glfw.ResizeEdgeLeft, glfw.ResizeEdgeRight:
		return glfw.ResizeEWCursor, true
	case glfw.ResizeEdgeTop, glfw.ResizeEdgeBottom:
		return glfw.ResizeNSCursor, true
	case glfw.ResizeEdgeTopLeft, glfw.ResizeEdgeBottomRight:
		return glfw.ResizeNWSECursor, true
	case glfw.ResizeEdgeTopRight, glfw.ResizeEdgeBottomLeft:
		return glfw.ResizeNESWCursor, true
	default:
		return 0, false
	}
}

func (w *window) updateWaylandResizeCursor() {
	if !runningWayland() || w.viewport == nil {
		return
	}

	var cursor *glfw.Cursor
	if w.canvas.decoration != nil && !w.fullScreen {
		edge := waylandResizeEdgeAt(w.mousePos, w.canvas.Size())
		if shape, ok := waylandResizeCursorShape(edge); ok {
			cursor = waylandDecorationCursors[shape]
		}
	}

	if cursor != nil {
		// The regular canvas hover path runs first. Re-apply the frame cursor
		// while inside a resize zone so child widgets cannot override it.
		w.viewport.SetInputMode(CursorMode, CursorNormal)
		w.viewport.SetCursor(cursor)
		w.waylandResizeCursor = cursor
		return
	}
	if w.waylandResizeCursor == nil {
		return
	}

	// Restore the cursor selected by the regular canvas hover path when the
	// pointer leaves the resize border. Reuse customCursor: creating a new
	// custom cursor here would leak one on every border crossing.
	w.waylandResizeCursor = nil
	rawCursor, isCustomCursor := fyneToNativeCursor(w.cursor)
	if isCustomCursor {
		rawCursor = w.customCursor
	}
	if rawCursor == nil {
		w.viewport.SetInputMode(CursorMode, CursorHidden)
		return
	}
	w.viewport.SetInputMode(CursorMode, CursorNormal)
	w.viewport.SetCursor(rawCursor)
}

// handleWaylandEdgeResize starts an interactive edge/corner resize if the
// primary button was pressed within waylandResizeBorder of a window edge while
// custom decorations are active. Returns true if a resize was started (so the
// click should not be processed further). Called from mouseClicked on press.
func (w *window) handleWaylandEdgeResize() bool {
	if w.canvas.decoration == nil || w.fullScreen {
		return false
	}

	_, _, pos := w.waylandCursorPosition()
	edge := waylandResizeEdgeAt(pos, w.canvas.Size())
	if edge == glfw.ResizeEdgeNone {
		return false
	}

	w.viewport.StartWindowResize(edge)
	return true
}

// handleWaylandWindowMenu asks the compositor to show its standard window menu
// when a secondary-button press lands anywhere in our custom title bar.
func (w *window) handleWaylandWindowMenu() bool {
	if w.fullScreen {
		return false
	}

	xpos, ypos, pos := w.waylandCursorPosition()
	if !pointInWindowDecoration(w.canvas, pos) {
		return false
	}

	w.viewport.ShowWindowMenu(int(xpos), int(ypos))
	return true
}

// pushWaylandIcon rasterizes the window/app icon to a square RGBA image and
// hands it to the compositor via xdg-toplevel-icon-v1. No-op if no icon is set
// or the compositor lacks the protocol. The buffer MUST be square: the C
// helper's add_buffer guard silently rejects non-square icons.
func (w *window) pushWaylandIcon() {
	res := w.decorationIcon()
	if res == nil {
		return
	}

	const sz = 64
	var img image.Image
	if svg.IsResourceSVG(res) {
		img = painter.PaintImage(&canvas.Image{Resource: res}, nil, sz, sz)
	} else {
		dec, _, err := image.Decode(bytes.NewReader(res.Content()))
		if err != nil {
			fyne.LogError("Failed to decode image for Wayland window icon", err)
			return
		}
		img = dec
	}

	// Center the source in a square RGBA so the buffer is always square.
	b := img.Bounds()
	side := b.Dx()
	if b.Dy() > side {
		side = b.Dy()
	}
	rgba := image.NewRGBA(image.Rect(0, 0, side, side))
	offset := image.Pt((side-b.Dx())/2, (side-b.Dy())/2)
	draw.Draw(rgba, image.Rectangle{Min: offset, Max: offset.Add(b.Size())}, img, b.Min, draw.Src)

	w.runOnMainWhenCreated(func() {
		w.viewport.SetWindowIconWayland(rgba.Pix, side, side)
	})
}
