//go:build android

package mobile

import (
	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/driver/mobile"
	"github.com/alexballas/refyne/v2/internal/driver/mobile/app"
)

// Declare conformity with URIOpener
var _ mobile.URIOpener = (*driver)(nil)

// SetOnOpenURI implements driver/mobile.URIOpener.
func (d *driver) SetOnOpenURI(fn func(fyne.URI, string)) {
	d.intents.setHandler(fn)
}

// startIntentDelivery connects the OS intent stream to the mailbox. It is called
// from Run once the function queue exists, so anything the mailbox releases from
// here on can be marshalled onto the Fyne goroutine rather than run inline.
func (d *driver) startIntentDelivery() {
	d.intents.setRun(func(fn func()) {
		d.DoFromGoroutine(fn, false)
	})

	app.SetIntentCallback(func(uri, mime string) {
		// nativeURI, not storage.ParseURI: content URIs need the Android
		// display-name lookup for Name() to return anything usable.
		d.intents.deliver(nativeURI(uri), mime)
	})
}
