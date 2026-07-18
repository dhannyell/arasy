//go:build js && wasm

// Canvas2D particle fallback. platform/raster defines the exact semantics
// and WebGPU is the real-time backend; this path runs the same
// particle.Emitter simulation but approximates per-particle tinting by
// quantizing the effect's color gradient into a few pre-tinted bitmaps —
// particle RGB always lies on the declared start→end line, so the gradient
// is one-dimensional and the existing tintedBitmap cache absorbs it. An
// adaptive Budget shrinks every emitter's live cap when the frame delta
// drifts past target.

package wasm

import (
	"log/slog"
	"math"
	"syscall/js"

	"github.com/dhannyell/frameforge/internal/diagnostic"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
	"github.com/dhannyell/frameforge/render"
)

const (
	particleColorStops  = 16 // gradient quantization; bounds the tint cache
	particleBudgetFloor = 256
)

func particleEmitScale(scale math2d.Vec2) math2d.Vec2 {
	if scale.X == 0 {
		scale.X = 1
	}
	if scale.Y == 0 {
		scale.Y = 1
	}
	return scale
}

func (c *Canvas2D) drawParticles(cmd render.DrawParticles, camera render.Camera) {
	decl, ok := c.manifest.EffectDecl(cmd.Effect)
	if !ok {
		c.warnMissingParticleEffect(cmd.Effect)
		return
	}
	if c.particles == nil {
		c.particles = particle.NewEmitterSet(
			particle.NewBudget(0.020, particleBudgetFloor, particleBudgetFloor),
		)
	}
	res := c.particles.Draw(
		particle.InstanceKey{Effect: uint32(cmd.Effect), Emitter: uint32(cmd.Emitter)},
		decl,
		particle.Input{Pos: cmd.Pos, Dir: cmd.Dir, Intensity: cmd.Intensity,
			Burst: cmd.Burst, Dt: cmd.Dt, Seed: cmd.Seed},
	)
	if res.BudgetReduced {
		c.diagnostics.Log(
			c.log,
			slog.LevelInfo,
			diagnostic.Key{Code: particle.DiagnosticBudgetReduced},
			"particle budget reduced; canvas2d is the fallback renderer",
		)
	}
	if res.Skipped {
		return
	}

	bitmap, textured := js.Value{}, false
	var texture render.TextureID
	if decl.Texture != "" {
		texture = c.manifest.Texture(decl.Texture)
		bitmap, textured = c.textures[texture], true
	}

	c.ctx.Call("save")
	if decl.Blend == particle.BlendAdditive {
		c.ctx.Set("globalCompositeOperation", "lighter")
	}
	for _, p := range res.Live {
		if textured && bitmap.Truthy() {
			c.drawTexturedParticle(bitmap, texture, decl, p, camera)
		} else {
			c.drawSolidParticle(p, camera, textured)
		}
	}
	c.ctx.Call("restore")
}

func (c *Canvas2D) warnMissingParticleEffect(id render.ParticleEffectID) {
	c.diagnostics.Log(
		c.log,
		slog.LevelWarn,
		diagnostic.Key{Code: particle.DiagnosticEffectMissing, ID: uint64(id)},
		"canvas2d: DrawParticles references unknown effect; command skipped",
		"effect_id", id,
	)
}

// drawTexturedParticle picks the pre-tinted bitmap for the particle's
// quantized gradient stop and lets globalAlpha carry the (exact) alpha,
// including the fade-in envelope.
func (c *Canvas2D) drawTexturedParticle(bitmap js.Value, id render.TextureID, decl particle.EffectDecl, p particle.LiveParticle, camera render.Camera) {
	x, y, w, h, ok := particleScreenRect(p, camera)
	if !ok {
		return
	}

	steps := float64(particleColorStops - 1)
	stop := math.Round(p.Progress*steps) / steps
	tint := render.Color{
		R: math2d.Lerp(decl.ColorStart[0], decl.ColorEnd[0], stop),
		G: math2d.Lerp(decl.ColorStart[1], decl.ColorEnd[1], stop),
		B: math2d.Lerp(decl.ColorStart[2], decl.ColorEnd[2], stop),
		A: 1,
	}
	source := c.tintedBitmap(bitmap, id, tint)

	c.ctx.Set("globalAlpha", p.Color[3])
	if p.Rotation != 0 || p.ScaleX < 0 {
		c.ctx.Call("save")
		c.ctx.Call("translate", x+w/2, y+h/2)
		if p.Rotation != 0 {
			c.ctx.Call("rotate", p.Rotation)
		}
		if p.ScaleX < 0 {
			c.ctx.Call("scale", -1, 1)
		}
		c.ctx.Call("drawImage", source, -w/2, -h/2, w, h)
		c.ctx.Call("restore")
		return
	}
	c.ctx.Call("drawImage", source, x, y, w, h)
}

// drawSolidParticle covers both the declared-untextured effect (exact
// interpolated color) and a texture whose bitmap has not decoded yet
// (magenta placeholder, matching the raster backend).
func (c *Canvas2D) drawSolidParticle(p particle.LiveParticle, camera render.Camera, missing bool) {

	x, y, w, h, ok := particleScreenRect(p, camera)
	if !ok {
		return
	}

	if missing {
		c.ctx.Set("fillStyle", "rgb(255, 0, 255)")
	} else {
		c.ctx.Set("fillStyle", cssColor(render.Color{
			R: p.Color[0], G: p.Color[1], B: p.Color[2], A: p.Color[3],
		}))
	}
	if p.Rotation != 0 || p.ScaleX < 0 {
		c.ctx.Call("save")
		c.ctx.Call("translate", x+w/2, y+h/2)
		if p.Rotation != 0 {
			c.ctx.Call("rotate", p.Rotation)
		}
		if p.ScaleX < 0 {
			c.ctx.Call("scale", -1, 1)
		}
		c.ctx.Call("fillRect", -w/2, -h/2, w, h)
		c.ctx.Call("restore")
		return
	}
	c.ctx.Call("fillRect", x, y, w, h)
}

func particleScreenRect(p particle.LiveParticle, camera render.Camera) (x, y, w, h float64, ok bool) {
	width := p.Size * math.Abs(p.ScaleX)
	halfWidth := width / 2
	halfHeight := p.Size / 2
	x, y, w, h = screenRect(math2d.NewRect(
		math2d.Vec2{X: p.Pos.X - halfWidth, Y: p.Pos.Y - halfHeight},
		math2d.Vec2{X: width, Y: p.Size},
	), camera)
	return x, y, w, h, w > 0 && p.Size > 0
}
