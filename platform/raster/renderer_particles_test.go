package raster

import (
	"bytes"
	"image/color"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const particleBehaviorManifest = `{
	"version": 1,
	"effects": [{
		"name": "test",
		"capacity": 8,
		"emit_rate": 4,
		"burst_size": 1,
		"lifetime": [10, 10],
		"speed": [2, 2],
		"direction": 0,
		"spread": 0,
		"shape": "point",
		"velocity_mode": "cone",
		"drag": 1,
		"size_start": [4, 4],
		"size_end": [4, 4],
		"color_start": [1, 1, 1, 1],
		"color_end": [1, 1, 1, 1]
	}]
}`

func TestParticleMissingEffectWarnsOnlyOnce(t *testing.T) {
	var logs bytes.Buffer
	renderer := &Renderer{
		Width:  16,
		Height: 16,
		log: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})),
	}
	command := render.DrawParticles{Effect: 99, Emitter: 1}

	renderer.Render([]render.Command{command})
	renderer.Render([]render.Command{command})

	const message = "DrawParticles references unknown effect"
	if got := strings.Count(logs.String(), message); got != 1 {
		t.Fatalf("warning count = %d, want 1; logs:\n%s", got, logs.String())
	}
}

// TestParticleDrawProducesForegroundPixels is a shallow integration check:
// a valid DrawParticles command must reach particle.EmitterSet and paint
// something. The lifecycle itself (dedupe, TTL, budget) is covered black-box
// in particle/emitterset_test.go against the shared module.
func TestParticleDrawProducesForegroundPixels(t *testing.T) {
	renderer, effect := newParticleBehaviorRenderer(t)
	command := particleBehaviorCommand(effect, 1, 0.25)
	command.Intensity = 1

	img := renderer.Render([]render.Command{command})

	background := color.RGBA{}
	foreground := false
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if c := img.RGBAAt(x, y); c != background {
				foreground = true
				break
			}
		}
	}
	if !foreground {
		t.Fatal("DrawParticles command produced no foreground pixels")
	}
}

func newParticleBehaviorRenderer(t *testing.T) (*Renderer, render.ParticleEffectID) {
	t.Helper()

	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(particleBehaviorManifest)},
	})
	if err != nil {
		t.Fatalf("load particle behavior manifest: %v", err)
	}

	renderer := &Renderer{
		Width:  64,
		Height: 64,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
		},
		Clear:  render.RGBA(0, 0, 0, 1),
		Assets: manifest,
	}
	return renderer, manifest.Effect("test")
}

func particleBehaviorCommand(effect render.ParticleEffectID, emitter render.EmitterID, dt float64) render.DrawParticles {
	return render.DrawParticles{
		Effect:  effect,
		Emitter: emitter,
		Pos:     math2d.Vec2{X: 20, Y: 20},
		Dt:      dt,
		Seed:    42,
	}
}
