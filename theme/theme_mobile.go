//go:build android || ios || mobile

package theme

import "github.com/alexballas/refyne/v2"

func setupSystemTheme(fallback fyne.Theme) fyne.Theme {
	return fallback
}
