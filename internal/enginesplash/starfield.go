package enginesplash

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
)

// Starfield geometry is a single uniform field over this package's
// 1920x1080 design canvas, generously overshooting every edge (each half-
// extent is 1.5x the canvas's own half-width/-height). EngineStarfieldScale
// preserves that overscan on 16:9 viewports and expands only the axis that
// would otherwise leave a letterboxed strip uncovered.
//
// The field draws behind the compiled splash content (engineSplashParticles
// in runtime/driver.go gives it a LayerValue under the logo's), so it needs
// no keep-clear zone around the ring/wordmark the way earlier region-based
// layouts did: the logo's own opaque shapes occlude the stars under them,
// the same way it would occlude a real sky.
const (
	engineStarSize     = 10.0
	engineStarDensity  = 1.4e-4
	engineSkyCenterX   = 960.0  // design space; the canvas's own center
	engineSkyCenterY   = 540.0  // design space; the canvas's own center
	engineSkyExtentX   = 1440.0 // design space half-width (1.5x the canvas's own 960)
	engineSkyExtentY   = 810.0  // design space half-height (1.5x the canvas's own 540)
	engineStarSeedSalt = uint64(2)
)

// engineSkyExtent is the field's design-space half-extent (see
// engineSkyExtentX/Y).
var engineSkyExtent = math2d.Vec2{X: engineSkyExtentX, Y: engineSkyExtentY}

// starCount is how many stars the field holds: its area times its density,
// floored at 3. The field is seeded by a single burst on its first frame.
func starCount() int {
	area := 4 * engineSkyExtent.X * engineSkyExtent.Y
	return max(int(math.Round(area*engineStarDensity)), 3)
}

// EngineStarfieldEffectName is the manifest effect name for the sky field.
const EngineStarfieldEffectName = "__ar_engine_starfield"

// EngineStarfieldEffect declares the sky: a uniform rectangular field of
// untextured points (no manifest entry needed, like EngineSparkEffect) with
// fixed size and a randomized per-star lifetime, so at any moment the field
// holds stars at every point of their fade curve — that reads as a sky,
// rather than a synchronized pulse.
func EngineStarfieldEffect() particle.EffectDecl {
	count := starCount()
	return particle.EffectDecl{
		Texture:    EngineMoteTextureName,
		Blend:      particle.BlendAdditive,
		Capacity:   2*count + 8,
		EmitRate:   float64(count) / 3.0,
		BurstSize:  count,
		Lifetime:   particle.Range{Min: 2.6, Max: 4.2},
		Shape:      particle.EmitRect,
		EmitExtent: engineSkyExtent,
		VelMode:    particle.VelCone,
		Drag:       1,
		SizeStart:  particle.Range{Min: engineStarSize, Max: engineStarSize},
		SizeEnd:    particle.Range{Min: engineStarSize, Max: engineStarSize},
		ColorStart: [4]float64{1, 0.97, 0.9, 0.55},
		ColorEnd:   [4]float64{0.94, 0.92, 0.88, 0},
		FadeIn:     0.08,
	}
}

// EngineStarfieldAnchor returns the sky field's emission center in viewport
// pixels for the given viewport.
func EngineStarfieldAnchor(viewport math2d.Rect) math2d.Vec2 {
	return EngineDesignPoint(viewport, engineSkyCenterX, engineSkyCenterY)
}

// EngineStarfieldScale returns the starfield emission scale for a viewport.
// Each half-extent is 75% of its viewport axis, exactly matching the static
// loading poster's percentage coordinates at every aspect ratio. Particle
// size stays unchanged because EmitScale affects geometry, not the sprite.
func EngineStarfieldScale(viewport math2d.Rect) math2d.Vec2 {
	return math2d.Vec2{
		X: 0.75 * viewport.Width() / engineSkyExtentX,
		Y: 0.75 * viewport.Height() / engineSkyExtentY,
	}
}

// EngineStarfieldSeed returns the static poster's canonical 1920x1080 seed.
// Keeping it viewport-independent makes every generated star occupy the same
// percentage position in the poster and live canvas during their cross-fade.
func EngineStarfieldSeed(math2d.Rect) uint64 {
	return (uint64(1920)<<32 | uint64(1080)) ^ engineStarSeedSalt
}
