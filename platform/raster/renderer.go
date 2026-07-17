package raster

import (
	"image"
	"log/slog"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/internal/diagnostic"
	"github.com/dhannyell/frameforge/particle"
	"github.com/dhannyell/frameforge/render"
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

	particleFrame    uint64
	particleEmitters map[emitterKey]*rasterEmitter
	particleScratch  []particle.LiveParticle
	diagnostics      diagnostic.Once
	log              *slog.Logger
}

// Render draws commands into a new RGBA image.
func (r *Renderer) Render(commands []render.Command) *image.RGBA {
	r.particleFrame++

	img := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))
	fillRect(img, img.Bounds(), toRGBA(r.Clear))

	render.Each(commands, rasterSink{renderer: r, image: img})

	r.pruneParticleEmitters()
	return img
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

func (s rasterSink) VisitImage(command render.DrawImage) {
	s.renderer.drawImage(s.image, command)
}

func (s rasterSink) VisitImageQuad(command render.DrawImageQuad) {
	s.renderer.drawImageQuad(s.image, command)
}

func (s rasterSink) VisitParticles(command render.DrawParticles) {
	s.renderer.drawParticles(s.image, command)
}
