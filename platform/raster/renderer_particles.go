package raster

import (
	"image"
	"image/color"
	"log/slog"
	"math"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

// particleWhiteTexture backs effects declared without a texture: blitting
// opaque white through the particle tint yields the interpolated particle
// color, matching the WebGPU backend's white 1×1 binding. A declared
// texture that fails lookup stays nil and renders the magenta placeholder.
var particleWhiteTexture = func() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	return img
}()

func (r *Renderer) drawParticles(img *image.RGBA, cmd render.DrawParticles) {
	decl, ok := r.Assets.EffectDecl(cmd.Effect)
	if !ok {
		r.warnMissingParticleEffect(cmd.Effect)
		return
	}
	if r.particles == nil {
		r.particles = particle.NewEmitterSet(nil)
	}
	res := r.particles.Draw(
		particle.InstanceKey{Effect: uint32(cmd.Effect), Emitter: uint32(cmd.Emitter)},
		decl,
		particle.Input{Pos: cmd.Pos, Dir: cmd.Dir, EmitScale: cmd.EmitScale, Intensity: cmd.Intensity,
			Burst: cmd.Burst, Dt: cmd.Dt, Seed: cmd.Seed},
	)
	if res.Skipped {
		return
	}
	texture := particleWhiteTexture
	if decl.Texture != "" {
		texture = r.Textures.Lookup(r.Assets.Texture(decl.Texture))
	}
	for _, p := range res.Live {
		r.drawParticle(img, texture, p, cmd.Clip, decl.Blend)
	}
}

func (r *Renderer) warnMissingParticleEffect(id render.ParticleEffectID) {
	r.diagnostics.Log(
		r.log,
		slog.LevelWarn,
		diagnostic.Key{Code: particle.DiagnosticEffectMissing, ID: uint64(id)},
		"raster: DrawParticles references unknown effect; command skipped",
		"effect_id", id,
	)
}

func (r *Renderer) drawParticle(
	img *image.RGBA,
	texture *image.RGBA,
	p particle.LiveParticle,
	clip math2d.Rect,
	mode particle.Blend,
) {
	if p.Size <= 0 || p.Color[3] <= 0 {
		return
	}

	halfWidth := p.Size * math.Abs(p.ScaleX) * 0.5
	halfHeight := p.Size * 0.5
	dest := math2d.NewRect(
		math2d.Vec2{X: p.Pos.X - halfWidth, Y: p.Pos.Y - halfHeight},
		math2d.Vec2{X: halfWidth * 2, Y: halfHeight * 2},
	)

	var source image.Rectangle
	if texture != nil {
		source = texture.Bounds()
	}
	r.drawResolvedImage(img, resolvedImageDraw{
		texture:  texture,
		source:   source,
		dest:     dest,
		rotation: p.Rotation,
		clip:     clip,
		tint:     particleTint(p.Color),
		blend:    mode,
	})
}

func particleTint(c [4]float64) tintFactors {
	return tintFactors{
		r: c[0] * c[3],
		g: c[1] * c[3],
		b: c[2] * c[3],
		a: c[3],
	}
}
