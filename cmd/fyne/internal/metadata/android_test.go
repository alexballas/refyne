package metadata

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const androidToml = `Website = "https://apps.fyne.io"

[Details]
Name = "Fyne App"
ID = "io.fyne.fyne"

[Android]
ShareMimeTypes = ["video/*", "audio/*", "image/*"]

[Android.BackgroundService]
Type = "connectedDevice"
KeepCPUAwake = true
KeepWiFiAwake = true
`

func TestLoadAndroidMetadata(t *testing.T) {
	data, err := Load(strings.NewReader(androidToml))
	assert.Nil(t, err)
	assert.NotNil(t, data.Android)
	assert.Equal(t, []string{"video/*", "audio/*", "image/*"}, data.Android.ShareMimeTypes)
	assert.NotNil(t, data.Android.BackgroundService)
	assert.Equal(t, AndroidBackgroundServiceConnectedDevice, data.Android.BackgroundService.Type)
	assert.True(t, data.Android.BackgroundService.KeepCPUAwake)
	assert.True(t, data.Android.BackgroundService.KeepWiFiAwake)
}

func TestSaveAndroidMetadata(t *testing.T) {
	data, err := Load(strings.NewReader(androidToml))
	assert.Nil(t, err)

	w := &bytes.Buffer{}
	assert.Nil(t, Save(data, w))

	data2, err := Load(bytes.NewReader(w.Bytes()))
	assert.Nil(t, err)
	assert.NotNil(t, data2.Android)
	assert.Equal(t, data.Android.ShareMimeTypes, data2.Android.ShareMimeTypes)
	assert.Equal(t, data.Android.BackgroundService, data2.Android.BackgroundService)
}

// An app that does not opt in must not gain an empty [Android] section.
func TestSaveWithoutAndroidMetadata(t *testing.T) {
	data, err := Load(strings.NewReader("Website = \"https://apps.fyne.io\"\n"))
	assert.Nil(t, err)
	assert.Nil(t, data.Android)

	w := &bytes.Buffer{}
	assert.Nil(t, Save(data, w))
	assert.NotContains(t, w.String(), "[Android]")
}
