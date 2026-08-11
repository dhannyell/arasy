//go:build js && wasm

package wasm

// Canvas2D parity table:
//   - Vignette uses a radial gradient overlay.
//   - ColorGrade uses the browser filter pipeline and a multiply tint.
//   - Pixelate uses a reduced self-copy with imageSmoothingEnabled=false.
//   - LensRain draws the same hash layout as the reference paths: two
//     static droplet layers plus a splash layer whose droplets appear and
//     gradually erode away. Refraction (N2) degrades to flat shaded drops
//     and logs once.
//   - Bloom, ChromaticAberration, and Shockwave are skipped with one
//     post.effect_unsupported diagnostic because Canvas2D has no equivalent
//     exact fullscreen sampling primitive.
//   - Blur uses ctx.filter = "blur(Npx)", picking N from Strength and Passes
//     so it visually tracks the Kawase result without reimplementing it.
//   - Fade is a solid fillRect at globalAlpha = Amount.
//   - CircleWipe fills the screen with an evenodd path (full rect minus a
//     circle hole), so the circle interior stays untouched.
//   - Dissolve batches every covered cell into one Path2D of rects and fills
//     it once, instead of one fillRect per cell.
//   - FilmGrain draws a small cached noise tile, tiled with a per-tick
//     offset; the tile is generated once per seed, never per frame.
//   - Scanlines fills hard half-period bands instead of the sin^2 profile.
//   - RadialBlur draws the source three extra times scaled around the focus
//     at decreasing alpha, instead of per-pixel streak sampling.
//
// Raster is the semantic reference. WebGPU and native mirror it exactly in
// WGSL; Canvas2D is intentionally an approximation or skip for the entries
// listed above.

