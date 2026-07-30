package enginesplash

import (
	"math"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
)

const (
	engineStarSize            = 10.0
	engineStarDensity         = 1.4e-4
	engineSkyCenterX          = 960.0
	engineSkyCenterY          = 540.0
	engineSkyExtentX          = 1440.0 // 1.5x the design half-width
	engineSkyExtentY          = 810.0  // 1.5x the design half-height
	engineStarSeedSalt        = uint64(2)
	engineStarfieldSeed       = (uint64(1920)<<32 | uint64(1080)) ^ engineStarSeedSalt
	engineStarfieldEffectName = asset.ReservedNamePrefix + "engine_starfield"
)

var engineSkyExtent = math2d.Vec2{X: engineSkyExtentX, Y: engineSkyExtentY}

func starCount() int {
	area := 4 * engineSkyExtent.X * engineSkyExtent.Y
	return max(int(math.Round(area*engineStarDensity)), 3)
}

// StarfieldEffect returns the effect shared by the runtime splash and
// static poster tooling.
func StarfieldEffect() particle.EffectDecl {
	count := starCount()
	return particle.EffectDecl{
		Texture:    engineMoteTextureName,
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

// StarfieldAnchor returns the field center in viewport pixels.
func StarfieldAnchor(viewport math2d.Rect) math2d.Vec2 {
	return engineDesignPoint(viewport, engineSkyCenterX, engineSkyCenterY)
}

func engineStarfieldScale(viewport math2d.Rect) math2d.Vec2 {
	return math2d.Vec2{
		X: 0.75 * viewport.Width() / engineSkyExtentX,
		Y: 0.75 * viewport.Height() / engineSkyExtentY,
	}
}
