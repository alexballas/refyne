package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/test"
	"github.com/alexballas/refyne/v2/theme"
)

func TestForceVariantTheme_Color(t *testing.T) {
	test.NewApp()
	dark := &forcedVariant{Theme: theme.DefaultTheme(), variant: theme.VariantDark}
	light := &forcedVariant{Theme: theme.DefaultTheme(), variant: theme.VariantLight}
	unspecified := fyne.ThemeVariant(99)

	assert.Equal(t, theme.DefaultTheme().Color(theme.ColorNameForeground, theme.VariantDark), dark.Color(theme.ColorNameForeground, unspecified))
	assert.Equal(t, theme.DefaultTheme().Color(theme.ColorNameForeground, theme.VariantLight), light.Color(theme.ColorNameForeground, unspecified))
}
