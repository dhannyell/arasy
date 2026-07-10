//go:build js && wasm

package wasm

import (
	"fmt"
	"log/slog"
	"math"
	"syscall/js"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

const imageQuadGridSize = 16

type Canvas2D struct {
	ctx       js.Value
	canvas    js.Value
	clear     render.Color
	textures  map[render.TextureID]js.Value
	tintCache map[string]js.Value
	log       *slog.Logger
}

func newCanvas2D(canvas js.Value, clear render.Color, manifest asset.Manifest, log *slog.Logger) *Canvas2D {
	if log == nil {
		log = slog.Default()
	}
	c := &Canvas2D{
		ctx:       canvas.Call("getContext", "2d"),
		canvas:    canvas,
		clear:     clear,
		textures:  make(map[render.TextureID]js.Value),
		tintCache: make(map[string]js.Value),
		log:       log,
	}
	for _, decl := range manifest.Textures() {
		c.loadTexture(decl)
	}
	return c
}

// loadTexture decodes manifest bytes asynchronously. Until createImageBitmap
// resolves, draw calls use the same magenta placeholder as the raster
// backend rather than blocking the browser render loop.
func (c *Canvas2D) loadTexture(decl asset.TextureDecl) {
	createImageBitmap := js.Global().Get("createImageBitmap")
	if createImageBitmap.IsUndefined() {
		c.log.Warn("Canvas2D ImageBitmap unavailable; texture will use placeholder", "texture", decl.Name)
		return
	}

	blob := js.Global().Get("Blob").New([]any{bytesToJS(decl.Data)})
	promise := createImageBitmap.Invoke(blob)

	var resolve, reject js.Func
	release := func() {
		resolve.Release()
		reject.Release()
	}
	resolve = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			c.log.Warn("Canvas2D texture decode returned no bitmap", "texture", decl.Name)
		} else {
			c.textures[decl.ID] = args[0]
		}
		release()
		return nil
	})
	reject = js.FuncOf(func(this js.Value, args []js.Value) any {
		c.log.Warn("Canvas2D texture decode failed; using placeholder", "texture", decl.Name)
		release()
		return nil
	})
	promise.Call("then", resolve).Call("catch", reject)
}

func (c *Canvas2D) UpdateTextures(updates []runtime.TextureUpdate) {
	for _, u := range updates {
		if u.Image == nil {
			continue
		}
		b := u.Image.Bounds()
		c.UpdateTexture(u.ID, u.Image.Pix, b.Dx(), b.Dy())
	}
}

func (c *Canvas2D) UpdateTexture(id render.TextureID, pix []byte, w, h int) {
	createImageBitmap := js.Global().Get("createImageBitmap")
	if createImageBitmap.IsUndefined() {
		return
	}
	canvas := js.Global().Get("document").Call("createElement", "canvas")
	canvas.Set("width", w)
	canvas.Set("height", h)
	ctx := canvas.Call("getContext", "2d")
	imgData := ctx.Call("createImageData", w, h)
	jsPix := js.Global().Get("Uint8ClampedArray").New(js.Global().Get("Array").New(len(pix)))
	js.CopyBytesToJS(jsPix, pix)
	imgData.Set("data", jsPix)
	ctx.Call("putImageData", imgData, 0, 0)
	createImageBitmap.Invoke(canvas).Call("then", js.FuncOf(func(this js.Value, args []js.Value) any {
		if old, ok := c.textures[id]; ok {
			old.Call("close")
		}
		c.textures[id] = args[0]
		return nil
	}))
}

func (c *Canvas2D) Render(frame runtime.Frame) {
	w := c.canvas.Get("width").Float()
	h := c.canvas.Get("height").Float()
	c.ctx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	c.ctx.Call("clearRect", 0, 0, w, h)
	if c.clear.A > 0 {
		c.ctx.Set("fillStyle", cssColor(c.clear))
		c.ctx.Call("fillRect", 0, 0, w, h)
	}

	for _, cmd := range frame.Commands {
		c.ctx.Call("save")
		c.applyClip(cmd, frame.Camera)
		switch cmd := cmd.(type) {
		case render.DrawRect:
			c.drawRect(cmd, frame.Camera)
		case render.DrawCircle:
			c.drawCircle(cmd, frame.Camera)
		case render.DrawPoint:
			c.drawPoint(cmd, frame.Camera)
		case render.DrawLine:
			c.drawLine(cmd, frame.Camera)
		case render.DrawArc:
			c.drawArc(cmd, frame.Camera)
		case render.DrawText:
			c.drawText(cmd, frame.Camera)
		case render.DrawPolygon:
			c.drawPolygon(cmd, frame.Camera)
		case render.DrawImage:
			c.drawImage(cmd, frame.Camera)
		case render.DrawImageQuad:
			c.drawImageQuad(cmd, frame.Camera)
		}
		c.ctx.Call("restore")
	}
}

