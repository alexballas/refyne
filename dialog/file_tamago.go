//go:build tamago || noos

package dialog

import fyne "github.com/alexballas/refyne/v2"

func getFavoriteLocations() (map[string]fyne.ListableURI, error) {
	return map[string]fyne.ListableURI{}, nil
}
