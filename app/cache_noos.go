//go:build noos || tinygo

package app

import (
	"path/filepath"

	fyne "github.com/alexballas/refyne/v2"
)

func rootAppCacheDir(a fyne.App) string {
	return filepath.Join(rootCacheDir(), a.UniqueID())
}
