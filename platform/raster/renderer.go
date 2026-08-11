package raster

import (
	"fmt"
	"image"
	"image/draw"
	"log/slog"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/diagnostic"
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
	if dst == nil {
		panic("raster: RenderInto target is nil")
	}

	wantBounds := image.Rect(0, 0, r.Width, r.Height)
	if dst.Bounds() != wantBounds {
		panic(fmt.Sprintf("raster: RenderInto target bounds = %v, want %v", dst.Bounds(), wantBounds))
	}

	if r.particles != nil {
		r.particles.BeginFrame()
	}

	fillRect(dst, dst.Bounds(), toRGBA(r.Clear))

	chain, ok := lastPostChain(commands)
	if ok {
		r.renderPostChain(dst, commands, chain)
		return
	}

	r.renderCommands(dst, commands)
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

func (r *Renderer) renderPostChain(dst *image.RGBA, commands []render.Command, chain render.PostChain) {
	below, above := postChainPartitions(commands, chain.BelowLayer)
	if r.postScratch == nil {
		r.postScratch = &postScratch{}
	}
	// The clear fill overwrites every pixel, so the frame-to-frame reuse of
	// the scene image cannot leak previous content.
	r.postScratch.scene = ensureImage(r.postScratch.scene, dst.Bounds())
	scene := r.postScratch.scene
	fillRect(scene, scene.Bounds(), toRGBA(r.Clear))
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
