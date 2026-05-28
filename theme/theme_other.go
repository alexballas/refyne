//go:build !hints

package theme

import (
	"image/color"

	fyne "github.com/alexballas/refyne/v2"
)

var (
	fallbackColor = color.Transparent
	fallbackIcon  = &fyne.StaticResource{}
)