// drawImage is the Canvas2D fallback implementation of DrawImage. It matches
// the raster/WebGPU backends' full RGB tint (not just alpha) via
// tintedBitmap, mirroring the JS dev host's frontend approximation
// (static/js/primitives.js tintedBitmap) so the two dev tools agree.
func (c *Canvas2D) drawImage(cmd render.DrawImage, camera render.Camera) {
	x, y, w, h := screenRect(cmd.Dest, camera)
	if w <= 0 || h <= 0 {
		return
	}

	bitmap, ok := c.textures[cmd.Texture]
	if !ok || !bitmap.Truthy() {
		c.drawImagePlaceholder(x, y, w, h, cmd.Rotation)
		return
	}

	sx, sy, sw, sh, ok := imageSource(cmd.Src, bitmap)
	if !ok {
		return
	}

	source := bitmap
	c.ctx.Call("save")
	if cmd.Tint != (render.Color{}) {
		c.ctx.Set("globalAlpha", cmd.Tint.A)
		source = c.tintedBitmap(bitmap, cmd.Texture, cmd.Tint)
	}
	if cmd.Rotation != 0 {
		c.ctx.Call("translate", x+w/2, y+h/2)
		c.ctx.Call("rotate", cmd.Rotation)
		c.ctx.Call("drawImage", source, sx, sy, sw, sh, -w/2, -h/2, w, h)
	} else {
		c.ctx.Call("drawImage", source, sx, sy, sw, sh, x, y, w, h)
	}
	c.ctx.Call("restore")
}

// drawImageQuad approximates projective mapping by splitting the source into
// a grid of affine triangles. This is intentionally a development-host
// approximation; the raster and WebGPU paths define the exact semantics.
func (c *Canvas2D) drawImageQuad(cmd render.DrawImageQuad, camera render.Camera) {
	var corners [4]math2d.Vec2
	for i, corner := range cmd.Corners {
		corners[i] = camera.WorldToScreen(corner)
	}
	if !math2d.IsConvexQuad(corners) {
		return
	}
	homography, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		return
	}

	bitmap, found := c.textures[cmd.Texture]
	if !found || !bitmap.Truthy() {
		c.fillScreenQuad(corners)
		return
	}
	sx, sy, sw, sh, ok := imageSource(cmd.Src, bitmap)
	if !ok {
		return
	}

	alpha := 1.0
	source := bitmap
	if cmd.Tint != (render.Color{}) {
		alpha = cmd.Tint.A
		source = c.tintedBitmap(bitmap, cmd.Texture, cmd.Tint)
	}
	for row := 0; row < imageQuadGridSize; row++ {
		v0 := float64(row) / imageQuadGridSize
		v1 := float64(row+1) / imageQuadGridSize
		for col := 0; col < imageQuadGridSize; col++ {
			u0 := float64(col) / imageQuadGridSize
			u1 := float64(col+1) / imageQuadGridSize

			p00 := homography.Apply(math2d.Vec2{X: u0, Y: v0})
			p10 := homography.Apply(math2d.Vec2{X: u1, Y: v0})
			p11 := homography.Apply(math2d.Vec2{X: u1, Y: v1})
			p01 := homography.Apply(math2d.Vec2{X: u0, Y: v1})
			s00 := math2d.Vec2{X: sx + u0*sw, Y: sy + v0*sh}
			s10 := math2d.Vec2{X: sx + u1*sw, Y: sy + v0*sh}
			s11 := math2d.Vec2{X: sx + u1*sw, Y: sy + v1*sh}
			s01 := math2d.Vec2{X: sx + u0*sw, Y: sy + v1*sh}

			c.drawTexturedTriangle(source, alpha, s00, s10, s01, p00, p10, p01)
			c.drawTexturedTriangle(source, alpha, s10, s11, s01, p10, p11, p01)
		}
	}
}

