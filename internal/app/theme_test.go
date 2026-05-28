package app_test

import (
	"testing"

	"github.com/alexballas/refyne/v2/internal/app"
	"github.com/alexballas/refyne/v2/test"
)

func TestApplySettings_BeforeContentSet(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("NoContent")
	defer w.Close()

	app.ApplySettings(a.Settings(), a)
}
