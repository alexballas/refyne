package app

import (
	"testing"

	"github.com/alexballas/refyne/v2/internal/driver/mobile/event/key"
	"github.com/stretchr/testify/assert"
)

func TestKeyEditsText(t *testing.T) {
	tests := []struct {
		name    string
		event   key.Event
		handled bool
	}{
		{name: "printable", event: key.Event{Rune: 'A', Code: key.CodeA}, handled: true},
		{name: "unicode printable", event: key.Event{Rune: 'λ'}, handled: true},
		{name: "return", event: key.Event{Code: key.CodeReturnEnter}, handled: true},
		{name: "tab", event: key.Event{Code: key.CodeTab}, handled: true},
		{name: "backspace", event: key.Event{Code: key.CodeDeleteBackspace}},
		{name: "forward delete", event: key.Event{Code: key.CodeDeleteForward}},
		{name: "left", event: key.Event{Code: key.CodeLeftArrow}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.handled, keyEditsText(test.event))
		})
	}
}
