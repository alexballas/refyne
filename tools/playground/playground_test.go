package playground

import (
	"bytes"
	"encoding/base64"
	"image/color"
	"image/png"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/driver/software"
	"github.com/alexballas/refyne/v2/internal/test"
	"github.com/alexballas/refyne/v2/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	obj := canvas.NewRectangle(color.Black)
	obj.SetMinSize(fyne.NewSquareSize(10))

	img := software.Render(obj, test.DarkTheme(theme.DefaultTheme()))
	assert.NotNil(t, img)

	enc, err := encodeImage(img)
	require.NoError(t, err)

	data, err := base64.StdEncoding.DecodeString(enc)
	require.NoError(t, err)
	decoded, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, img.Bounds(), decoded.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			assert.Equal(t, color.RGBAModel.Convert(img.At(x, y)), color.RGBAModel.Convert(decoded.At(x, y)))
		}
	}
}
