package widget_test

import (
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/driver/mobile"
	"github.com/alexballas/refyne/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestEntrySelectTapAfterDoubleTapMobile(t *testing.T) {
	e, _ := setupSelection(t, false)
	e.MultiLine = true

	test.DoubleTap(e)
	assert.NotEmpty(t, e.SelectedText())

	time.Sleep(fyne.CurrentApp().Driver().DoubleTapDelay() + 50*time.Millisecond)
	pos := fyne.NewPos(1, 1)
	e.TouchDown(&mobile.TouchEvent{PointEvent: fyne.PointEvent{Position: pos}})
	e.TouchUp(&mobile.TouchEvent{PointEvent: fyne.PointEvent{Position: pos}})
	test.Tap(e)
	assert.Empty(t, e.SelectedText())
}
