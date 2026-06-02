// Package gl provides a full Fyne render implementation using system OpenGL libraries.
package gl

import (
	"fmt"
	"image"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal"
	"github.com/alexballas/refyne/v2/internal/driver"
	"github.com/alexballas/refyne/v2/theme"
)

// Painter defines the functionality of our OpenGL based renderer
type Painter interface {
	// Init tell a new painter to initialise, usually called after a context is available
	Init()
	// Capture requests that the specified canvas be drawn to an in-memory image
	Capture(fyne.Canvas) image.Image
	// Clear tells our painter to prepare a fresh paint
	Clear()
	// SetTransparentBackground makes Clear use a transparent black background.
	// Wayland presents ARGB buffers with premultiplied alpha, so RGB must also be
	// zero wherever alpha is zero.
	SetTransparentBackground(bool)
	// SetPreserveFramebufferAlpha keeps the framebuffer alpha channel opaque
	// where content is drawn. Required for windows presented on an
	// alpha-capable (ARGB) surface — e.g. Wayland client-side decorations —
	// so that straight-alpha blending of semi-transparent content does not
	// erode the surface alpha and leave those areas see-through.
	SetPreserveFramebufferAlpha(bool)
	// Free is used to indicate that a certain canvas object is no longer needed
	Free(fyne.CanvasObject)
	// Paint a single fyne.CanvasObject but not its children.
	Paint(fyne.CanvasObject, fyne.Position, fyne.Size, *internal.ClipItem)
	// SetFrameBufferScale tells us when we have more than 1 framebuffer pixel for each output pixel
	SetFrameBufferScale(float32)
	// SetOutputSize is used to change the resolution of our output viewport
	SetOutputSize(int, int)
	// StartClipping tells us that the following paint actions should be clipped to the specified area.
	StartClipping(fyne.Position, fyne.Size)
	// StopClipping stops clipping paint actions.
	StopClipping()
}

// NewPainter creates a new GL based renderer for the provided canvas.
// If it is a master painter it will also initialise OpenGL
func NewPainter(c fyne.Canvas, ctx driver.WithContext) Painter {
	p := &painter{canvas: c, contextProvider: ctx}
	p.SetFrameBufferScale(1.0)
	return p
}

type painter struct {
	canvas                fyne.Canvas
	ctx                   context
	contextProvider       driver.WithContext
	program               ProgramState
	lineProgram           ProgramState
	rectangleProgram      ProgramState
	roundRectangleProgram ProgramState
	polygonProgram        ProgramState
	arcProgram            ProgramState
	bezierCurveProgram    ProgramState
	texScale              float32
	pixScale              float32 // pre-calculate scale*texScale for each draw
	transparentBackground bool
	preserveAlpha         bool
}

type ProgramState struct {
	ref        Program
	buff       Buffer
	uniforms   map[string]*UniformState
	attributes map[string]Attribute
}

type UniformState struct {
	ref  Uniform
	prev [4]float32
}

func (p *painter) SetUniform1f(pState ProgramState, name string, v float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v {
		return
	}
	u.prev[0] = v
	p.ctx.Uniform1f(u.ref, v)
}

func (p *painter) SetUniform2f(pState ProgramState, name string, v0, v1 float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v0 && u.prev[1] == v1 {
		return
	}
	u.prev[0] = v0
	u.prev[1] = v1
	p.ctx.Uniform2f(u.ref, v0, v1)
}

func (p *painter) SetUniform4f(pState ProgramState, name string, v0, v1, v2, v3 float32) {
	u := pState.uniforms[name]
	if u.prev[0] == v0 && u.prev[1] == v1 && u.prev[2] == v2 && u.prev[3] == v3 {
		return
	}
	u.prev[0] = v0
	u.prev[1] = v1
	u.prev[2] = v2
	u.prev[3] = v3
	p.ctx.Uniform4f(u.ref, v0, v1, v2, v3)
}

func (p *painter) UpdateVertexArray(pState ProgramState, name string, size, stride, offset int) {
	a := pState.attributes[name]

	p.ctx.VertexAttribPointerWithOffset(a, size, float, false, stride*floatSize, offset*floatSize)
	p.logError()
}

// Declare conformity to Painter interface
var _ Painter = (*painter)(nil)

func (p *painter) SetTransparentBackground(transparent bool) {
	p.transparentBackground = transparent
}

func (p *painter) SetPreserveFramebufferAlpha(preserve bool) {
	p.preserveAlpha = preserve
}

