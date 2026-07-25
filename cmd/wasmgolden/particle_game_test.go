package wasmgolden

import (
	"testing"

	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

func TestParticleGoldenFixturePrimesAdvancesThenFreezes(t *testing.T) {
	manifest, err := particleGoldenManifest()
	if err != nil {
		t.Fatalf("particleGoldenManifest: %v", err)
	}

	game := &particleGame{assets: manifest}
	ctx := &runtime.RenderContext{Assets: manifest}
	first := particleCommands(game.Render(ctx))
	second := particleCommands(game.Render(ctx))
	third := particleCommands(game.Render(ctx))

	if len(first) != 8 || len(second) != 8 || len(third) != 8 {
		t.Fatalf("particle command counts = (%d, %d, %d), want 8 each", len(first), len(second), len(third))
	}
	for i := range first {
		if first[i].Burst != 0 || first[i].Dt != 0 {
			t.Fatalf("first[%d] = Burst %d Dt %v, want seed-only Burst 0 Dt 0", i, first[i].Burst, first[i].Dt)
		}
		if second[i].Burst != 1 {
			t.Fatalf("second[%d].Burst = %d, want 1", i, second[i].Burst)
		}
		if third[i].Burst != 1 || third[i].Dt != 0 {
			t.Fatalf("third[%d] = Burst %d Dt %v, want frozen Burst 1 Dt 0", i, third[i].Burst, third[i].Dt)
		}
	}

	wantSecondDt := []float64{0.10, 0.30, 0.60, 0.90, 0.20, 0.20, 0.25, 0.25}
	for i, want := range wantSecondDt {
		if second[i].Dt != want {
			t.Fatalf("second[%d].Dt = %v, want %v", i, second[i].Dt, want)
		}
	}
	if second[4].Effect != second[5].Effect || second[4].Emitter != second[5].Emitter {
		t.Fatalf("duplicate commands do not share one emitter: %+v vs %+v", second[4], second[5])
	}
}

func particleCommands(commands []render.Command) []render.DrawParticles {
	particles := make([]render.DrawParticles, 0, len(commands))
	for _, command := range commands {
		if particle, ok := command.(render.DrawParticles); ok {
			particles = append(particles, particle)
		}
	}
	return particles
}