// tintedBitmap returns bitmap with its color channels multiplied by tint,
// preserving the original alpha, mirroring the JS frontend's tintedBitmap
// (static/js/primitives.js) so both dev hosts render the same tint. Results
// are cached per (textureID, tint rgb): an opaque offscreen canvas is
// composited with the bitmap ("multiply"), then "destination-in" restores
// the source alpha. A white tint is alpha-only and returns bitmap unchanged,
// since globalAlpha already handles it at the call site.
func (c *Canvas2D) tintedBitmap(bitmap js.Value, textureID render.TextureID, tint render.Color) js.Value {
	r := int(math.Round(tint.R * 255))
	g := int(math.Round(tint.G * 255))
	b := int(math.Round(tint.B * 255))
	if r == 255 && g == 255 && b == 255 {
		return bitmap
	}

	key := fmt.Sprintf("%d:%d,%d,%d", textureID, r, g, b)
	if cached, ok := c.tintCache[key]; ok {
		return cached
	}

	width := bitmap.Get("width")
	height := bitmap.Get("height")
	canvas := js.Global().Get("document").Call("createElement", "canvas")
	canvas.Set("width", width)
	canvas.Set("height", height)

	off := canvas.Call("getContext", "2d")
	off.Call("drawImage", bitmap, 0, 0)
	off.Set("globalCompositeOperation", "multiply")
	off.Set("fillStyle", fmt.Sprintf("rgb(%d, %d, %d)", r, g, b))
	off.Call("fillRect", 0, 0, width, height)
	off.Set("globalCompositeOperation", "destination-in")
	off.Call("drawImage", bitmap, 0, 0)

	c.tintCache[key] = canvas
	return canvas
}

func (c *Canvas2D) drawTexturedTriangle(bitmap js.Value, alpha float64, s0, s1, s2, d0, d1, d2 math2d.Vec2) {
	a, b, cc, d, e, f, ok := affineFromTriangles(s0, s1, s2, d0, d1, d2)
	if !ok {
		return
	}
	c.ctx.Call("save")
	c.ctx.Set("globalAlpha", alpha)
	c.ctx.Call("setTransform", a, b, cc, d, e, f)
	c.ctx.Call("beginPath")
	c.ctx.Call("moveTo", s0.X, s0.Y)
	c.ctx.Call("lineTo", s1.X, s1.Y)
	c.ctx.Call("lineTo", s2.X, s2.Y)
	c.ctx.Call("closePath")
	c.ctx.Call("clip")
	c.ctx.Call("drawImage", bitmap, 0, 0)
	c.ctx.Call("restore")
}

func affineFromTriangles(s0, s1, s2, d0, d1, d2 math2d.Vec2) (a, b, c, d, e, f float64, ok bool) {
	den := (s1.X-s0.X)*(s2.Y-s0.Y) - (s2.X-s0.X)*(s1.Y-s0.Y)
	if math.Abs(den) <= 1e-12 {
		return 0, 0, 0, 0, 0, 0, false
	}
	a = ((d1.X-d0.X)*(s2.Y-s0.Y) - (d2.X-d0.X)*(s1.Y-s0.Y)) / den
	b = ((d1.Y-d0.Y)*(s2.Y-s0.Y) - (d2.Y-d0.Y)*(s1.Y-s0.Y)) / den
	c = ((d2.X-d0.X)*(s1.X-s0.X) - (d1.X-d0.X)*(s2.X-s0.X)) / den
	d = ((d2.Y-d0.Y)*(s1.X-s0.X) - (d1.Y-d0.Y)*(s2.X-s0.X)) / den
	e = d0.X - a*s0.X - c*s0.Y
	f = d0.Y - b*s0.X - d*s0.Y
	return a, b, c, d, e, f, true
}

func imageSource(src math2d.Rect, bitmap js.Value) (x, y, w, h float64, ok bool) {
	bw := bitmap.Get("width").Float()
	bh := bitmap.Get("height").Float()
	if src == (math2d.Rect{}) {
		return 0, 0, bw, bh, bw > 0 && bh > 0
	}
	x = math.Max(0, math.Min(bw, math.Round(src.Min.X)))
	y = math.Max(0, math.Min(bh, math.Round(src.Min.Y)))
	maxX := math.Max(0, math.Min(bw, math.Round(src.Max.X)))
	maxY := math.Max(0, math.Min(bh, math.Round(src.Max.Y)))
	return x, y, maxX - x, maxY - y, maxX > x && maxY > y
}

