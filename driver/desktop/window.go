package desktop

// Window describes desktop-specific window features.
//
// Since: 2.8
type Window interface {
	// RequestFullScreenSecondary asks for a window to fullscreen on a non-primary monitor.
	// When there is only one screen it will fullscreen on the primary monitor.
	RequestFullScreenSecondary()

	// RequestAlwaysOnTop asks for the window to remain above other windows; call it before Show.
	// This is subject to support on the current system, and the window manager may decide that
	// other windows or panels remain above this one.
	RequestAlwaysOnTop()

	// RequestPosition asks the operating system to position this window at the x, y coordinate
	// of the native system. These values may be scaled and so are not necessarily pixel values.
	// This request may be ignored, for example on Linux Wayland.
	// There is no contract for what these values represent, and on multiple monitors they may be
	// greater than the coordinates of the current monitor.
	RequestPosition(x, y int)
}
