//go:build js && wasm

package wasm

// Canvas2D parity table:
//   - Vignette uses a radial gradient overlay.
//   - ColorGrade uses the browser filter pipeline and a multiply tint.
//   - Pixelate uses a reduced self-copy with imageSmoothingEnabled=false.
//   - LensRain N1 uses the same hash cells as the raster path and draws the
//     same droplet locations; N2 degrades to N1 and logs once.
//   - Bloom, ChromaticAberration, and Shockwave are skipped with one
//     post.effect_unsupported diagnostic because Canvas2D has no equivalent
//     exact fullscreen sampling primitive.
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

func (c *Canvas2D) renderPostFrame(commands []render.Command, camera render.Camera, chain render.PostChain, width, height int) {
	surface := c.ensurePostSurface(width, height)
	below, above := canvasPostPartitions(commands, chain.BelowLayer)
	clearCanvas(surface.ctx, width, height, c.clear)

	oldContext := c.ctx
	c.ctx = surface.ctx
	render.Each(below, canvasSink{canvas: c, camera: camera})
	c.ctx = oldContext

	source := surface.canvas
	sourceCtx := surface.ctx
	destination := c.canvas
	destinationCtx := oldContext
	sourceIsMain := false
	for _, effect := range chain.Effects {
		c.applyCanvasPostEffect(source, destination, destinationCtx, effect, chain.Time, width, height)
		source, destination = destination, source
		sourceCtx, destinationCtx = destinationCtx, sourceCtx
		sourceIsMain = !sourceIsMain
	}
	if !sourceIsMain {
		copyCanvas(source, c.canvas, oldContext, width, height)
	}

	render.Each(above, canvasSink{canvas: c, camera: camera})
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

func (c *Canvas2D) applyCanvasPostEffect(source, destination, destinationCtx js.Value, effect render.PostEffect, time float64, width, height int) {
	switch effect := effect.(type) {
	case render.Vignette:
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasVignette(destinationCtx, effect, width, height)
	case render.ColorGrade:
		c.canvasColorGrade(source, destination, destinationCtx, effect, width, height)
	case render.Pixelate:
		c.canvasPixelate(source, destination, destinationCtx, effect, width, height)
	case render.LensRain:
		copyCanvas(source, destination, destinationCtx, width, height)
		c.canvasLensRain(destinationCtx, effect, time, width, height)
	case render.Bloom, render.ChromaticAberration, render.Shockwave:
		copyCanvas(source, destination, destinationCtx, width, height)
		c.warnUnsupportedPostEffect(effect)
	}
}

func (c *Canvas2D) canvasVignette(ctx js.Value, effect render.Vignette, width, height int) {
	if effect.Strength == 0 {
		return
	}
	strength := math.Max(0, math.Min(1, effect.Strength))
	gradient := ctx.Call("createRadialGradient", width/2, height/2, 0,
		width/2, height/2, math.Hypot(float64(width), float64(height))/math.Sqrt2)
	gradient.Call("addColorStop", 0, "rgba(0,0,0,0)")
	gradient.Call("addColorStop", 1, cssColor(render.Color{R: effect.Color.R, G: effect.Color.G, B: effect.Color.B, A: 1}))
	ctx.Call("save")
	ctx.Set("globalAlpha", strength)
	ctx.Set("fillStyle", gradient)
	ctx.Call("fillRect", 0, 0, width, height)
	ctx.Call("restore")
}

func (c *Canvas2D) canvasColorGrade(source, destination, destinationCtx js.Value, effect render.ColorGrade, width, height int) {
	if effect == (render.ColorGrade{}) {
		copyCanvas(source, destination, destinationCtx, width, height)
		return
	}
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
	if effect.Size <= 0 {
		copyCanvas(source, destination, destinationCtx, width, height)
		return
	}
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
	ctx.Set("globalCompositeOperation", "screen")
	ctx.Set("fillStyle", "rgba(255,255,255,0.32)")
	for cellY := math.Floor(0.0 / size); cellY < math.Ceil(float64(height)/size); cellY++ {
		for cellX := math.Floor(0.0 / size); cellX < math.Ceil(float64(width)/size); cellX++ {
			hash := particle.Hash3([3]uint32{uint32(int32(cellX)), uint32(int32(cellY)), uint32(effect.Seed)})
			h := [3]float64{particle.Float01(hash[0]), particle.Float01(hash[1]), particle.Float01(hash[2])}
			if h[0] >= density {
				continue
			}
			centerX := (cellX + h[1]) * size
			fall := h[0] - time*speed*(0.6+0.8*h[0])
			centerY := (cellY + fall - math.Floor(fall)) * size
			radius := size * (0.12 + 0.08*h[2])
			ctx.Call("beginPath")
			ctx.Call("arc", centerX, centerY, radius, 0, 2*math.Pi)
			ctx.Call("fill")
		}
	}
	ctx.Call("restore")
}

func (c *Canvas2D) warnUnsupportedPostEffect(effect render.PostEffect) {
	name := fmt.Sprintf("%T", effect)
	c.diagnostics.Log(c.log, slog.LevelWarn,
		diagnostic.Key{Code: "post.effect_unsupported"},
		"Canvas2D: post effect approximated or skipped", "effect", name)
}