import (
	"fmt"
	"log/slog"
	"math"
	"syscall/js"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

type canvasPostSurface struct {
	canvas        js.Value
	ctx           js.Value
	width, height int
}

func lastCanvasPostChain(commands []render.Command) (render.PostChain, bool) {
	var chain render.PostChain
	found := false
	for _, command := range commands {
		if candidate, ok := command.(render.PostChain); ok {
			chain = candidate
			found = true
		}
	}
	return chain, found
}

func canvasPostPartitions(commands []render.Command, belowLayer int) (below, above []render.Command) {
	for _, command := range commands {
		if _, ok := command.(render.PostChain); ok {
			continue
		}
		if belowLayer == 0 || command.Layer() < belowLayer {
			below = append(below, command)
			continue
		}
		above = append(above, command)
	}
	return below, above
}

func (c *Canvas2D) ensurePostSurface(width, height int) *canvasPostSurface {
	if c.postSurface != nil && c.postSurface.width == width && c.postSurface.height == height {
		return c.postSurface
	}

	canvas := js.Global().Get("document").Call("createElement", "canvas")
	canvas.Set("width", width)
	canvas.Set("height", height)
	c.postSurface = &canvasPostSurface{
		canvas: canvas,
		ctx:    canvas.Call("getContext", "2d"),
		width:  width,
		height: height,
	}
	return c.postSurface
}

func (c *Canvas2D) renderPostFrame(commands []render.Command, camera render.Camera, chain render.PostChain, width, height int, preserve bool) {
	surface := c.ensurePostSurface(width, height)
	below, above := canvasPostPartitions(commands, chain.BelowLayer)
	clearCanvas(surface.ctx, width, height, c.clear)
	if preserve {
		copyCanvas(c.canvas, surface.canvas, surface.ctx, width, height)
	}

	oldContext := c.ctx
	c.ctx = surface.ctx
	c.renderCommands(below, camera)
	c.ctx = oldContext

	source := surface.canvas
	sourceCtx := surface.ctx
	destination := c.canvas
	destinationCtx := oldContext
	sourceIsMain := false
	for _, effect := range chain.Effects {
		// A skipped or identity effect keeps the current source instead of
		// paying a fullscreen copy just to swap the pair.
		if !c.applyCanvasPostEffect(source, destination, destinationCtx, effect, camera, chain.Time, width, height) {
			continue
		}
		source, destination = destination, source
		sourceCtx, destinationCtx = destinationCtx, sourceCtx
		sourceIsMain = !sourceIsMain
	}
	if !sourceIsMain {
		copyCanvas(source, c.canvas, oldContext, width, height)
	}

	c.renderCommands(above, camera)
}

func clearCanvas(ctx js.Value, width, height int, clear render.Color) {
	ctx.Call("save")
	ctx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	ctx.Call("clearRect", 0, 0, width, height)
	if clear.A > 0 {
		ctx.Set("fillStyle", cssColor(clear))
		ctx.Call("fillRect", 0, 0, width, height)
	}
	ctx.Call("restore")
}

func copyCanvas(source, destination, destinationCtx js.Value, width, height int) {
	destinationCtx.Call("save")
	destinationCtx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	destinationCtx.Set("globalCompositeOperation", "copy")
	destinationCtx.Set("filter", "none")
	destinationCtx.Call("clearRect", 0, 0, width, height)
	destinationCtx.Call("drawImage", source, 0, 0, width, height)
	destinationCtx.Call("restore")
}

// applyCanvasPostEffect runs one effect from source into destination and
// reports whether destination now holds the frame; false means the effect was
// an identity or a skip and the ping-pong must not swap.
func (c *Canvas2D) applyCanvasPostEffect(source, destination, destinationCtx js.Value, effect render.PostEffect, camera render.Camera, time float64, width, height int) bool {
	switch effect := effect.(type) {
	case render.Vignette:
		if effect.Strength == 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasVignette(destinationCtx, effect, width, height)
		return true
	case render.ColorGrade:
		if effect == (render.ColorGrade{}) {
			return false
		}
		c.canvasColorGrade(source, destination, destinationCtx, effect, width, height)
		return true
	case render.Pixelate:
		if effect.Size <= 0 {
			return false
		}
		c.canvasPixelate(source, destination, destinationCtx, effect, width, height)
		return true
	case render.LensRain:
		if effect.Density <= 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasLensRain(destinationCtx, effect, time, width, height)
		return true
	case render.Blur:
		if math.Max(0, math.Min(1, effect.Strength)) == 0 {
			return false
		}
		c.canvasBlur(source, destination, destinationCtx, effect, width, height)
		return true
	case render.Fade:
		if math.Max(0, math.Min(1, effect.Amount)) == 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasFade(destinationCtx, effect, width, height)
		return true
	case render.CircleWipe:
		if effect.Progress <= 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasCircleWipe(destinationCtx, effect, camera, width, height)
		return true
	case render.Dissolve:
		if math.Max(0, math.Min(1, effect.Amount)) <= 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasDissolve(destinationCtx, effect, width, height)
		return true
	case render.FilmGrain:
		if effect.Intensity == 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasFilmGrain(destinationCtx, effect, time, width, height)
		return true
	case render.Scanlines:
		if math.Max(0, math.Min(1, effect.Intensity)) == 0 &&
			math.Max(0, math.Min(1, effect.MaskStrength)) == 0 {
			return false
		}
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasScanlines(destinationCtx, effect, width, height)
		return true
	case render.RadialBlur:
		if math.Max(0, math.Min(1, effect.Strength)) == 0 {
			return false
		}
		c.canvasRadialBlur(source, destination, destinationCtx, effect, camera, width, height)
		return true
	case render.Bloom, render.ChromaticAberration, render.Shockwave:
		c.warnUnsupportedPostEffect(effect)
		return false
	default:
		return false
	}
}

// canvasScanlines batches every dark line into one Path2D of rects and fills
// it once at globalAlpha = intensity, approximating the sin^2 darkening with
// hard half-period bands.
func (c *Canvas2D) canvasScanlines(ctx js.Value, effect render.Scanlines, width, height int) {
	intensity := math.Max(0, math.Min(1, effect.Intensity))
	period := effect.Thickness
	if period <= 0 {
		period = 3
	}

	path2D := js.Global().Get("Path2D")
	path := path2D.New()
	for y := period / 2; y < float64(height); y += period {
		path.Call("rect", 0, y, width, period/2)
	}

	ctx.Call("save")
	if intensity > 0 {
		ctx.Set("globalAlpha", intensity)
		ctx.Set("fillStyle", "#000000")
		ctx.Call("fill", path)
	}
	mask := math.Max(0, math.Min(1, effect.MaskStrength))
	if mask > 0 {
		ctx.Set("globalAlpha", 1)
		ctx.Set("globalCompositeOperation", "multiply")
		ctx.Set("fillStyle", ctx.Call("createPattern", c.grilleTile(mask), "repeat"))
		ctx.Call("fillRect", 0, 0, width, height)
	}
	ctx.Call("restore")
}

// grilleTile builds a cached 3x1 multiply tile: each column keeps one triad
// channel and dims the other two by mask.
func (c *Canvas2D) grilleTile(mask float64) js.Value {
	if !c.postGrilleTile.IsUndefined() && c.postGrilleTileKey == mask {
		return c.postGrilleTile
	}
	tile := js.Global().Get("document").Call("createElement", "canvas")
	tile.Set("width", 3)
	tile.Set("height", 1)
	tileCtx := tile.Call("getContext", "2d")
	dim := byte((1 - mask) * 255)
	columns := [3][3]byte{{255, dim, dim}, {dim, 255, dim}, {dim, dim, 255}}
	for x, rgb := range columns {
		tileCtx.Set("fillStyle", fmt.Sprintf("rgb(%d,%d,%d)", rgb[0], rgb[1], rgb[2]))
		tileCtx.Call("fillRect", x, 0, 1, 1)
	}
	c.postGrilleTile = tile
	c.postGrilleTileKey = mask
	return tile
}

// canvasRadialBlur approximates the streak with three extra draws of the
// source scaled around the focus point at decreasing alpha, a classic zoom
// blur trick that avoids per-pixel sampling.
func (c *Canvas2D) canvasRadialBlur(source, destination, destinationCtx js.Value, effect render.RadialBlur, camera render.Camera, width, height int) {
	strength := math.Max(0, math.Min(1, effect.Strength))
	center := camera.WorldToScreen(effect.Center)

	copyCanvas(source, destination, destinationCtx, width, height)
	destinationCtx.Call("save")
	for i := 1; i <= 3; i++ {
		scale := 1 + strength*0.05*float64(i)
		destinationCtx.Set("globalAlpha", 0.35/float64(i))
		destinationCtx.Call("setTransform", scale, 0, 0, scale,
			center.X*(1-scale), center.Y*(1-scale))
		destinationCtx.Call("drawImage", source, 0, 0, width, height)
	}
	destinationCtx.Call("restore")
}

// canvasBlur approximates the Kawase pyramid with the browser's own blur
// filter; N tracks Strength and Passes so heavier chains read as heavier
// blur without reimplementing the pyramid on the CPU.
func (c *Canvas2D) canvasBlur(source, destination, destinationCtx js.Value, effect render.Blur, width, height int) {
	strength := math.Max(0, math.Min(1, effect.Strength))
	passes := effect.Passes
	if passes == 0 {
		passes = 3
	}
	passes = max(1, min(5, passes))
	radius := strength * float64(passes) * 2.5

	destinationCtx.Call("save")
	destinationCtx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	destinationCtx.Set("globalCompositeOperation", "copy")
	destinationCtx.Set("filter", fmt.Sprintf("blur(%gpx)", radius))
	destinationCtx.Call("clearRect", 0, 0, width, height)
	destinationCtx.Call("drawImage", source, 0, 0, width, height)
	destinationCtx.Set("filter", "none")
	destinationCtx.Call("restore")
}

// canvasFade fills the destination with Color at globalAlpha = Amount,
// approximating the linear lerp raster performs.
func (c *Canvas2D) canvasFade(ctx js.Value, effect render.Fade, width, height int) {
	ctx.Call("save")
	ctx.Set("globalAlpha", math.Max(0, math.Min(1, effect.Amount)))
	ctx.Set("fillStyle", cssColor(render.Color{R: effect.Color.R, G: effect.Color.G, B: effect.Color.B, A: 1}))
	ctx.Call("fillRect", 0, 0, width, height)
	ctx.Call("restore")
}

// canvasCircleWipe fills everything outside the visible circle with Color,
// using an evenodd path (full rect minus a circle hole) so the interior
// stays untouched. Softness is not feathered here; Canvas2D has no cheap
// per-pixel gradient ring, so the edge is hard — an accepted approximation.
func (c *Canvas2D) canvasCircleWipe(ctx js.Value, effect render.CircleWipe, camera render.Camera, width, height int) {
	center := camera.WorldToScreen(effect.Center)
	corners := [4][2]float64{{0, 0}, {float64(width), 0}, {0, float64(height)}, {float64(width), float64(height)}}
	farthest := 0.0
	for _, corner := range corners {
		d := math.Hypot(corner[0]-center.X, corner[1]-center.Y)
		if d > farthest {
			farthest = d
		}
	}
	radius := farthest * (1 - math.Max(0, math.Min(1, effect.Progress)))

	path2D := js.Global().Get("Path2D")
	path := path2D.New()
	path.Call("rect", 0, 0, width, height)
	path.Call("moveTo", center.X+radius, center.Y)
	path.Call("arc", center.X, center.Y, radius, 0, 2*math.Pi)

	ctx.Call("save")
	ctx.Set("fillStyle", cssColor(render.Color{R: effect.Color.R, G: effect.Color.G, B: effect.Color.B, A: 1}))
	ctx.Call("fill", path, "evenodd")
	ctx.Call("restore")
}

// canvasDissolve batches every covered cell into one Path2D of rects and
// fills it once, instead of one fillRect call per cell.
func (c *Canvas2D) canvasDissolve(ctx js.Value, effect render.Dissolve, width, height int) {
	amount := math.Max(0, math.Min(1, effect.Amount))
	cellSize := effect.CellSize
	if cellSize <= 0 {
		cellSize = 8
	}

	path2D := js.Global().Get("Path2D")
	path := path2D.New()
	count := 0
	for cellY := 0.0; cellY*cellSize < float64(height); cellY++ {
		for cellX := 0.0; cellX*cellSize < float64(width); cellX++ {
			hash := particle.Hash3([3]uint32{uint32(cellX), uint32(cellY), uint32(effect.Seed)})
			if particle.Float01(hash[0]) >= amount {
				continue
			}
			path.Call("rect", cellX*cellSize, cellY*cellSize, cellSize, cellSize)
			count++
		}
	}
	if count == 0 {
		return
	}

	ctx.Call("save")
	ctx.Set("fillStyle", cssColor(render.Color{R: effect.Color.R, G: effect.Color.G, B: effect.Color.B, A: 1}))
	ctx.Call("fill", path)
	ctx.Call("restore")
}

// canvasFilmGrain draws a small cached noise tile, tiled across the frame
// with a per-tick offset so it animates without regenerating pixels.
func (c *Canvas2D) canvasFilmGrain(ctx js.Value, effect render.FilmGrain, time float64, width, height int) {
	const tileSize = 64
	if c.postGrainTile.IsUndefined() || c.postGrainTileKey != effect.Seed {
		c.postGrainTile = c.buildGrainTile(effect.Seed, tileSize)
		c.postGrainTileKey = effect.Seed
	}

	tick := int(time * 60)
	offsetX := float64(tick * 7 % tileSize)
	offsetY := float64(tick * 13 % tileSize)
	size := effect.Size
	if size <= 0 {
		size = 1
	}
	ctx.Call("save")
	ctx.Set("globalAlpha", math.Min(1, math.Abs(effect.Intensity)*3))
	ctx.Set("globalCompositeOperation", "overlay")
	pattern := ctx.Call("createPattern", c.postGrainTile, "repeat")
	matrix := js.Global().Get("DOMMatrix").New()
	matrix = matrix.Call("scale", size)
	matrix = matrix.Call("translate", offsetX, offsetY)
	pattern.Call("setTransform", matrix)
	ctx.Set("fillStyle", pattern)
	ctx.Call("fillRect", 0, 0, width, height)
	ctx.Call("restore")
}

// buildGrainTile renders one deterministic noise tile canvas for seed. The
// tile is cached by the caller and regenerated only when the seed changes.
func (c *Canvas2D) buildGrainTile(seed uint64, tileSize int) js.Value {
	tile := js.Global().Get("document").Call("createElement", "canvas")
	tile.Set("width", tileSize)
	tile.Set("height", tileSize)
	tileCtx := tile.Call("getContext", "2d")
	imageData := tileCtx.Call("createImageData", tileSize, tileSize)
	data := imageData.Get("data")
	buffer := make([]byte, tileSize*tileSize*4)
	for y := 0; y < tileSize; y++ {
		for x := 0; x < tileSize; x++ {
			hash := particle.Hash3([3]uint32{uint32(x), uint32(y), uint32(seed)})
			gray := byte(particle.Float01(hash[0]) * 255)
			i := (y*tileSize + x) * 4
			buffer[i], buffer[i+1], buffer[i+2], buffer[i+3] = gray, gray, gray, 255
		}
	}
	js.CopyBytesToJS(data, buffer)
	tileCtx.Call("putImageData", imageData, 0, 0)
	return tile
}

func (c *Canvas2D) canvasVignette(ctx js.Value, effect render.Vignette, width, height int) {
	strength := math.Max(0, math.Min(1, effect.Strength))
	key := vignetteKey{width: width, height: height,
		color: render.Color{R: effect.Color.R, G: effect.Color.G, B: effect.Color.B, A: 1}}
	if c.postVignette.IsUndefined() || c.postVignetteKey != key {
		gradient := ctx.Call("createRadialGradient", width/2, height/2, 0,
			width/2, height/2, math.Hypot(float64(width), float64(height))/math.Sqrt2)
		gradient.Call("addColorStop", 0, "rgba(0,0,0,0)")
		gradient.Call("addColorStop", 1, cssColor(key.color))
		c.postVignette = gradient
		c.postVignetteKey = key
	}
	ctx.Call("save")
	ctx.Set("globalAlpha", strength)
	ctx.Set("fillStyle", c.postVignette)
	ctx.Call("fillRect", 0, 0, width, height)
	ctx.Call("restore")
}

func (c *Canvas2D) canvasColorGrade(source, destination, destinationCtx js.Value, effect render.ColorGrade, width, height int) {
	brightness := math.Max(0, 1+effect.Brightness) * 100
	contrast := math.Max(0, 1+effect.Contrast) * 100
	saturation := math.Max(0, 1+effect.Saturation) * 100
	filter := fmt.Sprintf("brightness(%g%%) contrast(%g%%) saturate(%g%%)", brightness, contrast, saturation)
	destinationCtx.Call("save")
	destinationCtx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	destinationCtx.Set("globalCompositeOperation", "copy")
	destinationCtx.Set("filter", filter)
	destinationCtx.Call("clearRect", 0, 0, width, height)
	destinationCtx.Call("drawImage", source, 0, 0, width, height)
	destinationCtx.Set("filter", "none")
	if effect.Tint != (render.Color{}) {
		destinationCtx.Set("globalCompositeOperation", "multiply")
		destinationCtx.Set("fillStyle", cssColor(render.Color{R: effect.Tint.R, G: effect.Tint.G, B: effect.Tint.B, A: 1}))
		destinationCtx.Call("fillRect", 0, 0, width, height)
	}
	destinationCtx.Call("restore")
}

func (c *Canvas2D) canvasPixelate(source, destination js.Value, destinationCtx js.Value, effect render.Pixelate, width, height int) {
	smallWidth := math.Max(1, math.Ceil(float64(width)/effect.Size))
	smallHeight := math.Max(1, math.Ceil(float64(height)/effect.Size))
	destinationCtx.Call("save")
	destinationCtx.Call("setTransform", 1, 0, 0, 1, 0, 0)
	destinationCtx.Set("globalCompositeOperation", "copy")
	destinationCtx.Set("imageSmoothingEnabled", false)
	destinationCtx.Call("clearRect", 0, 0, width, height)
	destinationCtx.Call("drawImage", source, 0, 0, width, height, 0, 0, smallWidth, smallHeight)
	destinationCtx.Call("drawImage", destination, 0, 0, smallWidth, smallHeight, 0, 0, width, height)
	destinationCtx.Call("restore")
}

func (c *Canvas2D) canvasLensRain(ctx js.Value, effect render.LensRain, time float64, width, height int) {
	density := math.Max(0, math.Min(1, effect.Density))
	if density == 0 {
		return
	}
	size := effect.Size
	if size <= 0 {
		size = 48
	}
	speed := effect.Speed
	if speed == 0 {
		speed = 0.15
	}
	if effect.Refraction > 0 {
		c.warnUnsupportedPostEffect(effect)
	}

	ctx.Call("save")
	ctx.Set("globalCompositeOperation", "source-over")
	c.canvasLensRainLayer(ctx, density, size*0.25, effect.Seed+0x13, width, height, 0.55)
	c.canvasLensRainLayer(ctx, density*0.55, size*0.85, effect.Seed+0x29, width, height, 0.85)
	c.canvasLensRainErode(ctx, density*0.50, size*2.20, speed, effect.Seed+0x41, time, width, height)
	ctx.Call("restore")
}

// canvasLensRainLayer batches every static droplet of one layer into four
// Path2D objects and draws each once, instead of ~13 context calls per
// droplet. The fills use one path per shade, so same-layer overlaps merge as
// a union instead of compositing twice — an accepted Canvas2D approximation.
// Droplet positions and radii mirror the raster and GPU static layers.
func (c *Canvas2D) canvasLensRainLayer(ctx js.Value, density, size float64, seed uint64,
	width, height int, opacity float64) {
	path2D := js.Global().Get("Path2D")
	shadow := path2D.New()
	inner := path2D.New()
	rim := path2D.New()
	highlight := path2D.New()
	// addEllipse moves to the arc's start point first; without the moveTo the
	// subpaths would be joined by stray connecting lines.
	addEllipse := func(path js.Value, centerX, centerY, radiusX, radiusY float64) {
		path.Call("moveTo", centerX+radiusX, centerY)
		path.Call("ellipse", centerX, centerY, radiusX, radiusY, 0, 0, 2*math.Pi)
	}

	drops := 0
	for cellY := 0.0; cellY*size < float64(height); cellY++ {
		for cellX := 0.0; cellX*size < float64(width); cellX++ {
			hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(seed)})
			h := [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
			if h[0] >= density {
				continue
			}
			centerX := (cellX + h[1]) * size
			centerY := (cellY + h[2]) * size
			radius := size * (0.30 + 0.15*h[0])
			addEllipse(shadow, centerX, centerY, radius, radius)
			addEllipse(inner, centerX, centerY, radius*0.78, radius*0.78)
			addEllipse(rim, centerX, centerY, radius*0.92, radius*0.92)
			addEllipse(highlight, centerX-radius*0.28, centerY-radius*0.32, radius*0.16, radius*0.18)
			drops++
		}
	}
	if drops == 0 {
		return
	}

	ctx.Set("globalAlpha", opacity)
	ctx.Set("fillStyle", "rgba(8,16,26,0.32)")
	ctx.Call("fill", shadow)
	ctx.Set("fillStyle", "rgba(190,220,240,0.12)")
	ctx.Call("fill", inner)
	ctx.Set("strokeStyle", "rgba(225,245,255,0.38)")
	ctx.Set("lineWidth", 0.6)
	ctx.Call("stroke", rim)
	ctx.Set("fillStyle", "rgba(255,255,255,0.48)")
	ctx.Call("fill", highlight)
	ctx.Set("globalAlpha", 1)
}

