package app

import (
	"strconv"

	"github.com/alexballas/refyne/v2/internal/driver/mobile/event/key"
)

// keyEditsText reports whether Android would also apply the key to the hidden
// EditText. Deletion remains unconsumed so it can travel through the input
// connection and text watcher.
func keyEditsText(event key.Event) bool {
	switch event.Code {
	case key.CodeDeleteBackspace, key.CodeDeleteForward:
		return false
	case key.CodeReturnEnter, key.CodeTab:
		return true
	}
	return strconv.IsPrint(event.Rune)
}