func (c *Canvas2D) drawImagePlaceholder(x, y, w, h, rotation float64) {
	c.ctx.Set("fillStyle", "rgb(255, 0, 255)")
	if rotation == 0 {
		c.ctx.Call("fillRect", x, y, w, h)
		return
	}
	c.ctx.Call("save")
	c.ctx.Call("translate", x+w/2, y+h/2)
	c.ctx.Call("rotate", rotation)
	c.ctx.Call("fillRect", -w/2, -h/2, w, h)
	c.ctx.Call("restore")
}

func (c *Canvas2D) fillScreenQuad(corners [4]math2d.Vec2) {
	c.ctx.Set("fillStyle", "rgb(255, 0, 255)")
	c.ctx.Call("beginPath")
	c.ctx.Call("moveTo", corners[0].X, corners[0].Y)
	for _, corner := range corners[1:] {
		c.ctx.Call("lineTo", corner.X, corner.Y)
	}
	c.ctx.Call("closePath")
	c.ctx.Call("fill")
}

func cssColor(c render.Color) string {
	return fmt.Sprintf("rgba(%d,%d,%d,%g)", int(math.Round(c.R*255)), int(math.Round(c.G*255)), int(math.Round(c.B*255)), c.A)
}

func (c *Canvas2D) drawRect(cmd render.DrawRect, camera render.Camera) {
	x, y, w, h := screenRect(cmd.Rect, camera)

	if cmd.CornerRadius > 0 && c.ctx.Get("roundRect").Truthy() {
		c.ctx.Call("beginPath")
		c.ctx.Call("roundRect", x, y, w, h, camera.WorldScalarToScreen(cmd.CornerRadius))
		if cmd.Fill.A > 0 {
			c.ctx.Set("fillStyle", cssColor(cmd.Fill))
			c.ctx.Call("fill")
		}
		if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
			c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
			c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
			c.ctx.Call("stroke")
		}
		return
	}

	if cmd.Fill.A > 0 {
		c.ctx.Set("fillStyle", cssColor(cmd.Fill))
		c.ctx.Call("fillRect", x, y, w, h)
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
		c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
		c.ctx.Call("strokeRect", x, y, w, h)
	}
}

func (c *Canvas2D) drawCircle(cmd render.DrawCircle, camera render.Camera) {
	center := camera.WorldToScreen(cmd.Center)
	radius := camera.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("arc", center.X, center.Y, radius, 0, 2*math.Pi)
	if cmd.Fill.A > 0 {
		c.ctx.Set("fillStyle", cssColor(cmd.Fill))
		c.ctx.Call("fill")
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
		c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
		c.ctx.Call("stroke")
	}
}

func (c *Canvas2D) drawPoint(cmd render.DrawPoint, camera render.Camera) {
	if cmd.Color.A <= 0 {
		return
	}

	center := camera.WorldToScreen(cmd.Center)
	radius := camera.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("arc", center.X, center.Y, radius, 0, 2*math.Pi)
	c.ctx.Set("fillStyle", cssColor(cmd.Color))
	c.ctx.Call("fill")
}

func (c *Canvas2D) drawLine(cmd render.DrawLine, camera render.Camera) {
	if cmd.Color.A <= 0 || cmd.Width <= 0 {
		return
	}

	from := camera.WorldToScreen(cmd.From)
	to := camera.WorldToScreen(cmd.To)

	c.ctx.Call("beginPath")
	c.ctx.Call("moveTo", from.X, from.Y)
	c.ctx.Call("lineTo", to.X, to.Y)
	c.ctx.Set("strokeStyle", cssColor(cmd.Color))
	c.ctx.Set("lineWidth", math.Max(1, camera.WorldScalarToScreen(cmd.Width)))
	c.ctx.Call("stroke")
}

func (c *Canvas2D) drawArc(cmd render.DrawArc, camera render.Camera) {
	if cmd.Color.A <= 0 || cmd.Width <= 0 {
		return
	}

	center := camera.WorldToScreen(cmd.Center)
	radius := camera.WorldScalarToScreen(cmd.Radius)
	if radius <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("arc", center.X, center.Y, radius, cmd.StartAngle, cmd.EndAngle)
	c.ctx.Set("strokeStyle", cssColor(cmd.Color))
	c.ctx.Set("lineWidth", math.Max(1, camera.WorldScalarToScreen(cmd.Width)))
	c.ctx.Call("stroke")
}