// canvasLensRainErode draws the splash layer: each droplet appears, holds
// and gradually shrinks away, landing at a new position inside its cell on
// every lifecycle — the same erosion math as the reference paths, drawn as
// flat shaded circles.
func (c *Canvas2D) canvasLensRainErode(ctx js.Value, density, size, speed float64, seed uint64,
	time float64, width, height int) {
	path2D := js.Global().Get("Path2D")
	drops := path2D.New()
	highlight := path2D.New()
	count := 0
	for cellY := 0.0; cellY*size < float64(height); cellY++ {
		for cellX := 0.0; cellX*size < float64(width); cellX++ {
			hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(seed)})
			h := [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
			if h[0] >= density {
				continue
			}
			tCycle := time*speed*(0.6+0.8*h[1]) + h[0]
			phase := fractCanvas(tCycle)
			grow := smoothstepCanvas(0, 0.07, phase)
			shrink := 1 - smoothstepCanvas(0.40, 0.97, phase)
			radiusScale := grow * shrink
			if radiusScale <= 0 {
				continue
			}
			cycleSeed := seed + uint64(uint32(int32(math.Floor(tCycle))))
			hash = particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(cycleSeed)})
			h = [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
			radius := size * (0.30 + 0.15*h[0]) * radiusScale
			if radius < 0.5 {
				continue
			}
			centerX := (cellX + h[1]) * size
			centerY := (cellY + h[2]) * size
			drops.Call("moveTo", centerX+radius, centerY)
			drops.Call("arc", centerX, centerY, radius, 0, 2*math.Pi)
			highlight.Call("moveTo", centerX-radius*0.28+radius*0.18, centerY-radius*0.32)
			highlight.Call("ellipse", centerX-radius*0.28, centerY-radius*0.32, radius*0.18, radius*0.20, 0, 0, 2*math.Pi)
			count++
		}
	}
	if count == 0 {
		return
	}

	ctx.Set("fillStyle", "rgba(8,16,26,0.30)")
	ctx.Call("fill", drops)
	ctx.Set("fillStyle", "rgba(255,255,255,0.45)")
	ctx.Call("fill", highlight)
}

