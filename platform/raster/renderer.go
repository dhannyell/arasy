package raster

import (
	"fmt"
	"image"
	"image/draw"
	"log/slog"
	"math"
	"slices"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
	"golang.org/x/image/font"
)

// Renderer draws backend-agnostic render commands into an RGBA image.
type Renderer struct {
	Width, Height int
	Camera        render.Camera
	Clear         render.Color

	// Textures resolves TextureIDs to decoded pixels. Nil is valid:
	// every DrawImage then renders the missing-texture placeholder.
	Textures pixel.Registry
	Fonts    typeface.Registry
	Assets   asset.Manifest

	// faceCache memoizes opentype faces per font and pixel size. Lazily
	// initialized; the Renderer is used from a single goroutine (offline
	// Run and tests), so no locking.
	faceCache map[faceKey]font.Face

	particles   *particle.EmitterSet
	diagnostics diagnostic.Once
	log         *slog.Logger

	// spriteBatches/textBatches hold retained batch content published through
	// Replace*Batch. Render-only: never snapshotted or recorded.
	spriteBatches map[render.SpriteBatchID][]render.SpriteInstance
	textBatches   map[render.TextBatchID][]render.TextLabel

	// additiveScratch is the transparent layer additive commands draw into
	// before being summed onto the frame. Lazily sized, reused across
	// commands and frames.
	additiveScratch *image.RGBA

	// postScratch keeps post-effect buffers alive across frames.
	postScratch *postScratch
	viewport    image.Rectangle
	viewportSet bool
}

// drawAdditive renders one command into a transparent scratch image and sums
// the result into dst channel-wise — the same light-accumulating operator the
// GPU backends select as an additive pipeline.
func (r *Renderer) drawAdditive(dst *image.RGBA, draw func(*image.RGBA)) {
	if r.additiveScratch == nil || r.additiveScratch.Bounds() != dst.Bounds() {
		r.additiveScratch = image.NewRGBA(dst.Bounds())
	} else {
		clear(r.additiveScratch.Pix)
	}
	draw(r.additiveScratch)
	addImage(dst, r.additiveScratch)
}

func addImage(dst, src *image.RGBA) {
	for i := 0; i < len(src.Pix); i += 4 {
		if src.Pix[i] == 0 && src.Pix[i+1] == 0 && src.Pix[i+2] == 0 && src.Pix[i+3] == 0 {
			continue
		}
		for j := i; j < i+4; j++ {
			dst.Pix[j] = saturatingAdd(dst.Pix[j], src.Pix[j])
		}
	}
}

// NewTarget allocates an RGBA image sized for this renderer.
func (r *Renderer) NewTarget() *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))
}

// Render draws commands into a new RGBA image.
func (r *Renderer) Render(commands []render.Command) *image.RGBA {
	img := r.NewTarget()
	r.RenderInto(img, commands)
	return img
}

// RenderInto draws commands into dst, which must match this renderer's
// dimensions. The image is cleared with r.Clear before drawing.
func (r *Renderer) RenderInto(dst *image.RGBA, commands []render.Command) {
	r.RenderPassesInto(dst, nil, render.Pass{Camera: r.Camera, Commands: commands})
}

// RenderPassesInto clears dst once, draws extra passes in declaration order,
// and draws the base pass last. Each pass uses its own camera and is clipped
// to that camera's viewport.
func (r *Renderer) RenderPassesInto(dst *image.RGBA, passes []render.Pass, base render.Pass) {
	if dst == nil {
		panic("raster: RenderInto target is nil")
	}

	wantBounds := image.Rect(0, 0, r.Width, r.Height)
	if dst.Bounds() != wantBounds {
		panic(fmt.Sprintf("raster: RenderInto target bounds = %v, want %v", dst.Bounds(), wantBounds))
	}

	fillRect(dst, dst.Bounds(), toRGBA(r.Clear))
	if r.particles != nil {
		r.particles.BeginFrame()
	}

	originalCamera := r.Camera
	originalViewport := r.viewport
	originalViewportSet := r.viewportSet
	defer func() {
		r.Camera = originalCamera
		r.viewport = originalViewport
		r.viewportSet = originalViewportSet
	}()

	var claimedTargets []render.TextureID
	hasScreenPass := false
	for _, pass := range passes {
		if pass.Target != 0 {
			if slices.Contains(claimedTargets, pass.Target) {
				r.diagnostics.Log(r.log, slog.LevelWarn,
					diagnostic.Key{Code: "render.pass_target_duplicate", ID: uint64(pass.Target)},
					"raster: target already has a pass owner this frame; pass skipped", "target", pass.Target)
				continue
			}
			if r.Assets.IsDynamic(pass.Target) {
				claimedTargets = append(claimedTargets, pass.Target)
			}
			target, ok := r.passTarget(pass.Target)
			if !ok {
				continue
			}
			clear(target.Pix)
			r.renderPass(target, pass, false, false)
			continue
		}
		hasScreenPass = true
		r.renderPass(dst, pass, false, false)
	}
	r.renderPass(dst, base, true, hasScreenPass)
}

// passTarget resolves the CPU storage of a dynamic texture that a pass may
// overwrite. Static and unknown textures stay source-only by contract.
func (r *Renderer) passTarget(id render.TextureID) (*image.RGBA, bool) {
	if !r.Assets.IsDynamic(id) {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "render.pass_target_invalid", ID: uint64(id)},
			"raster: pass target is not a declared dynamic texture; pass skipped", "target", id)
		return nil, false
	}
	target := r.Textures.Lookup(id)
	if target == nil {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "render.pass_target_unavailable", ID: uint64(id)},
			"raster: pass target has no pixel storage; pass skipped", "target", id)
		return nil, false
	}
	return target, true
}

