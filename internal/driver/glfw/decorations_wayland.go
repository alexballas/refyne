//go:build !wasm && wayland && (linux || freebsd || openbsd || netbsd)

package glfw

import (
	"os"
	"path/filepath"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/glfw"
)

// waylandAppID returns the app_id to advertise to the Wayland compositor. The
// compositor matches it against an installed <app_id>.desktop file to resolve
// the taskbar / title-bar icon and to group windows. Prefer the application's
// unique ID; fall back to the executable's base name so dev runs still get a
// stable, non-generic id; finally fall back to a constant.
func waylandAppID() string {
	if app := fyne.CurrentApp(); app != nil {
		if id := app.UniqueID(); id != "" {
			return id
		}
	}
	if exe, err := os.Executable(); err == nil {
		if base := filepath.Base(exe); base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return "refyne"
}

// applyWaylandWindowHints sets the pre-create window hints specific to Wayland
// (currently only the app_id). Must be called before glfw.CreateWindow.
func applyWaylandWindowHints() {
	glfw.WindowHintString(glfw.WaylandAppID, waylandAppID())
}
