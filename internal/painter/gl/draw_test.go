//go:build !windows || !ci

package gl

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
)

// blendRecorder records the last blend call. It embeds the context interface so
// it satisfies every method; blendFunc only ever calls the two blend methods.
type blendRecorder struct {
	context
	separate bool
	args     [4]uint32
}

func (r *blendRecorder) BlendFunc(srcFactor, destFactor uint32) {
	r.separate = false
	r.args = [4]uint32{srcFactor, destFactor, 0, 0}
}

func (r *blendRecorder) BlendFuncSeparate(srcRGB, destRGB, srcAlpha, destAlpha uint32) {
	r.separate = true
	r.args = [4]uint32{srcRGB, destRGB, srcAlpha, destAlpha}
}

func TestPainter_blendFuncPreservesAlpha(t *testing.T) {
	rec := &blendRecorder{}
	p := &painter{ctx: rec}

	// Default: plain colour+alpha blending, identical to before.
	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	assert.False(t, rec.separate)
	assert.Equal(t, [4]uint32{srcAlpha, oneMinusSrcAlpha, 0, 0}, rec.args)

	// Alpha-capable surface: colour factors unchanged, alpha saturates to opaque.
	p.preserveAlpha = true
	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	assert.True(t, rec.separate)
	assert.Equal(t, [4]uint32{srcAlpha, oneMinusSrcAlpha, one, oneMinusSrcAlpha}, rec.args)

	// A transparent (rounded-corner) clear must also preserve alpha.
	p.preserveAlpha = false
	p.transparentBackground = true
	p.blendFunc(one, oneMinusSrcAlpha)
	assert.True(t, rec.separate)
	assert.Equal(t, [4]uint32{one, oneMinusSrcAlpha, one, oneMinusSrcAlpha}, rec.args)
}

func TestGetFragmentColor(t *testing.T) {
	var c color.Color

	r, g, b, a := getFragmentColor(c)
	assert.Equal(t, float32(0), r)
	assert.Equal(t, float32(0), g)
	assert.Equal(t, float32(0), b)
	assert.Equal(t, float32(0), a)

	c = color.NRGBA{R: 0x0, G: 0x66, B: 0x99, A: 0xff}
	r, g, b, a = getFragmentColor(c)
	assert.Equal(t, float32(0), r)
	assert.Equal(t, float32(0.4), g)
	assert.Equal(t, float32(0.6), b)
	assert.Equal(t, float32(1), a)

	c = color.NRGBA{R: 0x0, G: 0x66, B: 0x99, A: 0x99}
	r, g, b, a = getFragmentColor(c)
	assert.Equal(t, float32(0), r)
	assert.Equal(t, float32(0.3999898), g)
	assert.Equal(t, float32(0.59998477), b)
	assert.Equal(t, float32(0.6), a)
}

func TestInnerRect_Stretch(t *testing.T) {
	pos := fyne.NewPos(10, 10)
	size := fyne.NewSize(40, 40)

	innerSize, innerPos := rectInnerCoords(size, pos, canvas.ImageFillStretch, 0.0)

	assert.Equal(t, size, innerSize)
	assert.Equal(t, pos, innerPos)
}

func TestInnerRect_StretchIgnoreRatio(t *testing.T) {
	pos := fyne.NewPos(10, 10)
	size := fyne.NewSize(40, 40)

	innerSize, innerPos := rectInnerCoords(size, pos, canvas.ImageFillStretch, 2.0)

	assert.Equal(t, size, innerSize)
	assert.Equal(t, pos, innerPos)
}

func TestInnerRect_ContainScale(t *testing.T) {
	pos := fyne.NewPos(10, 10)
	size := fyne.NewSize(40, 40)

	innerSize, innerPos := rectInnerCoords(size, pos, canvas.ImageFillContain, 1.0)

	assert.Equal(t, size, innerSize)
	assert.Equal(t, pos, innerPos)
}

func TestInnerRect_ContainPillarbox(t *testing.T) {
	pos := fyne.NewPos(10, 10)
	size := fyne.NewSize(40, 40)

	innerSize, innerPos := rectInnerCoords(size, pos, canvas.ImageFillContain, 0.5)

	assert.Equal(t, fyne.NewSize(20, 40), innerSize)
	assert.Equal(t, fyne.NewPos(20, 10), innerPos)
}

func TestInnerRect_Original(t *testing.T) {
	// TODO add check for minsize somehow?
	pos := fyne.NewPos(10, 10)
	size := fyne.NewSize(40, 40)

	innerSize1, innerPos1 := rectInnerCoords(size, pos, canvas.ImageFillOriginal, 0.5)
	innerSize2, innerPos2 := rectInnerCoords(size, pos, canvas.ImageFillContain, 0.5)

	assert.Equal(t, innerSize2, innerSize1)
	assert.Equal(t, innerPos2, innerPos1)
}
