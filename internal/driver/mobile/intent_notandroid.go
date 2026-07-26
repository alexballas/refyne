//go:build !android

package mobile

// startIntentDelivery does nothing away from Android, which is the only platform
// where the OS hands a running app a URI from another one. Drivers built for the
// other platforms deliberately do not satisfy driver/mobile.URIOpener.
func (d *driver) startIntentDelivery() {}
