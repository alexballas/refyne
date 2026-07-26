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
