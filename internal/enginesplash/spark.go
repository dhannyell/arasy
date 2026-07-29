package enginesplash

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
)

// Engine splash ring geometry, hand-maintained alongside the generated
// content: it mirrors the ring the compiled animation traces (moon-disc/
// ring centered at (960, 540) in the design's 1920x1080 space, ring radius
// 168), used to anchor the spark particle emitter to the same visual ring
// regardless of viewport size.
const (
	designWidth  = 1920.0
	designHeight = 1080.0
	RingCenterX  = 960.0
	RingCenterY  = 540.0
	RingRadius   = 168.0
)

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

// designOffset returns the same letterbox scale and top-left offset the
// compiled animation applies to its own 1920x1080 design space, so any
// design-space point can be placed at the correct on-screen position.
func designOffset(viewport math2d.Rect) (offset math2d.Vec2, scale float64) {
	scale = EngineSplashScale(viewport)
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

// EngineSplashRingAnchor returns the ring's center and radius in viewport
// pixels for the given viewport, applying the same letterbox scale/offset
// the compiled animation uses.
func EngineSplashRingAnchor(viewport math2d.Rect) (center math2d.Vec2, radius float64) {
	_, scale := designOffset(viewport)
	return EngineDesignPoint(viewport, RingCenterX, RingCenterY), RingRadius * scale
}

// EngineSparkEffectName is the name the engine's spark effect is registered
// under (asset.Manifest.WithEffect) — private to this package; runtime.
// Driver holds onto the resolved render.ParticleEffectID instead of
// re-resolving the name.
const EngineSparkEffectName = "__ar_engine_spark"

// EngineSparkEffect declares the warm sparks that drift off the compiled
// animation's orbital ring. It draws no texture (a filled quad, like
// TestGoldenParticlesUntextured) and needs no manifest entry — runtime.
// Driver registers it directly via asset.Manifest.WithEffect.
func EngineSparkEffect() particle.EffectDecl {
	return particle.EffectDecl{
		Texture:    EngineEmberTextureName,
		Blend:      particle.BlendAdditive,
		Capacity:   40,
		EmitRate:   10,
		Lifetime:   particle.Range{Min: 0.6, Max: 1.1},
		Speed:      particle.Range{Min: 6, Max: 16},
		Shape:      particle.EmitRing,
		EmitRadius: RingRadius,
		VelMode:    particle.VelRadial,
		Drag:       0.92,
		SizeStart:  particle.Range{Min: 2, Max: 3.5},
		SizeEnd:    particle.Range{Min: 0, Max: 0},
		// Warm ivory fading toward the ring's gold (#D8B77A), matching the
		// palette the ring stroke itself uses.
		ColorStart: [4]float64{0.953, 0.925, 0.867, 1},
		ColorEnd:   [4]float64{0.847, 0.718, 0.478, 0},
		FadeIn:     0.15,
	}
}

// EngineSparkIntensity is the spark emitter's own fade envelope within the
// segment's MinTime (EngineSplashMinTime): sparks join shortly after the
// ring traces in (matching the ring's own reveal around 7% of the
// animation) and fade out before the segment's own cross-fade would cut
// them mid-emission.
func EngineSparkIntensity(elapsed float64) float64 {
	const fadeIn, fadeOut = 0.2, 0.3
	switch {
	case elapsed < fadeIn:
		return math2d.Clamp(elapsed/fadeIn, 0, 1)
	case elapsed > EngineSplashMinTime-fadeOut:
		return math2d.Clamp((EngineSplashMinTime-elapsed)/fadeOut, 0, 1)
	default:
		return 1
	}
}