// blendFunc applies the requested colour blend factors. When rendering to an
// alpha-capable surface (transparent background, or an ARGB Wayland surface),
// the alpha channel is blended separately with (ONE, ONE_MINUS_SRC_ALPHA) so it
// accumulates toward opaque: drawing opaque content yields alpha 1 and
// semi-transparent content over an opaque body keeps alpha 1, while areas with
// nothing drawn (the rounded corners) stay fully transparent. The colour
// channels are unaffected, so visuals are identical to the plain BlendFunc.
func (p *painter) blendFunc(srcFactor, dstFactor uint32) {
	if p.preserveAlpha || p.transparentBackground {
		p.ctx.BlendFuncSeparate(srcFactor, dstFactor, one, oneMinusSrcAlpha)
		return
	}
	p.ctx.BlendFunc(srcFactor, dstFactor)
}

func (p *painter) Clear() {
	if p.transparentBackground {
		p.ctx.ClearColor(0, 0, 0, 0)
		p.ctx.Clear(bitColorBuffer | bitDepthBuffer)
		p.logError()
		return
	}

	r, g, b, a := theme.Color(theme.ColorNameBackground).RGBA()
	p.ctx.ClearColor(float32(r)/max16bit, float32(g)/max16bit, float32(b)/max16bit, float32(a)/max16bit)
	p.ctx.Clear(bitColorBuffer | bitDepthBuffer)
	p.logError()
}

func (p *painter) Free(obj fyne.CanvasObject) {
	p.freeTexture(obj)
}

func (p *painter) Paint(obj fyne.CanvasObject, pos fyne.Position, frame fyne.Size, clip *internal.ClipItem) {
	if obj.Visible() {
		p.drawObject(obj, pos, frame, clip)
	}
}

func (p *painter) SetFrameBufferScale(scale float32) {
	p.texScale = scale
	p.pixScale = p.canvas.Scale() * p.texScale
}

func (p *painter) SetOutputSize(width, height int) {
	p.ctx.Viewport(0, 0, width, height)
	p.logError()
}

func (p *painter) StartClipping(pos fyne.Position, size fyne.Size) {
	x := p.textureScale(pos.X)
	y := p.textureScale(p.canvas.Size().Height - pos.Y - size.Height)
	w := p.textureScale(size.Width)
	h := p.textureScale(size.Height)
	p.ctx.Scissor(int32(x), int32(y), int32(w), int32(h))
	p.ctx.Enable(scissorTest)
	p.logError()
}

func (p *painter) StopClipping() {
	p.ctx.Disable(scissorTest)
	p.logError()
}

func (p *painter) compileShader(source string, shaderType uint32) (Shader, error) {
	shader := p.ctx.CreateShader(shaderType)

	p.ctx.ShaderSource(shader, source)
	p.logError()
	p.ctx.CompileShader(shader)
	p.logError()

	info := p.ctx.GetShaderInfoLog(shader)
	if p.ctx.GetShaderi(shader, compileStatus) == glFalse {
		return noShader, fmt.Errorf("failed to compile OpenGL shader:\n%s\n>>> SHADER SOURCE\n%s\n<<< SHADER SOURCE", info, source)
	}

	// The info is probably a null terminated string.
	// An empty info has been seen as "\x00" or "\x00\x00".
	if len(info) > 0 && info != "\x00" && info != "\x00\x00" {
		fmt.Printf("OpenGL shader compilation output:\n%s\n>>> SHADER SOURCE\n%s\n<<< SHADER SOURCE\n", info, source)
	}

	return shader, nil
}

func (p *painter) createProgram(shaderFilename string) Program {
	// Why a switch over a filename?
	// Because this allows for a minimal change, once we reach Go 1.16 and use go:embed instead of
	// fyne bundle.
	vertexSrc, fragmentSrc := shaderSourceNamed(shaderFilename)
	if vertexSrc == nil {
		panic("shader not found: " + shaderFilename)
	}

	vertShader, err := p.compileShader(string(vertexSrc), vertexShader)
	if err != nil {
		panic(err)
	}
	fragShader, err := p.compileShader(string(fragmentSrc), fragmentShader)
	if err != nil {
		panic(err)
	}

	prog := p.ctx.CreateProgram()
	p.ctx.AttachShader(prog, vertShader)
	p.ctx.AttachShader(prog, fragShader)
	p.ctx.LinkProgram(prog)

	info := p.ctx.GetProgramInfoLog(prog)
	if p.ctx.GetProgrami(prog, linkStatus) == glFalse {
		panic(fmt.Errorf("failed to link OpenGL program:\n%s", info))
	}

	// The info is probably a null terminated string.
	// An empty info has been seen as "\x00" or "\x00\x00".
	if len(info) > 0 && info != "\x00" && info != "\x00\x00" {
		fmt.Printf("OpenGL program linking output:\n%s\n", info)
	}

	if glErr := p.ctx.GetError(); glErr != 0 {
		panic(fmt.Sprintf("failed to link OpenGL program; error code: %x", glErr))
	}

	p.ctx.UseProgram(prog)

	return prog
}

func (p *painter) logError() {
	logGLError(p.ctx.GetError)
}
