package runtime

import (
	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/internal/enginesplash"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// buildSplashPlaylist assembles the boot splash playlist: the built-in
// engine brand segment (unless disabled), followed by the manifest-declared
// segments in order.
func buildSplashPlaylist(cfg Config) []splashSegment {
	var segments []splashSegment
	if !cfg.DisableDefaultSplash {
		var content enginesplash.Content
		segments = append(segments, splashSegment{
			Content:    content.Append,
			MinTime:    enginesplash.MinTime,
			Fade:       enginesplash.Fade,
			Skippable:  enginesplash.Skippable,
			Background: enginesplash.Background,
			Particles:  enginesplash.NewParticles(cfg.Assets),
			source:     splashSourceDefault,
		})
	}
	for index, decl := range cfg.Assets.Splash() {
		segments = append(segments, splashSegment{
			Texture:   cfg.Assets.Texture(decl.Texture),
			CellW:     decl.CellW,
			CellH:     decl.CellH,
			Columns:   decl.Columns,
			Frames:    decl.Frames,
			Rate:      decl.Rate,
			MinTime:   decl.MinTime,
			Fade:      decl.Fade,
			Skippable: decl.Skippable,
			Particles: manifestSplashParticles(cfg.Assets, index, decl),
		})
	}
	return segments
}

// manifestSplashParticles resolves a game-declared segment's optional
// "effect" reference into a segment-scoped emitter at a fractional viewport
// anchor.
func manifestSplashParticles(assets asset.Manifest, segmentIndex int, decl asset.SplashDecl) func(elapsed, frameDt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
	if decl.Effect == "" {
		return nil
	}
	effect := assets.Effect(decl.Effect)
	// EmitterSet keys live state by (effect, emitter) and only rebuilds an
	// existing key when its seed changes. Scoping both values to manifest
	// order prevents consecutive segments that share an effect from
	// inheriting each other's particles while keeping each segment stable
	// across its own frames.
	emitter := render.EmitterID(segmentIndex + 1)
	seed := uint64(effect)<<32 | uint64(emitter)
	intensity := decl.EffectIntensity
	if intensity == 0 {
		intensity = 1
	}
	return func(elapsed, frameDt float64, viewport math2d.Rect, alpha float64) []render.DrawParticles {
		if effect == 0 || alpha <= 0 {
			return nil
		}
		pos := math2d.Vec2{
			X: viewport.Min.X + viewport.Width()*decl.EffectAnchorX,
			Y: viewport.Min.Y + viewport.Height()*decl.EffectAnchorY,
		}
		return []render.DrawParticles{{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Effect:      effect,
			Emitter:     emitter,
			Pos:         pos,
			Intensity:   intensity * alpha,
			Time:        elapsed,
			Dt:          frameDt,
			Seed:        seed,
		}}
	}
}
