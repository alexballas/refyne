//go:build flatpak && !windows && !android && !ios && !wasm && !js

package dialog

import (
	"testing"

	"github.com/alexballas/refyne/v2/driver"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/widget"
	"github.com/stretchr/testify/assert"
)

func TestFormatFilterName(t *testing.T) {
	actual := formatFilterName([]string{"1", "2", "3"}, 4)
	assert.Equal(t, "1, 2, 3", actual)

	actual = formatFilterName([]string{"1", "2", "3"}, 3)
	assert.Equal(t, "1, 2, 3", actual)

	actual = formatFilterName([]string{"1", "2", "3"}, 2)
	assert.Equal(t, "1, 2…", actual)
}

func TestFlatpakDialogWithoutNativeWindow(t *testing.T) {
	window := test.NewTempWindow(t, widget.NewLabel("Content"))
	for _, tc := range []struct {
		name     string
		dialog   *FileDialog
		override func(*FileDialog) bool
	}{
		{"open", NewFileOpen(nil, window), fileOpenOSOverride},
		{"folder", NewFolderOpen(nil, window), fileOpenOSOverride},
		{"save", NewFileSave(nil, window), fileSaveOSOverride},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, tc.override(tc.dialog))
			tc.dialog.Show()
			assert.NotNil(t, tc.dialog.dialog, "expected the built-in dialog")
			tc.dialog.Hide()
		})
	}
}

type portalNativeWindow struct{ context any }

func (w portalNativeWindow) RunNative(callback func(any)) { callback(w.context) }

func TestWindowHandleForPortal(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context any
		want    string
	}{
		{"X11", driver.X11WindowContext{WindowHandle: 0x123abc}, "x11:123abc"},
		{"Wayland", driver.WaylandWindowContext{WaylandSurface: 123}, ""},
		{"unknown", driver.UnknownContext{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, windowHandleForPortal(portalNativeWindow{context: tc.context}))
		})
	}
}
