//go:build wasm

package app

import (
	"syscall/js"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/theme"
)

// DefaultVariant returns the systems default fyne.ThemeVariant.
// Normally, you should not need this. It is extracted out of the root app package to give the
// settings app access to it.
func DefaultVariant() fyne.ThemeVariant {
	matches := js.Global().Call("matchMedia", "(prefers-color-scheme: dark)")
	if matches.Truthy() {
		if matches.Get("matches").Bool() {
			return theme.VariantDark
		}
		return theme.VariantLight
	}
	return theme.VariantDark
}
