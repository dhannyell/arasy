package enginesplash

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
)

// Crux (the Southern Cross) is the constellation Arasy's Guarani name is
// read against, placed in this package's 1920x1080 design canvas (see
// starfield.go). Each star is its own continuously-respawning point
// particle effect, tinted per its spectral class, whose per-frame Intensity
// (EngineCruxIntensity) carries the twinkle and relative magnitude.
type cruxStar struct {
	Name    string
	DX, DY  float64 // degrees, +x = screen right, +y = screen down
	Mag     float64 // apparent visual magnitude
	R, G, B float64 // spectral-class tint, 0..1
}

var engineCruxStars = []cruxStar{
	{Name: "acrux", DX: -0.25, DY: 3.10, Mag: 0.77, R: 0.70, G: 0.80, B: 1.00},
	{Name: "mimosa", DX: -2.96, DY: -0.31, Mag: 1.25, R: 0.72, G: 0.82, B: 1.00},
	{Name: "gacrux", DX: -0.89, DY: -2.89, Mag: 1.63, R: 1.00, G: 0.74, B: 0.56},
	{Name: "imai", DX: 1.11, DY: -1.25, Mag: 2.79, R: 0.75, G: 0.84, B: 1.00},
	{Name: "ginan", DX: 0.33, DY: 0.40, Mag: 3.59, R: 1.00, G: 0.85, B: 0.68},
}

const (
	// Upper-right quadrant, clear of the mark's orbit and of the wordmark.
	engineCruxCenterX = 1540.0
	engineCruxCenterY = 300.0
	engineCruxScale   = 26.0 // design units per degree
	engineCruxSize    = engineStarSize * 1.35
	// Peak brightness of the brightest member (Acrux); a constellation is
	// found because its stars are the bright ones, so it sits above the
	// surrounding field rather than dissolving into it.
	engineCruxPeak = 1.0
)

// engineCruxBrightness maps magnitude to relative brightness, damped (0.22
// instead of the true 2.512^-dmag) so every member stays readable instead of
// the dimmest (Ginan, mag 3.59) nearly vanishing next to Acrux.
func engineCruxBrightness(mag float64) float64 {
	return math.Pow(2.512, -0.22*(mag-engineCruxStars[0].Mag))
}

// EngineCruxEffectName is the manifest effect name for one Crux star.
func EngineCruxEffectName(name string) string {
	return "__ar_engine_crux_" + name
}

// EngineCruxNames returns the star name keys (for use with
// EngineCruxEffectName/EngineCruxAnchor/EngineCruxIntensity) in a stable
// order.
func EngineCruxNames() []string {
	names := make([]string, len(engineCruxStars))
	for i, star := range engineCruxStars {
		names[i] = star.Name
	}
	return names
}

// EngineCruxEffects declares one particle effect per Crux star: a small,
// continuously-respawning point tinted per its spectral class. No manifest
// entry or texture is needed.
func EngineCruxEffects() map[string]particle.EffectDecl {
	effects := make(map[string]particle.EffectDecl, len(engineCruxStars))
	for _, star := range engineCruxStars {
		effects[EngineCruxEffectName(star.Name)] = particle.EffectDecl{
			Texture:    EngineMoteTextureName,
			Blend:      particle.BlendAdditive,
			Capacity:   6,
			EmitRate:   8,
			Lifetime:   particle.Range{Min: 0.6, Max: 1.0},
			Shape:      particle.EmitPoint,
			VelMode:    particle.VelCone,
			SizeStart:  particle.Range{Min: engineCruxSize, Max: engineCruxSize},
			SizeEnd:    particle.Range{Min: engineCruxSize, Max: engineCruxSize},
			ColorStart: [4]float64{star.R, star.G, star.B, 1},
			ColorEnd:   [4]float64{star.R, star.G, star.B, 0},
			FadeIn:     0.2,
		}
	}
	return effects
}

// EngineCruxAnchor returns a star's fixed position in viewport pixels.
func EngineCruxAnchor(viewport math2d.Rect, name string) math2d.Vec2 {
	for _, star := range engineCruxStars {
		if star.Name == name {
			x := engineCruxCenterX + star.DX*engineCruxScale
			y := engineCruxCenterY + star.DY*engineCruxScale
			return EngineDesignPoint(viewport, x, y)
		}
	}
	return math2d.Vec2{}
}

// EngineCruxIntensity is a star's brightness at elapsed seconds into the
// segment: its magnitude-derived brightness modulated by a slow per-star
// twinkle, phase-shifted by index so the constellation never pulses in
// unison.
func EngineCruxIntensity(name string, elapsed float64) float64 {
	for index, star := range engineCruxStars {
		if star.Name != name {
			continue
		}
		twinkle := 1 + 0.12*math.Sin(2*math.Pi*(elapsed*0.55+float64(index)*0.37))
		return engineCruxPeak * engineCruxBrightness(star.Mag) * twinkle
	}
	return 0
}