func (r *Renderer) renderPass(dst *image.RGBA, pass render.Pass, base, preserve bool) {
	pass = pass.ResolveViewport(frameRect(dst.Bounds()))
	r.Camera = pass.Camera
	r.viewport = viewportBounds(pass.Camera.Viewport, dst.Bounds())
	r.viewportSet = true

	if base {
		if chain, ok := lastPostChain(pass.Commands); ok {
			r.renderPostChain(dst, pass.Commands, chain, preserve)
			return
		}
	}
	r.renderCommands(dst, pass.Commands)
}

func frameRect(bounds image.Rectangle) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: float64(bounds.Min.X), Y: float64(bounds.Min.Y)},
		math2d.Vec2{X: float64(bounds.Dx()), Y: float64(bounds.Dy())},
	)
}

func viewportBounds(viewport math2d.Rect, bounds image.Rectangle) image.Rectangle {
	return image.Rect(
		int(math.Floor(viewport.Min.X)),
		int(math.Floor(viewport.Min.Y)),
		int(math.Ceil(viewport.Max.X)),
		int(math.Ceil(viewport.Max.Y)),
	).Intersect(bounds)
}

// renderCommands is the single dispatch point into the sink: it attaches
// per-layer parallax camera state when the camera declares parallax and
// restores the base camera afterwards.
func (r *Renderer) renderCommands(dst *image.RGBA, commands []render.Command) {
	sink := rasterSink{renderer: r, image: dst}
	if len(r.Camera.Parallax) == 0 {
		render.Each(commands, sink)
		return
	}

	baseCamera := r.Camera
	defer func() { r.Camera = baseCamera }()

	sink.layers = &rasterLayerState{renderer: r, base: baseCamera}
	render.Each(commands, sink)
}

func lastPostChain(commands []render.Command) (render.PostChain, bool) {
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

func postChainPartitions(commands []render.Command, belowLayer int) (below, above []render.Command) {
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

func (r *Renderer) renderPostChain(dst *image.RGBA, commands []render.Command, chain render.PostChain, preserve bool) {
	below, above := postChainPartitions(commands, chain.BelowLayer)
	if r.postScratch == nil {
		r.postScratch = &postScratch{}
	}
	// The clear fill overwrites every pixel, so the frame-to-frame reuse of
	// the scene image cannot leak previous content.
	r.postScratch.scene = ensureImage(r.postScratch.scene, dst.Bounds())
	scene := r.postScratch.scene
	fillRect(scene, scene.Bounds(), toRGBA(r.Clear))
	if preserve {
		draw.Draw(scene, scene.Bounds(), dst, dst.Bounds().Min, draw.Src)
	}
	r.renderCommands(scene, below)
	applyPostChain(scene, chain, r.Camera, r.postScratch)
	draw.Draw(dst, dst.Bounds(), scene, scene.Bounds().Min, draw.Src)
	r.renderCommands(dst, above)
}

type rasterSink struct {
	renderer *Renderer
	image    *image.RGBA
	layers   *rasterLayerState
}

type rasterLayerState struct {
	renderer    *Renderer
	base        render.Camera
	active      int
	initialized bool
}

func (s *rasterLayerState) use(layer int) {
	if s.initialized && s.active == layer {
		return
	}
	s.renderer.Camera = s.base.ForLayer(layer)
	s.active = layer
	s.initialized = true
}

func (s rasterSink) prepareLayer(layer int) {
	if s.layers != nil {
		s.layers.use(layer)
	}
}

// compose routes a command to the frame directly or, when the command asks
// for additive blending, through the scratch layer that turns the
// source-over rasterizers into a light-accumulating draw.
func (s rasterSink) compose(mode render.Blend, draw func(*image.RGBA)) {
	if mode == render.BlendAdditive {
		s.renderer.drawAdditive(s.image, draw)
		return
	}
	draw(s.image)
}

func (s rasterSink) VisitRect(command render.DrawRect) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawRect(img, command) })
}

func (s rasterSink) VisitCircle(command render.DrawCircle) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawCircle(img, command) })
}

func (s rasterSink) VisitEllipse(command render.DrawEllipse) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawEllipse(img, command) })
}

func (s rasterSink) VisitPoint(command render.DrawPoint) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawPoint(img, command) })
}

func (s rasterSink) VisitLine(command render.DrawLine) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawLine(img, command) })
}

func (s rasterSink) VisitText(command render.DrawText) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawText(img, command) })
}

func (s rasterSink) VisitArc(command render.DrawArc) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawArc(img, command) })
}

func (s rasterSink) VisitPolygon(command render.DrawPolygon) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawPolygon(img, command) })
}

func (s rasterSink) VisitTriangles(command render.DrawTriangles) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawTriangles(img, command) })
}

func (s rasterSink) VisitImage(command render.DrawImage) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawImage(img, command) })
}

func (s rasterSink) VisitImageQuad(command render.DrawImageQuad) {
	s.prepareLayer(command.Layer())
	s.compose(command.BlendMode, func(img *image.RGBA) { s.renderer.drawImageQuad(img, command) })
}

func (s rasterSink) VisitParticles(command render.DrawParticles) {
	s.prepareLayer(command.Layer())
	s.renderer.drawParticles(s.image, command)
}

func (s rasterSink) VisitSpriteBatch(command render.DrawSpriteBatch) {
	s.prepareLayer(command.Layer())
	s.renderer.drawSpriteBatch(s.image, command)
}

func (s rasterSink) VisitTextBatch(command render.DrawTextBatch) {
	s.prepareLayer(command.Layer())
	s.renderer.drawTextBatch(s.image, command)
}

// VisitPostChain is handled by renderPostChain outside the ordinary command
// sink pass.
func (s rasterSink) VisitPostChain(render.PostChain) {}
