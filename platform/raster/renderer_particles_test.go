package raster

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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

func TestParticleEmitterExpiresAfter300FramesWithoutCommand(t *testing.T) {
	renderer, effect := newParticleBehaviorRenderer(t)
	key := emitterKey{effect: effect, emitter: 1}

	renderer.Render([]render.Command{particleBehaviorCommand(effect, 1, 0)})
	if renderer.particleEmitters[key] == nil {
		t.Fatal("particle emitter was not created")
	}

	for range particleEmitterTTL - 1 {
		renderer.Render(nil)
	}
	if renderer.particleEmitters[key] == nil {
		t.Fatalf("particle emitter expired before %d unseen frames", particleEmitterTTL)
	}

	renderer.Render(nil)
	if renderer.particleEmitters[key] != nil {
		t.Fatalf("particle emitter still exists after %d unseen frames", particleEmitterTTL)
	}
}

func TestParticleEmitterDtZeroFreezesLiveParticles(t *testing.T) {
	renderer, effect := newParticleBehaviorRenderer(t)
	key := emitterKey{effect: effect, emitter: 1}

	spawn := particleBehaviorCommand(effect, 1, 0.25)
	spawn.Intensity = 1
	renderer.Render([]render.Command{spawn})

	state := renderer.particleEmitters[key]
	if state == nil {
		t.Fatal("particle emitter was not created")
	}
	before := state.sim.Alive(nil)
	if len(before) == 0 {
		t.Fatal("particle emitter did not spawn a particle")
	}

	frozen := particleBehaviorCommand(effect, 1, 0)
	frozen.Pos = math2d.Vec2{X: 48, Y: 40}
	frozen.Intensity = 1
	frozen.Burst = 1
	renderer.Render([]render.Command{frozen})

	after := state.sim.Alive(nil)
	if !slices.Equal(after, before) {
		t.Fatalf("Dt 0 changed live particles:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestParticleEmitterIDsKeepIndependentPools(t *testing.T) {
	renderer, effect := newParticleBehaviorRenderer(t)

	firstCommand := particleBehaviorCommand(effect, 1, 0.25)
	firstCommand.Intensity = 1
	secondCommand := particleBehaviorCommand(effect, 2, 0.25)
	renderer.Render([]render.Command{firstCommand, secondCommand})

	first := renderer.particleEmitters[emitterKey{effect: effect, emitter: 1}]
	second := renderer.particleEmitters[emitterKey{effect: effect, emitter: 2}]
	if first == nil || second == nil {
		t.Fatalf("particle emitters = (%p, %p), want two live states", first, second)
	}
	if first.sim == second.sim {
		t.Fatal("distinct emitter IDs share the same particle pool")
	}
	if got := len(first.sim.Alive(nil)); got != 1 {
		t.Fatalf("first emitter live particles = %d, want 1", got)
	}
	if got := len(second.sim.Alive(nil)); got != 0 {
		t.Fatalf("second emitter live particles = %d, want 0", got)
	}

	renderer.Render([]render.Command{firstCommand})
	if got := len(second.sim.Alive(nil)); got != 0 {
		t.Fatalf("advancing first emitter changed second emitter to %d live particles", got)
	}
}

func TestParticleEmitterAdvancesOnlyOnceWhenDrawnTwiceInOneFrame(t *testing.T) {
	single, effect := newParticleBehaviorRenderer(t)
	duplicate, _ := newParticleBehaviorRenderer(t)
	command := particleBehaviorCommand(effect, 1, 0.25)
	command.Intensity = 1

	single.Render([]render.Command{command})
	duplicate.Render([]render.Command{command, command})

	key := emitterKey{effect: effect, emitter: 1}
	want := single.particleEmitters[key].sim.Alive(nil)
	got := duplicate.particleEmitters[key].sim.Alive(nil)
	if !slices.Equal(got, want) {
		t.Fatalf("duplicate draw advanced emitter twice:\nsingle:    %+v\nduplicate: %+v", want, got)
	}
}

func TestParticleEmitterFirstSeedWinsWithinFrame(t *testing.T) {
	renderer, effect := newParticleBehaviorRenderer(t)
	first := particleBehaviorCommand(effect, 1, 0.25)
	first.Intensity = 1
	conflicting := first
	conflicting.Seed = 99

	renderer.Render([]render.Command{first, conflicting})

	key := emitterKey{effect: effect, emitter: 1}
	state := renderer.particleEmitters[key]
	if state == nil {
		t.Fatal("particle emitter was not created")
	}
	if got := state.sim.Seed; got != first.Seed {
		t.Fatalf("seed after conflicting command = %d, want first seed %d", got, first.Seed)
	}

	renderer.Render([]render.Command{conflicting})
	state = renderer.particleEmitters[key]
	if got := state.sim.Seed; got != conflicting.Seed {
		t.Fatalf("seed on next frame = %d, want replacement seed %d", got, conflicting.Seed)
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
