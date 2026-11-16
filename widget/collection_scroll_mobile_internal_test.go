//go:build ci || no_glfw || android || ios || mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/test"
)

// TestCollections_TouchDragBindsNewRows is the touch counterpart of
// TestCollections_ScrollBindsNewRows: a drag on the content scrolls it on mobile.
func TestCollections_TouchDragBindsNewRows(t *testing.T) {
	test.NewTempApp(t)

	for _, c := range collectionScrollCases() {
		t.Run(c.name, func(t *testing.T) {
			_, scroller, cv, greatest, initial := setupCollectionScroll(t, c)
			pos := canvasPositionOf(cv, scroller).Add(fyne.NewPos(scroller.Size().Width/2, scroller.Size().Height/2))

			// dragging the finger up moves the content up, so the offset grows
			test.Drag(cv, pos, 0, -500)

			assert.Positive(t, scroller.Offset.Y)
			assert.Greater(t, *greatest, initial, "rows dragged into view must be bound")
		})
	}
}
