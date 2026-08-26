package widget

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
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
