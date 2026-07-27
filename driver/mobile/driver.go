// Package mobile provides desktop specific mobile functionality.
package mobile

import fyne "github.com/alexballas/refyne/v2"

// Driver represents the extended capabilities of a mobile driver
//
// Since: 2.4
type Driver interface {
	// GoBack asks the OS to go to the previous app / activity, where supported
	GoBack()
}

// URIOpener is implemented by mobile drivers that can be launched with a URI
// from another app, such as an Android share or open-with.
//
// Since: 2.8
type URIOpener interface {
	// SetOnOpenURI registers a callback invoked when the OS hands the app a URI,
	// together with the MIME type the sender declared, which may be empty. It is
	// called on the Fyne goroutine, and fires once for a URI that arrived before
	// registration.
	SetOnOpenURI(func(fyne.URI, string))
}

// BackgroundSession is implemented by mobile drivers that can keep the app
// process alive while it is not in the foreground.
//
// Android freezes a process shortly after it stops being visible, which stops
// every goroutine: connections the app is maintaining go unanswered until the
// peer drops them, and timers that would renew a session never fire. An app that
// has to keep talking to the network with the screen off - a player streaming to
// an external device, for instance - declares a session for as long as that work
// lasts.
//
// On Android this requires Android.BackgroundService in FyneApp.toml. Its Type
// must match the work being performed; the optional CPU and Wi-Fi locks are
// configured separately there. Drivers on platforms with no equivalent do not
// implement this interface, so a type assertion is the way to ask.
//
// Since: 2.8
type BackgroundSession interface {
	// StartBackgroundSession keeps the process running until
	// StopBackgroundSession, and shows the ongoing notification the OS requires
	// in exchange, labelled with the given title and text. Tapping it returns to
	// the app. Calling it again replaces the text of a running session rather
	// than starting a second one.
	//
	// The icon is the status bar entry. Android draws it as a stencil - it keeps
	// the alpha channel and throws the colours away - so pass a silhouette of the
	// app's mark in solid white on a transparent background, not the launcher
	// icon, which would come out as a featureless blob. A PNG of around 96x96 is
	// plenty. Pass nil to fall back to the launcher icon.
	//
	// It must be called while the app is in the foreground: Android refuses to
	// start the session otherwise. It reports no error when the OS declines,
	// since the app cannot do anything about it beyond carrying on.
	StartBackgroundSession(title, text string, icon fyne.Resource)

	// StopBackgroundSession releases the process and any configured power locks,
	// and takes the notification down. It is safe to call when no session is
	// running.
	StopBackgroundSession()

	// RequestNotificationPermission asks the user for permission to post
	// notifications, which Android 13 and later require before the session
	// notification can be shown. It returns immediately, does not wait for an
	// answer, and does nothing if permission was already given. Ask before the
	// first session rather than during it, so the dialog does not appear on top
	// of whatever the session was started for.
	RequestNotificationPermission()
}

// MulticastLocker is implemented by mobile drivers that can stop the platform
// filtering multicast traffic. Wi-Fi discards multicast and broadcast frames not
// addressed to the device unless a lock is held, which is what protocols like
// SSDP and mDNS depend on.
//
// On Android this requires Android.MulticastDiscovery in FyneApp.toml.
//
// Since: 2.8
type MulticastLocker interface {
	// AcquireMulticastLock lets multicast traffic through until the matching
	// ReleaseMulticastLock. Calls nest: the filter goes back up when the last
	// lock is released, so overlapping discovery runs are safe.
	AcquireMulticastLock()

	// ReleaseMulticastLock gives up one lock taken by AcquireMulticastLock.
	ReleaseMulticastLock()
}

// BatteryOptimization is implemented by mobile drivers that can ask the OS to
// exempt the app from power management heuristics.
//
// This is not what keeps a backgrounded app running - BackgroundSession is - but
// some vendors kill background apps well beyond the platform rules, and an
// exemption is what stops them.
//
// On Android the one-tap dialog requires Android.BatteryOptimizationExemption in
// FyneApp.toml; without it the request falls back to the system settings list.
//
// Since: 2.8
type BatteryOptimization interface {
	// IsIgnoringBatteryOptimizations reports whether the app is already exempt.
	// It reports true on platforms and versions with nothing to be exempt from,
	// so it can gate a prompt without further checks.
	IsIgnoringBatteryOptimizations() bool

	// RequestBatteryOptimizationExemption sends the user to the OS prompt for an
	// exemption. It returns immediately and does not report the answer; poll
	// IsIgnoringBatteryOptimizations afterwards if the result matters. The
	// decision belongs to the user, so ask at most once and let them decline.
	RequestBatteryOptimizationExemption()
}
