//go:build !no_glfw && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/layout"
	"github.com/alexballas/refyne/v2/widget"
)

// widthFollower's min height follows its width, as wrapped text's does.
type widthFollower struct {
	widget.BaseWidget
}

func (f *widthFollower) CreateRenderer() fyne.WidgetRenderer {
	return &widthFollowerRenderer{f: f}
}

type widthFollowerRenderer struct {
	f *widthFollower
}

func (r *widthFollowerRenderer) MinSize() fyne.Size {
	w := r.f.Size().Width
	if w < 1 {
		w = 1
	}
	return fyne.NewSize(10, 1000/w)
}

func (*widthFollowerRenderer) Layout(fyne.Size)             {}
func (*widthFollowerRenderer) Refresh()                     {}
func (*widthFollowerRenderer) Objects() []fyne.CanvasObject { return nil }
func (*widthFollowerRenderer) Destroy()                     {}

// besideLayout gives objects[0] the width of objects[1]'s min size and
// stacks them, so resizing the second resizes the first during a layout.
type besideLayout struct{}

func (besideLayout) Layout(objects []fyne.CanvasObject, _ fyne.Size) {
	guide := objects[1].MinSize()
	objects[0].Resize(fyne.NewSize(guide.Width, objects[0].MinSize().Height))
	objects[1].Resize(guide)
	objects[1].Move(fyne.NewPos(0, objects[0].Size().Height))
}

func (besideLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	guide := objects[1].MinSize()
	return fyne.NewSize(guide.Width, objects[0].MinSize().Height+guide.Height)
}

// wrapper is a widget holding one object, so a min size kept for the widget
// sits above what its layout resizes.
type wrapper struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func (w *wrapper) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.content)
}

func TestGlCanvas_MinSizeChangedByLayoutInPassReachesAncestors(t *testing.T) {
	w := createWindow("Test")
	c := w.Canvas()
	follower := &widthFollower{}
	follower.ExtendBaseWidget(follower)
	guide := canvas.NewRectangle(color.Black)
	guide.SetMinSize(fyne.NewSize(20, 10))
	outer := &wrapper{content: container.New(besideLayout{}, follower, guide)}
	outer.ExtendBaseWidget(outer)
	content := container.NewVBox(outer)
	w.SetContent(content)
	repaintWindow(w)

	runOnMain(func() {
		assert.Equal(t, float32(1000/20+10), outer.Size().Height)

		guide.SetMinSize(fyne.NewSize(40, 10))
		c.Refresh(guide)
	})
	repaintWindow(w)

	runOnMain(func() {
		// the guide's layout resized the follower in the pass, and the
		// follower's new min size has to reach the widget above it
		assert.Equal(t, float32(40), follower.Size().Width)
		assert.Equal(t, fyne.NewSize(40, 1000/40+10), outer.MinSize())
		assert.Equal(t, float32(1000/40+10), outer.Size().Height)
	})
}

// A GridWrap container's min height follows its width (its row count is set in
// Layout). Here a canvas.Text in a Border's leading slot grows, so the Border's
// layout in the min size pass narrows the GridWrap: no BaseWidget is resized,
// so the min size the wrapper widget kept at the start of the pass is stale.
func TestGlCanvas_MinSizeChangedByNonWidgetLayoutInPassReachesAncestors(t *testing.T) {
	w := createWindow("Test")
	label := canvas.NewText("a", color.Black)
	cells := make([]fyne.CanvasObject, 10)
	for i := range cells {
		r := canvas.NewRectangle(color.Black)
		cells[i] = r
	}
	grid := container.NewGridWrap(fyne.NewSize(20, 20), cells...)
	border := container.NewBorder(nil, nil, label, nil, grid)
	outer := &wrapper{content: border}
	outer.ExtendBaseWidget(outer)
	spacer := canvas.NewRectangle(color.Black)
	spacer.SetMinSize(fyne.NewSize(300, 10))
	content := container.NewVBox(spacer, outer, layout.NewSpacer())
	w.SetContent(content)
	w.Resize(fyne.NewSize(300, 400))
	repaintWindow(w)

	var before float32
	runOnMain(func() {
		before = outer.Size().Height
		label.Text = "a longer text!"
		label.Refresh()
	})
	repaintWindow(w)

	runOnMain(func() {
		t.Logf("grid size %v, grid min %v, border min %v, outer min %v, outer size %v (before %v)",
			grid.Size(), grid.MinSize(), border.MinSize(), outer.MinSize(), outer.Size(), before)
		assert.Greater(t, border.MinSize().Height, before, "test precondition: the grid wraps to more rows")
		assert.Equal(t, border.MinSize().Height, outer.Size().Height, "the wrapper should grow to its new min height")
	})
	repaintWindow(w)
	repaintWindow(w)
	runOnMain(func() {
		assert.Equal(t, border.MinSize().Height, outer.Size().Height, "still stale after more frames")
	})
}

// The same, with frames driven like the real loop: a frame only runs while the
// canvas is dirty, so a stale size would persist once it is clean.
func TestGlCanvas_MinSizeChangedByNonWidgetLayoutInPassOnlyWhileDirty(t *testing.T) {
	w := createWindow("Test")
	label := canvas.NewText("a", color.Black)
	cells := make([]fyne.CanvasObject, 10)
	for i := range cells {
		cells[i] = canvas.NewRectangle(color.Black)
	}
	grid := container.NewGridWrap(fyne.NewSize(20, 20), cells...)
	border := container.NewBorder(nil, nil, label, nil, grid)
	outer := &wrapper{content: border}
	outer.ExtendBaseWidget(outer)
	spacer := canvas.NewRectangle(color.Black)
	spacer.SetMinSize(fyne.NewSize(300, 10))
	w.SetContent(container.NewVBox(spacer, outer, layout.NewSpacer()))
	w.Resize(fyne.NewSize(300, 400))
	repaintWindow(w)

	frames := 0
	runOnMain(func() {
		w.RunWithContext(func() {
			cv := w.canvas
			cv.CheckDirtyAndClear()
			label.Text = "a longer text!"
			cv.Refresh(label) // no fyne app in these tests, so canvas.Refresh is a no-op
			for i := 0; i < 10 && cv.CheckDirtyAndClear(); i++ {
				d.repaintWindow(w.window)
				frames++
			}
		})
	})
	runOnMain(func() {
		t.Logf("frames run while dirty: %d, outer size %v, border min %v", frames, outer.Size(), border.MinSize())
		assert.Equal(t, border.MinSize().Height, outer.Size().Height, "stale after the canvas is clean")
	})
}
