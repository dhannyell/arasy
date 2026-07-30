package enginesplash

import (
	"math"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
)

type cruxStar struct {
	name    string
	dx, dy  float64 // degrees in screen axes
	mag     float64 // apparent magnitude
	r, g, b float64 // spectral tint
}

// engineCruxStars describes the Southern Cross in stable brightness order.
var engineCruxStars = [...]cruxStar{
	{name: "acrux", dx: -0.25, dy: 3.10, mag: 0.77, r: 0.70, g: 0.80, b: 1.00},
	{name: "mimosa", dx: -2.96, dy: -0.31, mag: 1.25, r: 0.72, g: 0.82, b: 1.00},
	{name: "gacrux", dx: -0.89, dy: -2.89, mag: 1.63, r: 1.00, g: 0.74, b: 0.56},
	{name: "imai", dx: 1.11, dy: -1.25, mag: 2.79, r: 0.75, g: 0.84, b: 1.00},
	{name: "ginan", dx: 0.33, dy: 0.40, mag: 3.59, r: 1.00, g: 0.85, b: 0.68},
}

const (
	engineCruxCenterX = 1540.0
	engineCruxCenterY = 300.0
	engineCruxScale   = 26.0
	engineCruxSize    = engineStarSize * 1.35
	engineCruxPeak    = 1.0
)

func engineCruxBrightness(mag float64) float64 {
	return math.Pow(2.512, -0.22*(mag-engineCruxStars[0].mag))
}

// CruxEffectName returns the reserved effect name for a Crux star.
func CruxEffectName(name string) string {
	return asset.ReservedNamePrefix + "engine_crux_" + name
}

func cruxEffect(star cruxStar) particle.EffectDecl {
	return particle.EffectDecl{
		Texture:    engineMoteTextureName,
		Blend:      particle.BlendAdditive,
		Capacity:   6,
		EmitRate:   8,
		Lifetime:   particle.Range{Min: 0.6, Max: 1.0},
		Shape:      particle.EmitPoint,
		VelMode:    particle.VelCone,
		SizeStart:  particle.Range{Min: engineCruxSize, Max: engineCruxSize},
		SizeEnd:    particle.Range{Min: engineCruxSize, Max: engineCruxSize},
		ColorStart: [4]float64{star.r, star.g, star.b, 1},
		ColorEnd:   [4]float64{star.r, star.g, star.b, 0},
		FadeIn:     0.2,
	}
}

// CruxNames returns Crux star names in stable brightness order.
func CruxNames() []string {
	names := make([]string, len(engineCruxStars))
	for i, star := range engineCruxStars {
		names[i] = star.name
	}
	return names
}

// CruxEffects returns the declarations used to bake static brand assets.
func CruxEffects() map[string]particle.EffectDecl {
	effects := make(map[string]particle.EffectDecl, len(engineCruxStars))
	for _, star := range engineCruxStars {
		effects[CruxEffectName(star.name)] = cruxEffect(star)
	}
	return effects
}

func cruxAnchor(viewport math2d.Rect, star cruxStar) math2d.Vec2 {
	x := engineCruxCenterX + star.dx*engineCruxScale
	y := engineCruxCenterY + star.dy*engineCruxScale
	return engineDesignPoint(viewport, x, y)
}

// CruxAnchor returns a named star's fixed viewport position.
func CruxAnchor(viewport math2d.Rect, name string) math2d.Vec2 {
	for _, star := range engineCruxStars {
		if star.name == name {
			return cruxAnchor(viewport, star)
		}
	}
	return math2d.Vec2{}
}

func cruxIntensity(star cruxStar, index int, elapsed float64) float64 {
	twinkle := 1 + 0.12*math.Sin(2*math.Pi*(elapsed*0.55+float64(index)*0.37))
	return engineCruxPeak * engineCruxBrightness(star.mag) * twinkle
}

// CruxIntensity returns a named star's animated brightness.
func CruxIntensity(name string, elapsed float64) float64 {
	for index, star := range engineCruxStars {
		if star.name == name {
			return cruxIntensity(star, index, elapsed)
		}
	}
	return 0
}
