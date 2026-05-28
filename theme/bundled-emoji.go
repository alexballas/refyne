//go:build !no_emoji

package theme

import (
	_ "embed"

	fyne "github.com/alexballas/refyne/v2"
)

//go:embed font/EmojiOneColor.otf
var emojiFontData []byte

var emoji = &fyne.StaticResource{
	StaticName:    "EmojiOneColor.otf",
	StaticContent: emojiFontData,
}
