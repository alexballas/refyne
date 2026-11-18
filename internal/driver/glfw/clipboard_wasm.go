//go:build wasm || test_web_driver

package glfw

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/fyne-io/glfw-js"
)

// Declare conformity with Clipboard interface
var _ fyne.Clipboard = clipboard{}

// systemClipboard reads and writes the system clipboard. Tests replace it so
// they neither depend on nor change the desktop's clipboard.
var systemClipboard = struct {
	get func() string
	set func(string)
}{glfw.GetClipboardString, glfw.SetClipboardString}

func NewClipboard() fyne.Clipboard {
	return clipboard{}
}

// clipboard represents the system clipboard
type clipboard struct{}

// Content returns the clipboard content
func (c clipboard) Content() string {
	return systemClipboard.get()
}

// SetContent sets the clipboard content
func (c clipboard) SetContent(content string) {
	systemClipboard.set(content)
}
