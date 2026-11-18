//go:build (ci || no_glfw || mobile) && (!windows || !ci)

package mobile

import (
	"strings"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/container"
	"github.com/alexballas/refyne/v2/widget"

	"github.com/stretchr/testify/assert"
)

// Test_canvas_DraggedTargetUsesPressPosition is refyne's version of upstream's
// Test_canvas_DraggedCompanionUsesPressTarget (fyne-io/fyne#6666). Refyne does
// not pass the unused drag direction on to a companion scroller, so it checks
// only that the drag goes to the scroller under the press: the first move
// lands on another column's scroller, which must not be dragged.
// Test_canvas_Dragged needs Scroll to be Draggable, which it is only with the
// tags this file builds with.
func Test_canvas_Dragged(t *testing.T) {
	dragged := false
	var draggedObj fyne.Draggable
	scroll := container.NewScroll(widget.NewLabel("Hi\nHi\nHi"))
	c := newCanvas(fyne.CurrentDevice()).(*canvas)
	c.SetContent(scroll)
	c.Resize(fyne.NewSize(40, 24))
	assert.Equal(t, float32(0), scroll.Offset.Y)

	c.tapDown(fyne.NewPos(32, 5), 0)
	c.tapMove(fyne.NewPos(32, 10), 0, func(wid fyne.Draggable, ev *fyne.DragEvent) {
		wid.Dragged(ev)
		dragged = true
		draggedObj = wid
	})

	assert.True(t, dragged)
	assert.Equal(t, scroll, draggedObj)
	dragged = false
	c.tapMove(fyne.NewPos(32, 5), 0, func(wid fyne.Draggable, ev *fyne.DragEvent) {
		wid.Dragged(ev)
		dragged = true
	})
	assert.True(t, dragged)
	assert.Equal(t, fyne.NewPos(0, 5), scroll.Offset)
}

func Test_canvas_DraggedTargetUsesPressPosition(t *testing.T) {
	leftHeader := container.NewHScroll(widget.NewLabel("Left header"))
	leftParent := container.NewVScroll(container.NewVBox(leftHeader,
		widget.NewLabel(strings.Repeat("left tall content\n", 40))))
	rightHeader := container.NewHScroll(widget.NewLabel("Right header"))
	rightParent := container.NewVScroll(container.NewVBox(rightHeader,
		widget.NewLabel(strings.Repeat("right tall content\n", 40))))

	content := container.NewWithoutLayout(leftParent, rightParent)
	c := newCanvas(fyne.CurrentDevice()).(*canvas)
	c.padded = false
	c.SetContent(content)
	c.Resize(fyne.NewSize(200, 120))
	leftParent.Move(fyne.NewPos(0, 0))
	leftParent.Resize(fyne.NewSize(100, 120))
	rightParent.Move(fyne.NewPos(100, 0))
	rightParent.Resize(fyne.NewSize(100, 120))

	var targets []fyne.Draggable
	var events []*fyne.DragEvent
	drag := func(wid fyne.Draggable, ev *fyne.DragEvent) {
		targets = append(targets, wid)
		events = append(events, ev)
		wid.Dragged(ev)
	}
	c.tapDown(fyne.NewPos(75, 15), 0)
	c.tapMove(fyne.NewPos(125, 5), 0, drag)
	c.tapMove(fyne.NewPos(130, 5), 0, drag)

	assert.Equal(t, []fyne.Draggable{leftHeader, leftHeader}, targets)
	if assert.Len(t, events, 2) {
		// the pressed header is at the canvas origin, so its drag positions are where the finger is
		assert.Equal(t, fyne.NewPos(125, 5), events[0].Position)
		assert.Equal(t, fyne.NewPos(130, 5), events[1].Position)
	}
	assert.Equal(t, fyne.NewPos(0, 0), rightHeader.Offset)
	assert.Equal(t, fyne.NewPos(0, 0), leftParent.Offset)
	assert.Equal(t, fyne.NewPos(0, 0), rightParent.Offset)
}
