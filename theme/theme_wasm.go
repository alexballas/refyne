//go:build wasm || test_web_driver

package theme

import fyne "github.com/alexballas/refyne/v2"

func setupSystemTheme(fallback fyne.Theme) fyne.Theme {
	return fallback
}
