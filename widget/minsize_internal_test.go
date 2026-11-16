package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/cache"
)

type countingWidget struct {
	BaseWidget
	calls int
}

func (w *countingWidget) CreateRenderer() fyne.WidgetRenderer {
	return &countingRenderer{w: w}
}

type countingRenderer struct {
	w *countingWidget
}

func (r *countingRenderer) MinSize() fyne.Size {
	r.w.calls++
	return fyne.NewSize(10, 10)
}

func (*countingRenderer) Layout(fyne.Size)             {}
func (*countingRenderer) Refresh()                     {}
func (*countingRenderer) Objects() []fyne.CanvasObject { return nil }
func (*countingRenderer) Destroy()                     {}

func TestBaseWidget_MinSizeOncePerPass(t *testing.T) {
	w := &countingWidget{}
	w.ExtendBaseWidget(w)

	w.MinSize()
	w.MinSize()
	assert.Equal(t, 2, w.calls, "outside a pass every call asks the renderer")

	cache.BeginMinSizePass()
	defer cache.EndMinSizePass()
	w.MinSize()
	w.MinSize()
	assert.Equal(t, 3, w.calls, "in a pass the renderer is asked once")
	w.Resize(fyne.NewSize(20, 20))
	w.MinSize()
	assert.Equal(t, 4, w.calls, "a resize can change the min size")
	w.Refresh()
	w.MinSize()
	assert.Equal(t, 5, w.calls, "so can a refresh")

	cache.EndMinSizePass()
	w.MinSize()
	assert.Equal(t, 6, w.calls)
	cache.BeginMinSizePass()
	w.MinSize()
	assert.Equal(t, 7, w.calls, "a new pass computes anew")
}

func TestBaseWidget_MinSizeInPassFollowsOtherWidgetChanges(t *testing.T) {
	above := &countingWidget{}
	above.ExtendBaseWidget(above)
	below := &countingWidget{}
	below.ExtendBaseWidget(below)

	cache.BeginMinSizePass()
	defer cache.EndMinSizePass()
	above.MinSize()
	below.MinSize()
	above.MinSize()
	assert.Equal(t, 1, above.calls)

	// a layout resizing a widget in the pass can change the min size of
	// the widgets above it, which were computed already
	below.Resize(fyne.NewSize(20, 20))
	above.MinSize()
	above.MinSize()
	assert.Equal(t, 2, above.calls, "a resize in the pass restarts it")

	below.Refresh()
	above.MinSize()
	assert.Equal(t, 3, above.calls, "so does a refresh")

	below.Resize(fyne.NewSize(20, 20))
	above.MinSize()
	assert.Equal(t, 3, above.calls, "a resize to the same size changes nothing")
}
