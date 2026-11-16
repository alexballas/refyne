package widget

import (
	"strings"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/driver/desktop"
	"github.com/alexballas/refyne/v2/driver/mobile"
	"github.com/alexballas/refyne/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestEntryTouchDownAccountsForScroll(t *testing.T) {
	test.NewApp()
	e := NewMultiLineEntry()
	e.SetText("Line 1\nLine 2\nLine 3\nLine 4\nLine 5")
	e.CreateRenderer()
	e.scroll.Offset.Y = 50

	e.TouchDown(&mobile.TouchEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(10, 10)}})
	assert.Greater(t, e.CursorRow, 0)
}

// The touch that focuses the entry must not scroll it back to the cursor first,
// else the touch lands on the row that the old cursor scrolled into view.
// Unlike upstream, refyne moves the cursor on touch down, so the entry may then
// scroll a little to show the cursor that the touch placed.
func TestEntryTouchDownFocusKeepsScroll(t *testing.T) {
	test.NewApp()
	e := NewMultiLineEntry()
	e.SetText("Line 1\nLine 2\nLine 3\nLine 4\nLine 5")
	w := test.NewTempWindow(t, e)
	w.Resize(fyne.NewSize(150, 60))
	e.scroll.Offset.Y = 50
	row, _ := e.sel.getRowCol(fyne.NewPos(10, 10+50))
	assert.Greater(t, row, 0)

	e.TouchDown(&mobile.TouchEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(10, 10)}})
	assert.True(t, e.focused)
	assert.Equal(t, row, e.CursorRow)
	assert.NotZero(t, e.scroll.Offset.Y)
}

// Disabling wrapping detaches the scroller and must drop its offset, otherwise a
// later triple tap selects a lower line than the one under the pointer.
func TestEntry_WrapOff_ClearsScrollOffset(t *testing.T) {
	test.NewApp()
	e := NewMultiLineEntry()
	e.Wrapping = fyne.TextWrapWord
	e.SetText("Line 1\nLine 2\nLine 3\nLine 4\nLine 5")
	w := test.NewTempWindow(t, e)
	w.Resize(fyne.NewSize(200, 80))

	e.scroll.Offset.Y = 50
	e.Wrapping = fyne.TextWrapOff
	e.Scroll = fyne.ScrollNone
	e.Refresh()

	assert.Nil(t, e.scroll.Content, "expected scroller content to be detached")
	assert.Equal(t, fyne.NewPos(0, 0), e.scroll.Offset)

	pos := fyne.NewPos(10, 10)
	e.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: pos},
		Button:     desktop.MouseButtonPrimary,
	})
	assert.Equal(t, 0, e.CursorRow, "after mouse down")

	touch := &mobile.TouchEvent{PointEvent: fyne.PointEvent{Position: pos}}
	e.TouchDown(touch)
	e.TouchUp(touch)
	assert.Equal(t, 0, e.CursorRow, "after touch")

	e.DoubleTapped(&fyne.PointEvent{Position: pos})
	e.TouchDown(touch)
	assert.Equal(t, "Line 1", e.SelectedText())

	e.Wrapping = fyne.TextWrapWord
	e.Refresh()
	if e.scroll.Content == nil {
		t.Fatal("expected scroller to reattach")
	}
	assert.Equal(t, fyne.NewPos(0, 0), e.scroll.Offset, "after reattach")
	e.sel.doubleTappedAtUnixMillis = 0
	e.TouchDown(touch)
	e.TouchUp(touch)
	assert.Equal(t, 0, e.CursorRow, "after wrap is restored")

	// The demo's Scroll mode turns wrapping off but keeps the scroller, so its offset stays.
	// (upstream's TestEntry_WrapOff_ScrollBothKeepsOffset, folded in here as it guards
	// against the fix going too far, so it cannot fail without it)
	e = NewMultiLineEntry()
	e.Wrapping = fyne.TextWrapWord
	e.SetText("Line 1" + strings.Repeat("\nLine", 19))
	w = test.NewTempWindow(t, e)
	w.Resize(fyne.NewSize(220, 160))

	// Park the cursor on a row that is already inside this offset, so refresh
	// keeps the scroll position instead of chasing the cursor back to the top.
	e.scroll.Offset.Y = 50
	e.CursorRow = 4
	e.Wrapping = fyne.TextWrapOff
	e.Scroll = fyne.ScrollBoth
	e.Refresh()

	if e.scroll.Content == nil {
		t.Fatal("expected scroller to stay attached")
	}
	assert.Equal(t, float32(50), e.scroll.Offset.Y, "scroll both keeps the offset")
}
