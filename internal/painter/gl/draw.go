package gl

import (
	"image/color"
	"math"
	"sort"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/canvas"
	"github.com/alexballas/refyne/v2/internal"
	"github.com/alexballas/refyne/v2/internal/cache"
	paint "github.com/alexballas/refyne/v2/internal/painter"
)

const edgeSoftness = 1.0

func (p *painter) createBuffer(size int) Buffer {
	vbo := p.ctx.CreateBuffer()
	p.logError()
	p.ctx.BindBuffer(arrayBuffer, vbo)
	p.logError()
	p.ctx.BufferData(arrayBuffer, make([]float32, size), staticDraw)
	p.logError()
	return vbo
}

func (p *painter) drawBlur(b *canvas.Blur, pos fyne.Position, frame fyne.Size) {
	if b.Radius == 0 {
		return
	}
	radius := b.Radius * p.pixScale

	x := roundToPixel(pos.X*p.pixScale, 1.0)
	y := roundToPixel(pos.Y*p.pixScale, 1.0)
	bw := int(roundToPixel(b.Size().Width*p.pixScale, 1.0))
	bh := int(roundToPixel(b.Size().Height*p.pixScale, 1.0))
	if bw <= 0 || bh <= 0 {
		return
	}

	if !p.blurSnapTexValid || p.blurSnapW != bw || p.blurSnapH != bh {
		if p.blurSnapTexValid {
			p.ctx.DeleteTexture(p.blurSnapTex)
		}
		p.blurSnapTex = p.newTexture(canvas.ImageScaleFastest)
		p.ctx.TexImage2D(texture2D, 0, bw, bh, colorFormatRGBA, unsignedByte, nil)
		p.blurSnapTexValid = true
		p.blurSnapW = bw
		p.blurSnapH = bh
	}

	fbY := p.fbHeight - int(y) - bh
	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, p.blurSnapTex)
	p.ctx.CopyTexSubImage2D(texture2D, 0, 0, 0, int(x), fbY, bw, bh)
	p.logError()

	points, _ := p.rectCoords(b.Size(), pos, frame, canvas.ImageFillStretch, 1.0, 0)
	points[4], points[9] = points[9], points[4]
	points[14], points[19] = points[19], points[14]

	uniforms := &p.uniforms.blur
	p.ctx.UseProgram(p.blurProgram.ref)
	p.updateBuffer(p.blurProgram.buff, points)
	p.UpdateVertexArray(uniforms.vert, 3, 5, 0)
	p.UpdateVertexArray(uniforms.vertTexCoord, 2, 5, 3)

	p.blendFunc(one, oneMinusSrcAlpha)
	p.logError()

	p.SetUniform1f(uniforms.radius, radius)
	p.SetUniform2f(uniforms.size, float32(bw), float32(bh))

	values, ok := cache.GetBlurKernel(radius)
	if !ok {
		values = createKernel(radius)
		cache.SetBlurKernel(radius, values)
	}
	p.SetUniform1fv(p.getUniformLocation(p.blurProgram, "kernel"), values)

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawCircle(circle *canvas.Circle, pos fyne.Position, frame fyne.Size) {
	radius := paint.GetMaximumRadius(circle.Size())
	program := p.roundRectangleProgram
	uniforms := &p.uniforms.roundRect

	// Vertex: BEG
	bounds, points := p.vecSquareCoordsWithShadow(pos, circle, frame, circle.Shadow)
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	strokeWidthScaled := roundToPixel(circle.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidthHalf, strokeWidthScaled*0.5)

	rectSizeWidthScaled := x2Scaled - x1Scaled - strokeWidthScaled
	rectSizeHeightScaled := y2Scaled - y1Scaled - strokeWidthScaled
	p.SetUniform2f(uniforms.rectSizeHalf, rectSizeWidthScaled*0.5, rectSizeHeightScaled*0.5)

	radiusScaled := roundToPixel(radius*p.pixScale, 1.0)
	p.SetUniform4f(uniforms.radius, radiusScaled, radiusScaled, radiusScaled, radiusScaled)

	r, g, b, a := getFragmentColor(circle.FillColor)
	p.SetUniform4f(uniforms.fillColor, r, g, b, a)

	strokeColor := circle.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	var addShadow float32
	if paint.IsShadowVisible(circle.Shadow) {
		r, g, b, a = getFragmentColor(circle.Shadow.Color)
		p.SetUniform4f(uniforms.shadowColor, r, g, b, a)
		p.SetUniform2f(uniforms.shadowOffset, roundToPixel(circle.Shadow.Offset.X*p.pixScale, 1.0), roundToPixel(circle.Shadow.Offset.Y*p.pixScale, 1.0))
		p.SetUniform1f(uniforms.shadowBlurRadius, roundToPixel(circle.Shadow.BlurRadius*p.pixScale, 1.0))
		p.SetUniform1f(uniforms.shadowSpread, roundToPixel(circle.Shadow.Spread*p.pixScale, 1.0))
		p.SetUniform1f(uniforms.shadowType, float32(circle.Shadow.Variant))
		addShadow = 1.0
	}
	p.SetUniform1f(uniforms.addShadow, addShadow)
	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawGradient(o fyne.CanvasObject, texCreator func(fyne.CanvasObject) Texture, pos fyne.Position, frame fyne.Size) {
	p.drawTextureWithDetails(o, texCreator, pos, o.Size(), frame, canvas.ImageFillStretch, 1.0, 0)
}

func (p *painter) drawImage(img *canvas.Image, pos fyne.Position, frame fyne.Size) {
	p.drawTextureWithDetails(img, p.newGlImageTexture, pos, img.Size(), frame, img.FillMode, float32(img.Alpha()), 0)
}

func (p *painter) drawLine(line *canvas.Line, pos fyne.Position, frame fyne.Size) {
	if line.StrokeColor == color.Transparent || line.StrokeColor == nil || line.StrokeWidth == 0 {
		return
	}
	points, halfWidth, feather := p.lineCoords(pos, line.Position1, line.Position2, line.StrokeWidth, 0.5, frame)
	uniforms := &p.uniforms.line
	p.ctx.UseProgram(p.lineProgram.ref)
	p.updateBuffer(p.lineProgram.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()

	r, g, b, a := getFragmentColor(line.StrokeColor)
	p.SetUniform4f(uniforms.color, r, g, b, a)

	p.SetUniform1f(uniforms.lineWidth, halfWidth)

	p.SetUniform1f(uniforms.feather, feather)

	p.ctx.DrawArrays(triangles, 0, 6)
	p.logError()
}

func (p *painter) drawBezierCurve(bezierCurve *canvas.BezierCurve, pos fyne.Position, frame fyne.Size) {
	if bezierCurve.StrokeColor == color.Transparent || bezierCurve.StrokeColor == nil || bezierCurve.StrokeWidth == 0 {
		return
	}

	// Vertex: BEG
	bounds, points := p.vecRectCoords(pos, bezierCurve, frame, 0.0)
	program := p.bezierCurveProgram
	uniforms := &p.uniforms.bezier
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	// ensure stroke width is not larger than the size of the object
	strokeWidth := fyne.Min(bezierCurve.StrokeWidth, fyne.Min(bezierCurve.Size().Width, bezierCurve.Size().Height))
	if strokeWidth < 1 {
		strokeWidth = 1
	}
	p1, p2, cp := paint.NormalizeBezierCurvePoints(bezierCurve.StartPoint, bezierCurve.EndPoint, bezierCurve.ControlPoints, bezierCurve.Size(), strokeWidth/2.0)

	p1XScaled, p1YScaled := roundToPixel(p1.X*p.pixScale, 1.0), roundToPixel(p1.Y*p.pixScale, 1.0)
	p.SetUniform2f(uniforms.startPoint, p1XScaled, p1YScaled)

	p2XScaled, p2YScaled := roundToPixel(p2.X*p.pixScale, 1.0), roundToPixel(p2.Y*p.pixScale, 1.0)
	p.SetUniform2f(uniforms.endPoint, p2XScaled, p2YScaled)

	if len(cp) == 1 {
		cpXScaled, cpYScaled := roundToPixel(cp[0].X*p.pixScale, 1.0), roundToPixel(cp[0].Y*p.pixScale, 1.0)
		p.SetUniform2f(uniforms.controlPoint1, cpXScaled, cpYScaled)
	} else if len(cp) == 2 {
		cp1XScaled, cp1YScaled := roundToPixel(cp[0].X*p.pixScale, 1.0), roundToPixel(cp[0].Y*p.pixScale, 1.0)
		p.SetUniform2f(uniforms.controlPoint1, cp1XScaled, cp1YScaled)

		cp2XScaled, cp2YScaled := roundToPixel(cp[1].X*p.pixScale, 1.0), roundToPixel(cp[1].Y*p.pixScale, 1.0)
		p.SetUniform2f(uniforms.controlPoint2, cp2XScaled, cp2YScaled)
	}
	p.SetUniform1f(uniforms.numControlPoints, fyne.Min(float32(len(cp)), 2))

	strokeWidthScaled := roundToPixel(strokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidthHalf, strokeWidthScaled*0.5)

	r, g, b, a := getFragmentColor(bezierCurve.StrokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawObject(o fyne.CanvasObject, pos fyne.Position, frame fyne.Size, clip *internal.ClipItem) {
	switch obj := o.(type) {
	case *canvas.Blur:
		p.drawBlur(obj, pos, frame)
	case *canvas.Circle:
		p.drawCircle(obj, pos, frame)
	case *canvas.Line:
		p.drawLine(obj, pos, frame)
	case *canvas.Image:
		p.drawImage(obj, pos, frame)
	case *canvas.Raster:
		p.drawRaster(obj, pos, frame)
	case *canvas.Rectangle:
		p.drawRectangle(obj, pos, frame)
	case *canvas.Text:
		p.drawText(obj, pos, frame, clip)
	case *canvas.LinearGradient:
		p.drawGradient(obj, p.newGlLinearGradientTexture, pos, frame)
	case *canvas.RadialGradient:
		p.drawGradient(obj, p.newGlRadialGradientTexture, pos, frame)
	case *canvas.Polygon:
		p.drawPolygon(obj, pos, frame)
	case *canvas.RegularPolygon:
		p.drawRegularPolygon(obj, pos, frame)
	case *canvas.ArbitraryPolygon:
		p.drawArbitraryPolygon(obj, pos, frame)
	case *canvas.Arc:
		p.drawArc(obj, pos, frame)
	case *canvas.BezierCurve:
		p.drawBezierCurve(obj, pos, frame)
	case *canvas.Shader:
		p.drawShader(obj, pos, frame)
	case *canvas.Ellipse:
		p.drawEllipse(obj, pos, frame)
	}
}

// shaderProgram returns the cached state for the given shader, building and
// caching the program on first use.
func (p *painter) shaderProgram(shader *canvas.Shader) (*shaderState, bool) {
	if p.shaderPrograms == nil {
		p.shaderPrograms = make(map[string]*shaderState)
	}
	if state, ok := p.shaderPrograms[shader.Name]; ok {
		return state, state.valid
	}

	ref, err := p.createProgramFromSource(rectangleVertexSource(), userShaderFragment(shader))
	if err != nil {
		fyne.LogError("Failed to compile shader "+shader.Name, err)
		p.shaderPrograms[shader.Name] = &shaderState{}
		return p.shaderPrograms[shader.Name], false
	}

	state := &shaderState{
		program: ProgramState{
			ref:        ref,
			buff:       p.createBuffer(16),
			uniforms:   make(map[string]*UniformState),
			attributes: make(map[string]Attribute),
		},
		valid: true,
	}
	p.shaderPrograms[shader.Name] = state
	return state, true
}

func (p *painter) drawShader(shader *canvas.Shader, pos fyne.Position, frame fyne.Size) {
	state, ok := p.shaderProgram(shader)
	if !ok {
		return
	}
	program := state.program

	// Vertex: BEG
	bounds, points := p.vecRectCoords(pos, shader, frame, 0.0)
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(p.getAttribLocation(program, "vert"), 2, 4, 0)
	p.UpdateVertexArray(p.getAttribLocation(program, "normal"), 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(p.getUniformLocation(program, "frame_size"), frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(p.getUniformLocation(program, "rect_coords"), x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	for name, v := range shader.Uniforms {
		p.setProgramUniform1f(program, name, v)
	}

	p.bindShaderTextures(state, shader)
	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) bindShaderTextures(state *shaderState, shader *canvas.Shader) {
	if len(shader.Textures) == 0 {
		return
	}
	if state.textures == nil {
		state.textures = make(map[string]*shaderTexture, len(shader.Textures))
	}

	names := make([]string, 0, len(shader.Textures))
	for name, img := range shader.Textures {
		names = append(names, name)
		if cached := state.textures[name]; cached == nil || cached.src != img {
			if cached != nil {
				p.ctx.DeleteTexture(cached.tex)
			}
			state.textures[name] = &shaderTexture{tex: p.imgToTexture(img, canvas.ImageScaleSmooth), src: img}
		}
	}
	sort.Strings(names)

	for i, name := range names {
		p.ctx.ActiveTexture(texture0 + uint32(i))
		p.ctx.BindTexture(texture2D, state.textures[name].tex)
		p.setProgramUniform1i(state.program, name, int32(i))
	}
	p.ctx.ActiveTexture(texture0)
}

func (p *painter) drawRaster(img *canvas.Raster, pos fyne.Position, frame fyne.Size) {
	p.drawTextureWithDetails(img, p.newGlRasterTexture, pos, img.Size(), frame, canvas.ImageFillStretch, float32(img.Alpha()), 0)
}

func (p *painter) drawRectangle(rect *canvas.Rectangle, pos fyne.Position, frame fyne.Size) {
	topRightRadius := paint.GetCornerRadius(rect.TopRightCornerRadius, rect.CornerRadius)
	topLeftRadius := paint.GetCornerRadius(rect.TopLeftCornerRadius, rect.CornerRadius)
	bottomRightRadius := paint.GetCornerRadius(rect.BottomRightCornerRadius, rect.CornerRadius)
	bottomLeftRadius := paint.GetCornerRadius(rect.BottomLeftCornerRadius, rect.CornerRadius)
	p.drawOblong(rect, rect.FillColor, rect.StrokeColor, rect.StrokeWidth, topRightRadius, topLeftRadius, bottomRightRadius, bottomLeftRadius, rect.Aspect, rect.Shadow, pos, frame)
}

func (p *painter) drawOblong(obj fyne.CanvasObject, fill, stroke color.Color, strokeWidth, topRightRadius, topLeftRadius, bottomRightRadius, bottomLeftRadius, aspect float32, shadow canvas.Shadow, pos fyne.Position, frame fyne.Size) {
	if !paint.IsShadowVisible(shadow) && (fill == color.Transparent || fill == nil) && (stroke == color.Transparent || stroke == nil || strokeWidth == 0) {
		return
	}

	roundedCorners := topRightRadius != 0 || topLeftRadius != 0 || bottomRightRadius != 0 || bottomLeftRadius != 0
	var program ProgramState
	var vert, normal Attribute
	var frameSize, rectCoords, fillColorU, strokeColorU *UniformState
	if roundedCorners {
		program = p.roundRectangleProgram
		uniforms := &p.uniforms.roundRect
		vert, normal = uniforms.vert, uniforms.normal
		frameSize, rectCoords = uniforms.frameSize, uniforms.rectCoords
		fillColorU, strokeColorU = uniforms.fillColor, uniforms.strokeColor
	} else {
		program = p.rectangleProgram
		uniforms := &p.uniforms.rect
		vert, normal = uniforms.vert, uniforms.normal
		frameSize, rectCoords = uniforms.frameSize, uniforms.rectCoords
		fillColorU, strokeColorU = uniforms.fillColor, uniforms.strokeColor
	}

	// Vertex: BEG
	bounds, points := p.vecRectCoordsWithShadow(pos, obj, frame, aspect, shadow)
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(vert, 2, 4, 0)
	p.UpdateVertexArray(normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	strokeWidthScaled := roundToPixel(strokeWidth*p.pixScale, 1.0)
	if roundedCorners {
		uniforms := &p.uniforms.roundRect
		p.SetUniform1f(uniforms.strokeWidthHalf, strokeWidthScaled*0.5)

		rectSizeWidthScaled := x2Scaled - x1Scaled - strokeWidthScaled
		rectSizeHeightScaled := y2Scaled - y1Scaled - strokeWidthScaled
		p.SetUniform2f(uniforms.rectSizeHalf, rectSizeWidthScaled*0.5, rectSizeHeightScaled*0.5)

		// the maximum possible corner radii for a circular shape, calculated taking into account the rect coords with aspect ratio
		size := fyne.NewSize(bounds[2]-bounds[0], bounds[3]-bounds[1])
		topRightRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(topRightRadius, topLeftRadius, bottomRightRadius, size)*p.pixScale,
			1.0,
		)
		topLeftRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(topLeftRadius, topRightRadius, bottomLeftRadius, size)*p.pixScale,
			1.0,
		)
		bottomRightRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(bottomRightRadius, bottomLeftRadius, topRightRadius, size)*p.pixScale,
			1.0,
		)
		bottomLeftRadiusScaled := roundToPixel(
			paint.GetMaximumCornerRadius(bottomLeftRadius, bottomRightRadius, topLeftRadius, size)*p.pixScale,
			1.0,
		)
		p.SetUniform4f(uniforms.radius, topRightRadiusScaled, bottomRightRadiusScaled, topLeftRadiusScaled, bottomLeftRadiusScaled)

		edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
		p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)
	} else {
		p.SetUniform1f(p.uniforms.rect.strokeWidth, strokeWidthScaled)
	}

	r, g, b, a := getFragmentColor(fill)
	p.SetUniform4f(fillColorU, r, g, b, a)

	strokeColor := stroke
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(strokeColorU, r, g, b, a)

	var addShadow float32
	if paint.IsShadowVisible(shadow) {
		r, g, b, a = getFragmentColor(shadow.Color)
		if roundedCorners {
			uniforms := &p.uniforms.roundRect
			p.SetUniform4f(uniforms.shadowColor, r, g, b, a)
			p.SetUniform2f(uniforms.shadowOffset, roundToPixel(shadow.Offset.X*p.pixScale, 1.0), roundToPixel(shadow.Offset.Y*p.pixScale, 1.0))
			p.SetUniform1f(uniforms.shadowBlurRadius, roundToPixel(shadow.BlurRadius*p.pixScale, 1.0))
			p.SetUniform1f(uniforms.shadowSpread, roundToPixel(shadow.Spread*p.pixScale, 1.0))
			p.SetUniform1f(uniforms.shadowType, float32(shadow.Variant))
			p.SetUniform1f(uniforms.addShadow, 1.0)
		} else {
			uniforms := &p.uniforms.rect
			p.SetUniform4f(uniforms.shadowColor, r, g, b, a)
			p.SetUniform2f(uniforms.shadowOffset, roundToPixel(shadow.Offset.X*p.pixScale, 1.0), roundToPixel(shadow.Offset.Y*p.pixScale, 1.0))
			p.SetUniform1f(uniforms.shadowBlurRadius, roundToPixel(shadow.BlurRadius*p.pixScale, 1.0))
			p.SetUniform1f(uniforms.shadowSpread, roundToPixel(shadow.Spread*p.pixScale, 1.0))
			p.SetUniform1f(uniforms.shadowType, float32(shadow.Variant))
			p.SetUniform1f(uniforms.addShadow, 1.0)
		}
		addShadow = 1.0
	}
	if addShadow == 0 {
		if roundedCorners {
			p.SetUniform1f(p.uniforms.roundRect.addShadow, 0)
		} else {
			p.SetUniform1f(p.uniforms.rect.addShadow, 0)
		}
	}
	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawPolygon(polygon *canvas.Polygon, pos fyne.Position, frame fyne.Size) {
	if ((polygon.FillColor == color.Transparent || polygon.FillColor == nil) && (polygon.StrokeColor == color.Transparent || polygon.StrokeColor == nil || polygon.StrokeWidth == 0)) || polygon.Sides < 3 {
		return
	}
	size := polygon.Size()

	// Vertex: BEG
	bounds, points := p.vecRectCoords(pos, polygon, frame, 0.0)
	program := p.polygonProgram
	uniforms := &p.uniforms.polygon
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	outerRadius := fyne.Min(size.Width, size.Height) / 2
	outerRadiusScaled := roundToPixel(outerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.outerRadius, outerRadiusScaled)

	p.SetUniform1f(uniforms.angle, polygon.Angle)
	p.SetUniform1f(uniforms.sides, float32(polygon.Sides))

	cornerRadius := fyne.Min(paint.GetMaximumRadius(size), polygon.CornerRadius)
	cornerRadiusScaled := roundToPixel(cornerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.cornerRadius, cornerRadiusScaled)

	strokeWidthScaled := roundToPixel(polygon.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidth, strokeWidthScaled)

	r, g, b, a := getFragmentColor(polygon.FillColor)
	p.SetUniform4f(uniforms.fillColor, r, g, b, a)

	strokeColor := polygon.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawRegularPolygon(polygon *canvas.RegularPolygon, pos fyne.Position, frame fyne.Size) {
	if ((polygon.FillColor == color.Transparent || polygon.FillColor == nil) && (polygon.StrokeColor == color.Transparent || polygon.StrokeColor == nil || polygon.StrokeWidth == 0)) || polygon.Sides < 3 {
		return
	}
	size := polygon.Size()

	// Vertex: BEG
	bounds, points := p.vecRectCoords(pos, polygon, frame, 0.0)
	program := p.polygonProgram
	uniforms := &p.uniforms.polygon
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	outerRadius := fyne.Min(size.Width, size.Height) / 2
	outerRadiusScaled := roundToPixel(outerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.outerRadius, outerRadiusScaled)

	p.SetUniform1f(uniforms.angle, polygon.Angle)
	p.SetUniform1f(uniforms.sides, float32(polygon.Sides))

	cornerRadius := fyne.Min(paint.GetMaximumRadius(size), polygon.CornerRadius)
	cornerRadiusScaled := roundToPixel(cornerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.cornerRadius, cornerRadiusScaled)

	strokeWidthScaled := roundToPixel(polygon.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidth, strokeWidthScaled)

	r, g, b, a := getFragmentColor(polygon.FillColor)
	p.SetUniform4f(uniforms.fillColor, r, g, b, a)

	strokeColor := polygon.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawArbitraryPolygon(polygon *canvas.ArbitraryPolygon, pos fyne.Position, frame fyne.Size) {
	if len(polygon.Points) < 3 || ((polygon.FillColor == color.Transparent || polygon.FillColor == nil) && (polygon.StrokeColor == color.Transparent || polygon.StrokeColor == nil || polygon.StrokeWidth == 0)) {
		return
	}

	// Vertex: BEG
	bounds, points := p.vecRectCoords(pos, polygon, frame, 0.0)
	program := p.arbitraryPolygonProgram
	uniforms := &p.uniforms.arbitraryPolygon
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	numPoints := int(fyne.Min(paint.ArbitraryPolygonVerticesMaximum, float32(len(polygon.Points))))
	p.SetUniform1f(uniforms.vertexCount, float32(numPoints))

	size := polygon.Size()
	clampPoint := func(p fyne.Position) (float32, float32) {
		return fyne.Min(fyne.Max(p.X, 0), fyne.Max(size.Width, 0)), fyne.Min(fyne.Max(p.Y, 0), fyne.Max(size.Height, 0))
	}

	fixedPoints := make([]fyne.Position, numPoints)
	cornerRadii := make([]float32, numPoints)
	for i := 0; i < numPoints; i++ {
		px, py := polygon.Points[i].X, polygon.Points[i].Y
		if polygon.NormalizedPoints {
			px, py = px*size.Width, py*size.Height
		}
		px, py = clampPoint(fyne.NewPos(px, py))
		fixedPoints[i] = fyne.NewPos(px, py)

		var radius float32
		if i < len(polygon.CornerRadii) {
			radius = polygon.CornerRadii[i]
		}
		cornerRadii[i] = radius
	}

	cornerRadii = paint.GetMaximumCornerRadii(fixedPoints, cornerRadii)

	verticesScaled := make([]float32, numPoints*2)
	cornerRadiiScaled := make([]float32, numPoints)
	for i := 0; i < numPoints; i++ {
		verticesScaled[i*2] = roundToPixel(fixedPoints[i].X*p.pixScale, 1.0)
		verticesScaled[i*2+1] = roundToPixel(fixedPoints[i].Y*p.pixScale, 1.0)
		cornerRadiiScaled[i] = roundToPixel(cornerRadii[i]*p.pixScale, 1.0)
	}

	p.SetUniform2fv(uniforms.vertices, verticesScaled)
	p.SetUniform1fv(uniforms.cornerRadii, cornerRadiiScaled)

	r, g, b, a := getFragmentColor(polygon.FillColor)
	p.SetUniform4f(uniforms.fillColor, r, g, b, a)

	strokeColor := polygon.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	strokeWidthScaled := roundToPixel(polygon.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidth, strokeWidthScaled)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawArc(arc *canvas.Arc, pos fyne.Position, frame fyne.Size) {
	if ((arc.FillColor == color.Transparent || arc.FillColor == nil) && (arc.StrokeColor == color.Transparent || arc.StrokeColor == nil || arc.StrokeWidth == 0)) || arc.StartAngle == arc.EndAngle {
		return
	}

	// Vertex: BEG
	bounds, points := p.vecRectCoords(pos, arc, frame, 0.0)
	program := p.arcProgram
	uniforms := &p.uniforms.arc
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	outerRadius := fyne.Min(arc.Size().Width, arc.Size().Height) / 2
	outerRadiusScaled := roundToPixel(outerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.outerRadius, outerRadiusScaled)

	innerRadius := outerRadius * float32(math.Min(1.0, math.Max(0.0, float64(arc.CutoutRatio))))
	innerRadiusScaled := roundToPixel(innerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.innerRadius, innerRadiusScaled)

	startAngle, endAngle := paint.NormalizeArcAngles(arc.StartAngle, arc.EndAngle)
	p.SetUniform1f(uniforms.startAngle, startAngle)
	p.SetUniform1f(uniforms.endAngle, endAngle)

	cornerRadius := fyne.Min(paint.GetMaximumRadiusArc(outerRadius, innerRadius, arc.EndAngle-arc.StartAngle), arc.CornerRadius)
	cornerRadiusScaled := roundToPixel(cornerRadius*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.cornerRadius, cornerRadiusScaled)

	strokeWidthScaled := roundToPixel(arc.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidth, strokeWidthScaled)

	r, g, b, a := getFragmentColor(arc.FillColor)
	p.SetUniform4f(uniforms.fillColor, r, g, b, a)

	strokeColor := arc.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawEllipse(ellipse *canvas.Ellipse, pos fyne.Position, frame fyne.Size) {
	size := ellipse.Size()
	radiusX := size.Width / 2
	radiusY := size.Height / 2
	program := p.ellipseProgram
	uniforms := &p.uniforms.ellipse

	width, height := size.Components()
	rotPad := float32(math.Abs(float64(width)-float64(height)) / 2)
	xPad, yPad := float32(0), float32(0)
	if width > height {
		yPad = rotPad
	} else {
		xPad = rotPad
	}

	// Vertex: BEG
	bounds, points := p.vecRectCoordsWithPadAndShadow(pos, ellipse, frame, -xPad, -yPad, ellipse.Shadow)
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 2, 4, 0)
	p.UpdateVertexArray(uniforms.normal, 2, 4, 2)

	p.blendFunc(srcAlpha, oneMinusSrcAlpha)
	p.logError()
	// Vertex: END

	// Fragment: BEG
	frameWidthScaled, frameHeightScaled := p.scaleFrameSize(frame)
	p.SetUniform2f(uniforms.frameSize, frameWidthScaled, frameHeightScaled)

	x1Scaled, x2Scaled, y1Scaled, y2Scaled := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(uniforms.rectCoords, x1Scaled, x2Scaled, y1Scaled, y2Scaled)

	strokeWidthScaled := roundToPixel(ellipse.StrokeWidth*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.strokeWidth, strokeWidthScaled)

	radiusXScaled := roundToPixel(radiusX*p.pixScale, 1.0)
	radiusYScaled := roundToPixel(radiusY*p.pixScale, 1.0)
	p.SetUniform2f(uniforms.radius, radiusXScaled, radiusYScaled)

	p.SetUniform1f(uniforms.angle, 0)

	r, g, b, a := getFragmentColor(ellipse.FillColor)
	p.SetUniform4f(uniforms.fillColor, r, g, b, a)

	strokeColor := ellipse.StrokeColor
	if strokeColor == nil {
		strokeColor = color.Transparent
	}
	r, g, b, a = getFragmentColor(strokeColor)
	p.SetUniform4f(uniforms.strokeColor, r, g, b, a)

	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	p.SetUniform1f(uniforms.edgeSoftness, edgeSoftnessScaled)

	var addShadow float32
	if paint.IsShadowVisible(ellipse.Shadow) {
		r, g, b, a = getFragmentColor(ellipse.Shadow.Color)
		p.SetUniform4f(uniforms.shadowColor, r, g, b, a)
		p.SetUniform2f(uniforms.shadowOffset, roundToPixel(ellipse.Shadow.Offset.X*p.pixScale, 1.0), roundToPixel(ellipse.Shadow.Offset.Y*p.pixScale, 1.0))
		p.SetUniform1f(uniforms.shadowBlurRadius, roundToPixel(ellipse.Shadow.BlurRadius*p.pixScale, 1.0))
		p.SetUniform1f(uniforms.shadowSpread, roundToPixel(ellipse.Shadow.Spread*p.pixScale, 1.0))
		p.SetUniform1f(uniforms.shadowType, float32(ellipse.Shadow.Variant))
		addShadow = 1.0
	}
	p.SetUniform1f(uniforms.addShadow, addShadow)

	p.logError()
	// Fragment: END

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) drawText(text *canvas.Text, pos fyne.Position, frame fyne.Size, clip *internal.ClipItem) {
	if text.Text == "" {
		return
	}
	decorated := text.TextStyle.Underline || text.TextStyle.Strikethrough
	if text.Text == " " && !decorated {
		return
	}

	size := text.MinSize()
	containerSize := text.Size()
	switch text.Alignment {
	case fyne.TextAlignTrailing:
		pos = fyne.NewPos(pos.X+containerSize.Width-size.Width, pos.Y)
	case fyne.TextAlignCenter:
		pos = fyne.NewPos(pos.X+(containerSize.Width-size.Width)/2, pos.Y)
	}

	if containerSize.Height > size.Height {
		pos = fyne.NewPos(pos.X, pos.Y+(containerSize.Height-size.Height)/2)
	}

	var clipPos fyne.Position
	var clipSize fyne.Size
	if clip != nil {
		clipPos, clipSize = clip.Rect()
	} else {
		clipSize = frame
	}
	if pos.Y > clipPos.Y+clipSize.Height || pos.Y+size.Height < clipPos.Y ||
		pos.X > clipPos.X+clipSize.Width || pos.X+size.Width < clipPos.X {
		return
	}

	// text size is sensitive to position on screen
	size.Width = roundToPixel(size.Width, p.pixScale)
	size.Height = roundToPixel(size.Height, p.pixScale)
	size.Width += roundToPixel(paint.VectorPad(text), p.pixScale)
	size.Height += roundToPixel(paint.TextVectorPad, p.pixScale)
	p.drawTextureWithDetails(text, p.newGlTextTexture, pos, size, frame, canvas.ImageFillStretch, 1.0, 0)

	if decorated {
		_, baseline := cache.GetFontMetrics(text.Text, text.TextSize, text.TextStyle, text.FontSource)
		line := canvas.NewLine(text.Color)
		line.Resize(fyne.NewSize(size.Width, 0))
		if text.TextStyle.Underline {
			underlinePos := fyne.NewPos(pos.X, pos.Y+baseline+paint.UnderlineOffsetFromBaseline)
			p.drawLine(line, underlinePos, frame)
		}
		if text.TextStyle.Strikethrough {
			strikePos := fyne.NewPos(pos.X, pos.Y+baseline*paint.StrikethroughToBaselineFactor)
			p.drawLine(line, strikePos, frame)
		}
	}
}

func (p *painter) drawTextureWithDetails(o fyne.CanvasObject, creator func(canvasObject fyne.CanvasObject) Texture,
	pos fyne.Position, size, frame fyne.Size, fill canvas.ImageFill, alpha float32, pad float32,
) {
	texture, err := p.getTexture(o, creator)
	if err != nil {
		return
	}

	cornerRadius := float32(0)
	aspect := float32(0)
	if img, ok := o.(*canvas.Image); ok {
		aspect = img.Aspect()
		if aspect == 0 {
			aspect = 1 // fallback, should not occur - normally an image load error
		}
		if img.CornerRadius > 0 {
			cornerRadius = img.CornerRadius
		}
	}
	points, insets := p.rectCoords(size, pos, frame, fill, aspect, pad)
	inner, _ := rectInnerCoords(size, pos, fill, aspect)

	uniforms := &p.uniforms.simple
	p.ctx.UseProgram(p.program.ref)
	p.updateBuffer(p.program.buff, points)
	p.UpdateVertexArray(uniforms.vert, 3, 5, 0)
	p.UpdateVertexArray(uniforms.vertTexCoord, 2, 5, 3)

	// Set corner radius and texture size in pixels
	cornerRadius = fyne.Min(paint.GetMaximumRadius(size), cornerRadius)
	p.SetUniform1f(uniforms.cornerRadius, cornerRadius*p.pixScale)
	p.SetUniform2f(uniforms.size, inner.Width*p.pixScale, inner.Height*p.pixScale)
	p.SetUniform4f(uniforms.inset, insets[0], insets[1], insets[2], insets[3]) // texture coordinate insets (minX, minY, maxX, maxY)

	p.SetUniform1f(uniforms.alpha, alpha)

	p.blendFunc(one, oneMinusSrcAlpha)
	p.logError()

	p.ctx.ActiveTexture(texture0)
	p.ctx.BindTexture(texture2D, texture)
	p.logError()

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
}

func (p *painter) lineCoords(pos, pos1, pos2 fyne.Position, lineWidth, feather float32, frame fyne.Size) ([]float32, float32, float32) {
	// Shift line coordinates so that they match the target position.
	xPosDiff := pos.X - fyne.Min(pos1.X, pos2.X)
	yPosDiff := pos.Y - fyne.Min(pos1.Y, pos2.Y)
	pos1.X = roundToPixel(pos1.X+xPosDiff, p.pixScale)
	pos1.Y = roundToPixel(pos1.Y+yPosDiff, p.pixScale)
	pos2.X = roundToPixel(pos2.X+xPosDiff, p.pixScale)
	pos2.Y = roundToPixel(pos2.Y+yPosDiff, p.pixScale)

	if lineWidth <= 1 {
		offset := float32(0.5)                  // adjust location for lines < 1pt on regular display
		if lineWidth <= 0.5 && p.pixScale > 1 { // and for 1px drawing on HiDPI (width 0.5)
			offset = 0.25
		}
		if pos1.X == pos2.X {
			pos1.X -= offset
			pos2.X -= offset
		}
		if pos1.Y == pos2.Y {
			pos1.Y -= offset
			pos2.Y -= offset
		}
	}

	x1Pos := pos1.X / frame.Width
	x1 := -1 + x1Pos*2
	y1Pos := pos1.Y / frame.Height
	y1 := 1 - y1Pos*2
	x2Pos := pos2.X / frame.Width
	x2 := -1 + x2Pos*2
	y2Pos := pos2.Y / frame.Height
	y2 := 1 - y2Pos*2

	normalX := (pos2.Y - pos1.Y) / frame.Width
	normalY := (pos2.X - pos1.X) / frame.Height
	dirLength := float32(math.Sqrt(float64(normalX*normalX + normalY*normalY)))
	normalX /= dirLength
	normalY /= dirLength

	normalObjX := normalX * 0.5 * frame.Width
	normalObjY := normalY * 0.5 * frame.Height
	widthMultiplier := float32(math.Sqrt(float64(normalObjX*normalObjX + normalObjY*normalObjY)))
	halfWidth := (roundToPixel(lineWidth+feather, p.pixScale) * 0.5) / widthMultiplier
	featherWidth := feather / widthMultiplier

	points := append(
		p.scratch(),
		// coord x, y normal x, y
		x1, y1, normalX, normalY,
		x2, y2, normalX, normalY,
		x2, y2, -normalX, -normalY,
		x2, y2, -normalX, -normalY,
		x1, y1, normalX, normalY,
		x1, y1, -normalX, -normalY,
	)
	return points, halfWidth, featherWidth
}

// rectCoords calculates the openGL coordinate space of a rectangle
func (p *painter) rectCoords(size fyne.Size, pos fyne.Position, frame fyne.Size,
	fill canvas.ImageFill, aspect float32, pad float32,
) ([]float32, [4]float32) {
	size, pos = rectInnerCoords(size, pos, fill, aspect)
	size, pos = roundToPixelCoords(size, pos, p.pixScale)

	xPos := (pos.X - pad) / frame.Width
	x1 := -1 + xPos*2
	x2Pos := (pos.X + size.Width + pad) / frame.Width
	x2 := -1 + x2Pos*2

	yPos := (pos.Y - pad) / frame.Height
	y1 := 1 - yPos*2
	y2Pos := (pos.Y + size.Height + pad) / frame.Height
	y2 := 1 - y2Pos*2

	xInset := float32(0.0)
	yInset := float32(0.0)

	if fill == canvas.ImageFillCover {
		viewAspect := size.Width / size.Height

		if viewAspect > aspect {
			newHeight := size.Width / aspect
			heightPad := (newHeight - size.Height) / 2
			yInset = heightPad / newHeight
		} else if viewAspect < aspect {
			newWidth := size.Height * aspect
			widthPad := (newWidth - size.Width) / 2
			xInset = widthPad / newWidth
		}
	}

	insets := [4]float32{xInset, yInset, 1.0 - xInset, 1.0 - yInset}

	points := append(
		p.scratch(),
		// coord x, y, z texture x, y
		x1, y2, 0, insets[0], insets[3], // top left
		x1, y1, 0, insets[0], insets[1], // bottom left
		x2, y2, 0, insets[2], insets[3], // top right
		x2, y1, 0, insets[2], insets[1], // bottom right
	)
	return points, insets
}

func rectInnerCoords(size fyne.Size, pos fyne.Position, fill canvas.ImageFill, aspect float32) (fyne.Size, fyne.Position) {
	if fill == canvas.ImageFillContain || fill == canvas.ImageFillOriginal {
		// change pos and size accordingly

		viewAspect := size.Width / size.Height

		newWidth, newHeight := size.Width, size.Height
		widthPad, heightPad := float32(0), float32(0)
		if viewAspect > aspect {
			newWidth = size.Height * aspect
			widthPad = (size.Width - newWidth) / 2
		} else if viewAspect < aspect {
			newHeight = size.Width / aspect
			heightPad = (size.Height - newHeight) / 2
		}

		return fyne.NewSize(newWidth, newHeight), fyne.NewPos(pos.X+widthPad, pos.Y+heightPad)
	}

	return size, pos
}

func (p *painter) vecRectCoords(pos fyne.Position, rect fyne.CanvasObject, frame fyne.Size, aspect float32) ([4]float32, []float32) {
	return p.vecRectCoordsWithShadow(pos, rect, frame, aspect, canvas.Shadow{})
}

func (p *painter) vecRectCoordsWithShadow(pos fyne.Position, rect fyne.CanvasObject, frame fyne.Size, aspect float32, shadow canvas.Shadow) ([4]float32, []float32) {
	xPad, yPad := float32(0), float32(0)

	if aspect != 0 {
		inner := rect.Size()
		frameAspect := inner.Width / inner.Height

		if frameAspect > aspect {
			newWidth := inner.Height * aspect
			xPad = (inner.Width - newWidth) / 2
		} else if frameAspect < aspect {
			newHeight := inner.Width / aspect
			yPad = (inner.Height - newHeight) / 2
		}
	}

	return p.vecRectCoordsWithPadAndShadow(pos, rect, frame, xPad, yPad, shadow)
}

func (p *painter) vecRectCoordsWithPadAndShadow(pos fyne.Position, rect fyne.CanvasObject, frame fyne.Size, xPad, yPad float32, shadow canvas.Shadow) ([4]float32, []float32) {
	size := rect.Size()
	pos1 := rect.Position()

	xPosDiff := pos.X - pos1.X + xPad
	yPosDiff := pos.Y - pos1.Y + yPad
	pos1.X = roundToPixel(pos1.X+xPosDiff, p.pixScale)
	pos1.Y = roundToPixel(pos1.Y+yPosDiff, p.pixScale)
	size.Width = roundToPixel(size.Width-2*xPad, p.pixScale)
	size.Height = roundToPixel(size.Height-2*yPad, p.pixScale)

	shadowPads := paint.GetShadowPaddings(shadow)
	shadowPadLeft := roundToPixel(shadowPads[0], p.pixScale)
	shadowPadTop := roundToPixel(shadowPads[1], p.pixScale)
	shadowPadRight := roundToPixel(shadowPads[2], p.pixScale)
	shadowPadBottom := roundToPixel(shadowPads[3], p.pixScale)

	// without edge softness adjustment the rectangle has cropped edges
	edgeSoftnessScaled := roundToPixel(edgeSoftness*p.pixScale, 1.0)
	x1Pos := pos1.X
	x1Norm := -1 + (x1Pos-edgeSoftnessScaled-shadowPadLeft)*2/frame.Width
	x2Pos := pos1.X + size.Width
	x2Norm := -1 + (x2Pos+edgeSoftnessScaled+shadowPadRight)*2/frame.Width
	y1Pos := pos1.Y
	y1Norm := 1 - (y1Pos-edgeSoftnessScaled-shadowPadTop)*2/frame.Height
	y2Pos := pos1.Y + size.Height
	y2Norm := 1 - (y2Pos+edgeSoftnessScaled+shadowPadBottom)*2/frame.Height

	// output a norm for the fill and the vert is unused, but we pass 0 to avoid optimisation issues
	coords := append(
		p.scratch(),
		0, 0, x1Norm, y1Norm, // first triangle
		0, 0, x2Norm, y1Norm, // second triangle
		0, 0, x1Norm, y2Norm,
		0, 0, x2Norm, y2Norm,
	)

	return [4]float32{x1Pos, y1Pos, x2Pos, y2Pos}, coords
}

func (p *painter) vecSquareCoordsWithShadow(pos fyne.Position, rect fyne.CanvasObject, frame fyne.Size, shadow canvas.Shadow) ([4]float32, []float32) {
	return p.vecRectCoordsWithShadow(pos, rect, frame, 1, shadow)
}

func roundToPixel(v float32, pixScale float32) float32 {
	if pixScale == 1.0 {
		return float32(math.Round(float64(v)))
	}

	return float32(math.Round(float64(v*pixScale))) / pixScale
}

func roundToPixelCoords(size fyne.Size, pos fyne.Position, pixScale float32) (fyne.Size, fyne.Position) {
	end := pos.Add(size)
	end.X = roundToPixel(end.X, pixScale)
	end.Y = roundToPixel(end.Y, pixScale)
	pos.X = roundToPixel(pos.X, pixScale)
	pos.Y = roundToPixel(pos.Y, pixScale)
	size.Width = end.X - pos.X
	size.Height = end.Y - pos.Y

	return size, pos
}

// Returns FragmentColor(red,green,blue,alpha) from fyne.Color
func getFragmentColor(col color.Color) (float32, float32, float32, float32) {
	if col == nil {
		return 0, 0, 0, 0
	}
	r, g, b, a := col.RGBA()
	if a == 0 {
		return 0, 0, 0, 0
	}
	alpha := float32(a)
	return float32(r) / alpha, float32(g) / alpha, float32(b) / alpha, alpha / 0xffff
}

func (p *painter) scaleFrameSize(frame fyne.Size) (float32, float32) {
	frameWidthScaled := roundToPixel(frame.Width*p.pixScale, 1.0)
	frameHeightScaled := roundToPixel(frame.Height*p.pixScale, 1.0)
	return frameWidthScaled, frameHeightScaled
}

// Returns scaled RectCoords(x1,x2,y1,y2) in same order
func (p *painter) scaleRectCoords(x1, x2, y1, y2 float32) (float32, float32, float32, float32) {
	x1Scaled := roundToPixel(x1*p.pixScale, 1.0)
	x2Scaled := roundToPixel(x2*p.pixScale, 1.0)
	y1Scaled := roundToPixel(y1*p.pixScale, 1.0)
	y2Scaled := roundToPixel(y2*p.pixScale, 1.0)
	return x1Scaled, x2Scaled, y1Scaled, y2Scaled
}

func createKernel(radius float32) []float32 {
	sum := float32(0.0)
	length := int(radius)*2 + 1
	values := make([]float32, length)
	for i, x := 0, float64(-radius); i < length; i, x = i+1, x+1 {
		value := float32(math.Exp(-(x * x / 4 / float64(radius))))
		values[i] = value
		sum += value
	}
	for i := 0; i < length; i++ {
		values[i] /= sum
	}

	return values
}