func fractCanvas(value float64) float64 {
	return value - math.Floor(value)
}

func smoothstepCanvas(edge0, edge1, value float64) float64 {
	if edge0 == edge1 {
		if value < edge0 {
			return 0
		}
		return 1
	}
	t := math.Max(0, math.Min(1, (value-edge0)/(edge1-edge0)))
	return t * t * (3 - 2*t)
}

func (c *Canvas2D) warnUnsupportedPostEffect(effect render.PostEffect) {
	name := fmt.Sprintf("%T", effect)
	// The ID keys the once-filter per effect kind, so one skipped effect does
	// not silence the warning for a different one.
	c.diagnostics.Log(c.log, slog.LevelWarn,
		diagnostic.Key{Code: "post.effect_unsupported", ID: postEffectDiagnosticID(effect)},
		"Canvas2D: post effect approximated or skipped", "effect", name)
}

func postEffectDiagnosticID(effect render.PostEffect) uint64 {
	switch effect.(type) {
	case render.Vignette:
		return 1
	case render.ColorGrade:
		return 2
	case render.Pixelate:
		return 3
	case render.Bloom:
		return 4
	case render.ChromaticAberration:
		return 5
	case render.Shockwave:
		return 6
	case render.LensRain:
		return 7
	case render.Blur:
		return 8
	case render.Fade:
		return 9
	case render.CircleWipe:
		return 10
	case render.Dissolve:
		return 11
	case render.FilmGrain:
		return 12
	case render.Scanlines:
		return 13
	case render.RadialBlur:
		return 14
	default:
		return 0
	}
}
