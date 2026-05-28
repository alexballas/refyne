//go:build !ios && !android && !wasm && !js

package dialog

import (
	"github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/storage"
)

func getFavoriteLocation(homeURI fyne.URI, name string) (fyne.URI, error) {
	return storage.Child(homeURI, name)
}
