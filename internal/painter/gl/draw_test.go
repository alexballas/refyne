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

type clearRecorder struct {
	context
	color [4]float32
	mask  uint32
}

func (r *clearRecorder) Clear(mask uint32) {
	r.mask = mask
}

func (r *clearRecorder) ClearColor(red, green, blue, alpha float32) {
	r.color = [4]float32{red, green, blue, alpha}
}

func (r *clearRecorder) GetError() uint32 {
	return 0
}

type linkFailureContext struct {
	context
	linkCalls       int
	deletedPrograms int
	deletedShaders  int
}

func (c *linkFailureContext) AttachShader(Program, Shader) {}

func (c *linkFailureContext) CompileShader(Shader) {}

func (c *linkFailureContext) CreateProgram() Program {
	return noProgram
}

func (c *linkFailureContext) CreateShader(uint32) Shader {
	return noShader
}

func (c *linkFailureContext) DeleteProgram(Program) {
	c.deletedPrograms++
}

func (c *linkFailureContext) DeleteShader(Shader) {
	c.deletedShaders++
}

func (c *linkFailureContext) GetError() uint32 {
	return 0
}

func (c *linkFailureContext) GetProgrami(Program, uint32) int {
	return int(glFalse) // linking always fails
}

func (c *linkFailureContext) GetProgramInfoLog(Program) string {
	return ""
}

func (c *linkFailureContext) GetShaderi(Shader, uint32) int {
	return 1
}

func (c *linkFailureContext) GetShaderInfoLog(Shader) string {
	return ""
}

func (c *linkFailureContext) LinkProgram(Program) {
	c.linkCalls++
}

func (c *linkFailureContext) ShaderSource(Shader, string) {}

type compileFailureContext struct {
	context
	deletedShaders int
}

func (c *compileFailureContext) CompileShader(Shader) {}

func (c *compileFailureContext) CreateShader(uint32) Shader {
	return noShader
}

func (c *compileFailureContext) DeleteShader(Shader) {
	c.deletedShaders++
}

func (c *compileFailureContext) GetError() uint32 {
	return 0
}

func (c *compileFailureContext) GetShaderi(Shader, uint32) int {
	return int(glFalse)
}

func (c *compileFailureContext) GetShaderInfoLog(Shader) string {
	return ""
}

func (c *compileFailureContext) ShaderSource(Shader, string) {}

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

func arbitraryPolygonShaderName(t *testing.T) string {
	t.Helper()

	for _, name := range []string{"arbitrary_polygon", "arbitrary_polygon_es"} {
		vertex, fragment := shaderSourceNamed(name)
		if vertex != nil && fragment != nil {
			return name
		}
	}
	t.Fatal("arbitrary polygon shader not embedded for this build")
	return ""
}

func TestOptionalProgramLinkFailure(t *testing.T) {
	ctx := &linkFailureContext{}
	p := &painter{ctx: ctx}

	state := p.initOptionalProgram(arbitraryPolygonShaderName(t), 16, nil, nil)
	assert.True(t, state.unsupported, "an optional shader link failure must disable only that program")
	assert.Equal(t, 1, ctx.linkCalls, "test must exercise the linker failure path")
	assert.Equal(t, 1, ctx.deletedPrograms)
	assert.Equal(t, 2, ctx.deletedShaders)
}

func TestOptionalProgramMissingSourcePanics(t *testing.T) {
	p := &painter{ctx: &linkFailureContext{}}

	assert.PanicsWithValue(t, "shader not found: missing", func() {
		p.initOptionalProgram("missing", 16, nil, nil)
	})
}

func TestProgramCompileFailureCleansShader(t *testing.T) {
	ctx := &compileFailureContext{}
	p := &painter{ctx: ctx}

	_, err := p.createProgramFromSource("broken", []byte("vertex"), []byte("fragment"))
	assert.ErrorContains(t, err, `OpenGL program "broken": failed to compile OpenGL shader`)
	assert.Equal(t, 1, ctx.deletedShaders)
}

func TestProgramLinkFailureIncludesNameAndCleansResources(t *testing.T) {
	ctx := &linkFailureContext{}
	p := &painter{ctx: ctx}

	_, err := p.createProgramFromSource("arbitrary_polygon_es", []byte("vertex"), []byte("fragment"))
	assert.ErrorContains(t, err, `failed to link OpenGL program "arbitrary_polygon_es"`)
	assert.Equal(t, 1, ctx.linkCalls)
	assert.Equal(t, 1, ctx.deletedPrograms)
	assert.Equal(t, 2, ctx.deletedShaders)
}

func TestPainter_ClearTransparentBackgroundUsesPremultipliedAlpha(t *testing.T) {
	rec := &clearRecorder{}
	p := &painter{ctx: rec, transparentBackground: true}

	p.Clear()

	assert.Equal(t, [4]float32{}, rec.color)
	assert.Equal(t, uint32(bitColorBuffer|bitDepthBuffer), rec.mask)
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

func TestKernelToRGBAKeepsFullWeight(t *testing.T) {
	// Weights that do not add up to 255 shift the brightness of anything drawn
	// under a blur, twice over because the blur runs in two passes.
	for _, radius := range []float32{1, 5, 10, 25, maxKernelRadius} {
		data := kernelToRGBA(createKernel(radius))

		total := 0
		for i := 0; i < len(data); i += 4 {
			total += int(data[i])
		}

		assert.Equal(t, 255, total, "kernel weights for radius %v must total 255", radius)
	}
}
