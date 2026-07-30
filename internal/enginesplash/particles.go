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
	stackedDesignWidth  = 840.0
	stackedDesignHeight = 750.0
)

// LoadAssets injects the private texture and effects used by the default
// engine splash.
func LoadAssets(manifest asset.Manifest) asset.Manifest {
	manifest, _ = manifest.WithTexture(engineMoteTextureName, engineMoteTexturePNG)
	manifest, _ = manifest.WithEffect(engineStarfieldEffectName, StarfieldEffect())
	for _, star := range engineCruxStars {
		manifest, _ = manifest.WithEffect(CruxEffectName(star.name), cruxEffect(star))
	}
	return manifest
}

// NewParticles resolves the starfield and Crux emitters once and returns
// their per-frame commands.
func NewParticles(assets asset.Manifest) func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	starfieldEffect := assets.Effect(engineStarfieldEffectName)

	type resolvedCrux struct {
		star   cruxStar
		index  int
		effect render.ParticleEffectID
	}
	cruxStars := make([]resolvedCrux, 0, len(engineCruxStars))
	for index, star := range engineCruxStars {
		cruxStars = append(cruxStars, resolvedCrux{
			star:   star,
			index:  index,
			effect: assets.Effect(CruxEffectName(star.name)),
		})
	}

	return func(elapsed, dt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
		if alpha <= 0 {
			return nil
		}

		contentScale := engineSplashScale(viewport)
		cruxScale := math2d.Vec2{X: contentScale, Y: contentScale}
		commands := make([]render.DrawParticles, 0, 1+len(cruxStars))
		if starfieldEffect != 0 {
			commands = append(commands, render.DrawParticles{
				BaseCommand: render.BaseCommand{LayerValue: engineSkyLayer},
				Effect:      starfieldEffect,
				Emitter:     1,
				Pos:         StarfieldAnchor(viewport),
				EmitScale:   engineStarfieldScale(viewport),
				Intensity:   alpha,
				Time:        elapsed,
				Dt:          dt,
				Burst:       1,
				Seed:        engineStarfieldSeed,
			})
		}

		seed := viewportSeed(viewport)
		for _, resolved := range cruxStars {
			if resolved.effect == 0 {
				continue
			}
			intensity := cruxIntensity(resolved.star, resolved.index, elapsed) * alpha
			if intensity <= 0 {
				continue
			}
			commands = append(commands, render.DrawParticles{
				BaseCommand: render.BaseCommand{LayerValue: engineSkyLayer},
				Effect:      resolved.effect,
				EmitScale:   cruxScale,
				Emitter:     1,
				Pos:         cruxAnchor(viewport, resolved.star),
				Intensity:   intensity,
				Time:        elapsed,
				Dt:          dt,
				Seed:        uint64(resolved.effect) ^ seed,
			})
		}

		return commands
	}
}

// viewportSeed changes only when rounded viewport dimensions change, forcing
// fixed-position Crux emitters to respawn after a resize.
func viewportSeed(viewport math2d.Rect) uint64 {
	w := uint64(math.Round(viewport.Width()))
	h := uint64(math.Round(viewport.Height()))
	return w<<32 | h
}

func engineSplashScale(viewport math2d.Rect) float64 {
	return math.Min(viewport.Width()/designWidth, viewport.Height()/designHeight)
}

func stackedViewport(viewport math2d.Rect) math2d.Rect {
	w, h := viewport.Width(), viewport.Height()
	scale := math.Min(w/stackedDesignWidth, h/stackedDesignHeight)
	boxWidth := scale * stackedDesignWidth
	minX := viewport.Min.X + (w-boxWidth)/2
	return math2d.NewRect(math2d.Vec2{X: minX, Y: viewport.Min.Y}, math2d.Vec2{X: boxWidth, Y: h})
}

func engineSplashViewport(viewport math2d.Rect) math2d.Rect {
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

func designOffset(viewport math2d.Rect) (offset math2d.Vec2, scale float64) {
	viewport = engineSplashViewport(viewport)
	scale = math.Min(viewport.Width()/designWidth, viewport.Height()/designHeight)
	offset = math2d.Vec2{
		X: viewport.Min.X + (viewport.Width()-designWidth*scale)/2,
		Y: viewport.Min.Y + (viewport.Height()-designHeight*scale)/2,
	}
	return offset, scale
}

func engineDesignPoint(viewport math2d.Rect, x, y float64) math2d.Vec2 {
	offset, scale := designOffset(viewport)
	return math2d.Vec2{X: offset.X + x*scale, Y: offset.Y + y*scale}
}
