package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_RootURI(t *testing.T) {
	if runtime.GOOS == "windows" {
		home := t.TempDir()
		t.Setenv("USERPROFILE", home)
		require.NoError(t, os.MkdirAll(filepath.Join(home, "AppData", "Roaming", "fyne"), 0o755))
	}

	id := "io.fyne.test"
	a := &fyneApp{uniqueID: id}
	d := makeStoreDocs(id, &store{a: a})

	w, err := d.Create("test")
	assert.NoError(t, err)
	err = w.Close()
	assert.NoError(t, err)
	err = d.Remove("test")
	assert.NoError(t, err)
}
