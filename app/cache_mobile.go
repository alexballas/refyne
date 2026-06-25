//go:build android || ios || mobile

package app

import fyne "github.com/alexballas/refyne/v2"

func rootAppCacheDir(a fyne.App) string {
	return a.(*fyneApp).storageRoot()
}
