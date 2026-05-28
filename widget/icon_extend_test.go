package widget

import (
	"testing"

	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/theme"
	"github.com/stretchr/testify/assert"
)

type extendedIcon struct {
	Icon
}

func newExtendedIcon() *extendedIcon {
	icon := &extendedIcon{}
	icon.ExtendBaseWidget(icon)
	return icon
}

func TestIcon_Extended_SetResource(t *testing.T) {
	icon := newExtendedIcon()
	icon.SetResource(theme.ComputerIcon())

	objs := cache.Renderer(icon).Objects()
	assert.Len(t, objs, 1)
	assert.Equal(t, theme.ComputerIcon(), objs[0].(*canvas.Image).Resource)
}