func (c *Canvas2D) drawText(cmd render.DrawText, camera render.Camera) {
	if cmd.Text == "" || cmd.Color.A <= 0 {
		return
	}

	size := cmd.Size
	if size <= 0 {
		size = 13
	}
	screenSize := camera.WorldScalarToScreen(size)
	if screenSize <= 0 {
		return
	}

	pos := camera.WorldToScreen(cmd.Position)
	c.ctx.Set("fillStyle", cssColor(cmd.Color))
	c.ctx.Set("font", fmt.Sprintf("%gpx monospace", screenSize))
	c.ctx.Set("textAlign", canvasTextAlign(cmd.Align))
	c.ctx.Set("textBaseline", canvasTextBaseline(cmd.Baseline))

	if cmd.Rotation == 0 {
		c.ctx.Call("fillText", cmd.Text, pos.X, pos.Y)
		return
	}

	c.ctx.Call("translate", pos.X, pos.Y)
	c.ctx.Call("rotate", cmd.Rotation)
	c.ctx.Call("fillText", cmd.Text, 0, 0)
}

func (c *Canvas2D) drawPolygon(cmd render.DrawPolygon, camera render.Camera) {
	if len(cmd.Points) < 3 {
		return
	}

	first := camera.WorldToScreen(cmd.Points[0])
	c.ctx.Call("beginPath")
	c.ctx.Call("moveTo", first.X, first.Y)
	for _, point := range cmd.Points[1:] {
		screen := camera.WorldToScreen(point)
		c.ctx.Call("lineTo", screen.X, screen.Y)
	}
	c.ctx.Call("closePath")

	if cmd.Fill.A > 0 {
		c.ctx.Set("fillStyle", cssColor(cmd.Fill))
		c.ctx.Call("fill")
	}
	if cmd.Stroke.A > 0 && cmd.StrokeWidth > 0 {
		c.ctx.Set("strokeStyle", cssColor(cmd.Stroke))
		c.ctx.Set("lineWidth", camera.WorldScalarToScreen(cmd.StrokeWidth))
		c.ctx.Call("stroke")
	}
}

func (c *Canvas2D) applyClip(cmd render.Command, camera render.Camera) {
	clip := commandClip(cmd)
	if clip.Width() <= 0 || clip.Height() <= 0 {
		return
	}

	cMin := camera.WorldToScreen(clip.Min)
	cMax := camera.WorldToScreen(clip.Max)

	x := math.Min(cMin.X, cMax.X)
	y := math.Min(cMin.Y, cMax.Y)
	w := math.Abs(cMax.X - cMin.X)
	h := math.Abs(cMax.Y - cMin.Y)

	if w <= 0 || h <= 0 {
		return
	}

	c.ctx.Call("beginPath")
	c.ctx.Call("rect", x, y, w, h)
	c.ctx.Call("clip")
}

func commandClip(cmd render.Command) math2d.Rect {
	switch cm := cmd.(type) {
	case render.DrawRect:
		return cm.Clip
	case render.DrawCircle:
		return cm.Clip
	case render.DrawPoint:
		return cm.Clip
	case render.DrawLine:
		return cm.Clip
	case render.DrawText:
		return cm.Clip
	case render.DrawArc:
		return cm.Clip
	case render.DrawPolygon:
		return cm.Clip
	case render.DrawImage:
		return cm.Clip
	case render.DrawImageQuad:
		return cm.Clip
	default:
		return math2d.Rect{}
	}
}

func screenRect(rect math2d.Rect, camera render.Camera) (x, y, w, h float64) {
	min := camera.WorldToScreen(rect.Min)
	max := camera.WorldToScreen(rect.Max)

	x = math.Min(min.X, max.X)
	y = math.Min(min.Y, max.Y)
	w = math.Abs(max.X - min.X)
	h = math.Abs(max.Y - min.Y)
	return x, y, w, h
}

func canvasTextAlign(align render.TextAlign) string {
	switch align {
	case render.AlignCenter:
		return "center"
	case render.AlignRight:
		return "right"
	default:
		return "left"
	}
}

func canvasTextBaseline(baseline render.TextBaseline) string {
	switch baseline {
	case render.BaselineTop:
		return "top"
	case render.BaselineMiddle:
		return "middle"
	case render.BaselineBottom:
		return "bottom"
	default:
		return "alphabetic"
	}
}
