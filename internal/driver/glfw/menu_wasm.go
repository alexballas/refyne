//go:build wasm || test_web_driver

package glfw

import fyne "github.com/alexballas/refyne/v2"

func addMissingQuitForMainMenu(menus *fyne.MainMenu, w *window) *fyne.MainMenu {
	// no-op for a web browser
	return menus
}
