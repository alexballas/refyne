//go:build ci || no_glfw || android || ios || mobile

package widget

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
)

func TestScrollContainer_Dragged_Extended(t *testing.T) {
	s, rect, r := newExtendedScroll(t)

	s.Dragged(&fyne.DragEvent{Dragged: fyne.NewDelta(0, -300)})

	assertExtendedScrollMoved(t, s, rect, r)
}
