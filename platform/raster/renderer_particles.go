package raster

import (
	"image"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
	"github.com/dhannyell/frameforge/render"
)

const particleEmitterTTL = 300

type emitterKey struct {
	effect  render.ParticleEffectID
	emitter render.EmitterID
}

type rasterEmitter struct {
	sim      *particle.Emitter
	texture  render.TextureID
	lastSeen uint64
}

func (r *Renderer) drawParticles(img *image.RGBA, cmd render.DrawParticles) {
	decl, ok := r.Assets.EffectDecl(cmd.Effect)
	if !ok {
		return
	}

	if r.particleEmitters == nil {
		r.particleEmitters = make(map[emitterKey]*rasterEmitter)
	}

	key := emitterKey{
		effect:  cmd.Effect,
		emitter: cmd.Emitter,
	}

	state := r.particleEmitters[key]
	if state == nil || state.sim.Seed != cmd.Seed {
		state = &rasterEmitter{
			sim:     particle.NewEmitter(decl, cmd.Seed),
			texture: r.Assets.Texture(decl.Texture),
		}
		r.particleEmitters[key] = state
	}

	state.lastSeen = r.particleFrame

	state.sim.Step(
		cmd.Pos,
		cmd.Dir,
		cmd.Intensity,
		cmd.Burst,
		cmd.Dt,
		decl.Capacity,
	)

	if cap(r.particleScratch) < decl.Capacity {
		r.particleScratch = make([]particle.LiveParticle, 0, decl.Capacity)
	}

	live := state.sim.Alive(r.particleScratch[:0])
	r.particleScratch = live

	var texture *image.RGBA
	if decl.Texture != "" {
		texture = r.Textures.Lookup(state.texture)
	}

	for _, p := range live {
		r.drawParticle(
			img,
			texture,
			p,
			cmd.Clip,
			decl.Blend,
		)
	}
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

	half := p.Size * 0.5
	dest := math2d.NewRect(
		math2d.Vec2{X: p.Pos.X - half, Y: p.Pos.Y - half},
		math2d.Vec2{X: p.Size, Y: p.Size},
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

func (r *Renderer) pruneParticleEmitters() {
	for key, state := range r.particleEmitters {
		if r.particleFrame-state.lastSeen >= particleEmitterTTL {
			delete(r.particleEmitters, key)
		}
	}
}
