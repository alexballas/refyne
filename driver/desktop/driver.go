// Package desktop provides desktop specific driver functionality.
package desktop

import fyne "github.com/alexballas/refyne/v2"

// Driver represents the extended capabilities of a desktop driver
type Driver interface {
	// CreateSplashWindow creates a new borderless window that is centered on screen.
	CreateSplashWindow() fyne.Window

	// CurrentKeyModifiers returns the set of key modifiers that are currently active.
	//
	// Since: 2.4
	CurrentKeyModifiers() fyne.KeyModifier

	// HasSecondaryDisplay returns true if more than one non-mirrored screen is available.
	// This is commonly used alongside [Window.RequestFullScreenSecondary].
	//
	// Since: 2.8
	HasSecondaryDisplay() bool
}
