package widget_test

import (
	"net/url"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/alexballas/refyne/v2/widget"
	"github.com/stretchr/testify/assert"
)

func TestButton_Accessibility(t *testing.T) {
	b := widget.NewButton("Click me", func() {})
	assert.Equal(t, "Click me", b.AccessibilityLabel())
	assert.Equal(t, fyne.AccessibleRoleButton, b.AccessibilityRole())

	// Without text the icon name is used as the label.
	icon := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {})
	assert.Equal(t, theme.CancelIcon().Name(), icon.AccessibilityLabel())
	assert.Equal(t, fyne.AccessibleRoleButton, icon.AccessibilityRole())
}

func TestLabel_Accessibility(t *testing.T) {
	l := widget.NewLabel("Hello")
	assert.Equal(t, "Hello", l.AccessibilityLabel())
	assert.Equal(t, fyne.AccessibleRoleText, l.AccessibilityRole())
}

func TestHyperlink_Accessibility(t *testing.T) {
	u, _ := url.Parse("https://example.com")
	hl := widget.NewHyperlink("Example", u)
	assert.Equal(t, "Example", hl.AccessibilityLabel())
	assert.Equal(t, fyne.AccessibleRoleLink, hl.AccessibilityRole())
}
