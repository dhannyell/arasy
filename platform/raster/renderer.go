package raster

import (
	"fmt"
	"image"
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

	render.Each(commands, rasterSink{renderer: r, image: dst})
}

type rasterSink struct {
	renderer *Renderer
	image    *image.RGBA
}

func (s rasterSink) VisitRect(command render.DrawRect) {
	s.renderer.drawRect(s.image, command)
}

func (s rasterSink) VisitCircle(command render.DrawCircle) {
	s.renderer.drawCircle(s.image, command)
}

func (s rasterSink) VisitEllipse(command render.DrawEllipse) {
	s.renderer.drawEllipse(s.image, command)
}

func (s rasterSink) VisitPoint(command render.DrawPoint) {
	s.renderer.drawPoint(s.image, command)
}

func (s rasterSink) VisitLine(command render.DrawLine) {
	s.renderer.drawLine(s.image, command)
}

func (s rasterSink) VisitText(command render.DrawText) {
	s.renderer.drawText(s.image, command)
}

func (s rasterSink) VisitArc(command render.DrawArc) {
	s.renderer.drawArc(s.image, command)
}

func (s rasterSink) VisitPolygon(command render.DrawPolygon) {
	s.renderer.drawPolygon(s.image, command)
}

func (s rasterSink) VisitTriangles(command render.DrawTriangles) {
	s.renderer.drawTriangles(s.image, command)
}

func (s rasterSink) VisitImage(command render.DrawImage) {
	s.renderer.drawImage(s.image, command)
}

func (s rasterSink) VisitImageQuad(command render.DrawImageQuad) {
	s.renderer.drawImageQuad(s.image, command)
}

func (s rasterSink) VisitParticles(command render.DrawParticles) {
	s.renderer.drawParticles(s.image, command)
}

func (s rasterSink) VisitSpriteBatch(command render.DrawSpriteBatch) {
	s.renderer.drawSpriteBatch(s.image, command)
}

func (s rasterSink) VisitTextBatch(command render.DrawTextBatch) {
	s.renderer.drawTextBatch(s.image, command)
}
