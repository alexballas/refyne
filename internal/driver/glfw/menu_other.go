//go:build !darwin || no_native_menus

package glfw

import fyne "github.com/alexballas/refyne/v2"

func setupNativeMenu(_ *window, _ *fyne.MainMenu) {
	// no-op
}
