package enginesplash

import (
	"math"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const (
	designWidth         = 1920.0
	designHeight        = 1080.0
	narrowWidthFactor   = 0.8
	StackedDesignWidth  = 840.0
	StackedDesignHeight = 750.0
)

func LoadAssets(manifest asset.Manifest) asset.Manifest {
	manifest, _ = manifest.WithTexture(EngineMoteTextureName, EngineMoteTexture())
	manifest, _ = manifest.WithEffect(EngineStarfieldEffectName, EngineStarfieldEffect())
	cruxEffects := EngineCruxEffects()
	for _, name := range EngineCruxNames() {
		effectName := EngineCruxEffectName(name)
		manifest, _ = manifest.WithEffect(effectName, cruxEffects[effectName])
	}
	return manifest
}

// EngineSplashParticles combines every particle system the engine's default
// splash draws: the ring spark (anchored to the compiled animation's own
// ring, tracking its letterbox scale, drawn in FRONT of the content since it
// reads as erupting off the ring), the uniform ambient starfield, and the
// five Crux (Southern Cross) stars (both drawn BEHIND the content) — each
// its own emitter, resolved once here against the effects newDriver
// registered via WithEffect, so a missing effect (DisableDefaultSplash games
// that still set Content some other way) simply draws nothing rather than
// panicking.
func EngineSplashParticles(assets asset.Manifest) func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	starfieldEffect := assets.Effect(EngineStarfieldEffectName)

	type namedEffect struct {
		name   string
		effect render.ParticleEffectID
	}
	cruxStars := make([]namedEffect, 0, len(EngineCruxNames()))
	for _, name := range EngineCruxNames() {
		cruxStars = append(cruxStars, namedEffect{name, assets.Effect(EngineCruxEffectName(name))})
	}

	return func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
		commands := make([]render.DrawParticles, 0, 2+len(cruxStars))

		if alpha > 0 {
			contentScale := EngineSplashScale(viewport)
			cruxScale := math2d.Vec2{X: contentScale, Y: contentScale}
			skyScale := EngineStarfieldScale(viewport)
			// EmitterSet only rebuilds an effect's live particles when its
			// Seed changes (particle/emitterset.go); a constant Seed here
			// would leave every already-spawned star exactly where it was
			// emitted (starfield/Crux have no velocity) even after a resize
			// moves their anchor and rescales their spread, so most of the
			// sky would stay stuck at the previous viewport's layout for the
			// rest of that star's multi-second lifetime. Folding the
			// viewport's own size into the seed forces a full reseed — every
			// star in the field respawns at the new layout — the one frame
			// the viewport actually changes, and never lets a redraw at an
			// unchanged size touch it.
			viewportSeed := viewportSeed(viewport)
			if starfieldEffect != 0 {
				commands = append(commands, render.DrawParticles{
					BaseCommand: render.BaseCommand{LayerValue: EngineSkyLayer},
					Effect:      starfieldEffect,
					Emitter:     1,
					Pos:         EngineStarfieldAnchor(viewport),
					EmitScale:   skyScale,
					Intensity:   alpha,
					Time:        elapsed,
					Dt:          dt,
					Burst:       1,
					Seed:        EngineStarfieldSeed(viewport),
				})
			}
			for _, star := range cruxStars {
				if star.effect == 0 {
					continue
				}
				intensity := EngineCruxIntensity(star.name, elapsed) * alpha
				if intensity <= 0 {
					continue
				}
				commands = append(commands, render.DrawParticles{
					BaseCommand: render.BaseCommand{LayerValue: EngineSkyLayer},
					Effect:      star.effect,
					EmitScale:   cruxScale,
					Emitter:     1,
					Pos:         EngineCruxAnchor(viewport, star.name),
					Intensity:   intensity,
					Time:        elapsed,
					Dt:          dt,
					Seed:        uint64(star.effect) ^ viewportSeed,
				})
			}
		}

		return commands
	}
}

// viewportSeed folds a viewport's size into a stable value that only
// changes when the viewport's dimensions do, for XOR-ing into a particle
// effect's Seed so a resize forces EmitterSet to rebuild that effect's live
// particles (see engineSplashParticles). Rounded to the pixel so
// floating-point jitter on an otherwise-unchanged size can't flip it.
func viewportSeed(viewport math2d.Rect) uint64 {
	w := uint64(math.Round(viewport.Width()))
	h := uint64(math.Round(viewport.Height()))
	return w<<32 | h
}

// EngineSplashScale returns the same uniform letterbox scale factor the
// compiled animation applies to its own 1920x1080 design space before
// drawing (see arasySplashScale in the generated file, which is private and
// duplicated here so callers outside this package — the spark emitter in
// runtime/splash.go — can place a design-space point/radius at the correct
// on-screen size without depending on render.DrawParticles.EmitScale, whose
// support currently differs across backends).
func EngineSplashScale(viewport math2d.Rect) float64 {
	return math.Min(viewport.Width()/designWidth, viewport.Height()/designHeight)
}

// StackedViewport letterbox-fits the stacked design canvas into
// the real viewport (plain "meet" fit — the stacked canvas is already
// portrait-shaped, so it needs none of the landscape fit's narrow-width
// boost).
func StackedViewport(viewport math2d.Rect) math2d.Rect {
	w, h := viewport.Width(), viewport.Height()
	scale := math.Min(w/StackedDesignWidth, h/StackedDesignHeight)
	boxWidth := scale * StackedDesignWidth
	minX := viewport.Min.X + (w-boxWidth)/2
	return math2d.NewRect(math2d.Vec2{X: minX, Y: viewport.Min.Y}, math2d.Vec2{X: boxWidth, Y: h})
}

func EngineSplashViewport(viewport math2d.Rect) math2d.Rect {
	w, h := viewport.Width(), viewport.Height()
	widthForScale := w
	if minWidth := h * narrowWidthFactor; widthForScale < minWidth {
		widthForScale = minWidth
	}
	scale := math.Min(widthForScale/designWidth, h/designHeight)
	boxWidth := scale * designWidth
	minX := viewport.Min.X + (w-boxWidth)/2
	return math2d.NewRect(math2d.Vec2{X: minX, Y: viewport.Min.Y}, math2d.Vec2{X: boxWidth, Y: h})
}

// designOffset returns the same letterbox scale and top-left offset the
// compiled animation applies to its own 1920x1080 design space, so any
// design-space point can be placed at the correct on-screen position.
func designOffset(viewport math2d.Rect) (offset math2d.Vec2, scale float64) {
	viewport = EngineSplashViewport(viewport)
	scale = math.Min(viewport.Width()/designWidth, viewport.Height()/designHeight)
	offset = math2d.Vec2{
		X: viewport.Min.X + (viewport.Width()-designWidth*scale)/2,
		Y: viewport.Min.Y + (viewport.Height()-designHeight*scale)/2,
	}
	return offset, scale
}

// EngineDesignPoint maps a point in the engine splash's 1920x1080 design
// space to viewport pixels, applying the same letterbox scale/offset the
// compiled animation uses. Callers anchoring particle emitters to fixed
// design-space positions (starfield regions, Crux stars) use this instead of
// duplicating the letterbox math.
func EngineDesignPoint(viewport math2d.Rect, x, y float64) math2d.Vec2 {
	offset, scale := designOffset(viewport)
	return math2d.Vec2{X: offset.X + x*scale, Y: offset.Y + y*scale}
}
