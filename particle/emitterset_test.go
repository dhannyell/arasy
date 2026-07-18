package particle_test

import (
	"slices"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
)

// testEffectDecl builds an EffectDecl in Go, mirroring the manifest used by
// platform/raster/renderer_particles_test.go before the lifecycle moved
// here. Ranges collapse to a single value so spawn output only depends on
// EmitRate/Dt, not on the per-particle hash.
func testEffectDecl(capacity int) particle.EffectDecl {
	return particle.EffectDecl{
		Name:       "test",
		Capacity:   capacity,
		EmitRate:   8,
		BurstSize:  1,
		Lifetime:   particle.Range{Min: 10, Max: 10},
		Speed:      particle.Range{Min: 2, Max: 2},
		Shape:      particle.EmitPoint,
		VelMode:    particle.VelCone,
		Drag:       1,
		SizeStart:  particle.Range{Min: 4, Max: 4},
		SizeEnd:    particle.Range{Min: 4, Max: 4},
		ColorStart: [4]float64{1, 1, 1, 1},
		ColorEnd:   [4]float64{1, 1, 1, 1},
	}
}

func testInput(seed uint64, dt float64) particle.Input {
	return particle.Input{
		Pos:       math2d.Vec2{X: 20, Y: 20},
		Intensity: 1,
		Dt:        dt,
		Seed:      seed,
	}
}

func liveCopy(r particle.Result) []particle.LiveParticle {
	return append([]particle.LiveParticle(nil), r.Live...)
}

func TestEmitterSetAdvancesOnlyOnceWhenDrawnTwiceInOneFrame(t *testing.T) {
	set := particle.NewEmitterSet(nil)
	decl := testEffectDecl(8)
	key := particle.InstanceKey{Effect: 1, Emitter: 1}
	in := testInput(42, 0.25)

	set.BeginFrame()
	first := liveCopy(set.Draw(key, decl, in))
	if len(first) == 0 {
		t.Fatal("first draw did not spawn any particle")
	}
	second := liveCopy(set.Draw(key, decl, in))

	if !slices.Equal(second, first) {
		t.Fatalf("duplicate draw in the same frame advanced the emitter:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestEmitterSetFirstSeedWinsWithinFrame(t *testing.T) {
	set := particle.NewEmitterSet(nil)
	decl := testEffectDecl(8)
	key := particle.InstanceKey{Effect: 1, Emitter: 1}
	first := testInput(1, 0.25)
	conflicting := testInput(2, 0.25)

	set.BeginFrame()
	firstRes := set.Draw(key, decl, first)
	if firstRes.Skipped {
		t.Fatal("first draw of the frame was skipped")
	}
	conflictRes := set.Draw(key, decl, conflicting)
	if !conflictRes.Skipped {
		t.Fatal("conflicting seed within the same frame was not skipped")
	}
	if len(conflictRes.Live) != 0 {
		t.Fatalf("skipped draw returned live particles: %+v", conflictRes.Live)
	}

	// A new frame with the losing seed recreates the instance: the pool
	// resets, so the same small Dt spawns the fresh-emitter count again
	// instead of continuing to accumulate on top of the first seed's state.
	set.BeginFrame()
	fresh := set.Draw(key, decl, conflicting)
	if fresh.Skipped {
		t.Fatal("new seed on a new frame was skipped")
	}
	if len(fresh.Live) != len(firstRes.Live) {
		t.Fatalf("reseed on new frame live count = %d, want fresh-emitter count %d", len(fresh.Live), len(firstRes.Live))
	}
}

func TestEmitterSetDtZeroFreezesLiveParticles(t *testing.T) {
	set := particle.NewEmitterSet(nil)
	decl := testEffectDecl(8)
	key := particle.InstanceKey{Effect: 1, Emitter: 1}

	set.BeginFrame()
	before := liveCopy(set.Draw(key, decl, testInput(7, 0.25)))
	if len(before) == 0 {
		t.Fatal("setup draw did not spawn any particle")
	}

	set.BeginFrame()
	after := liveCopy(set.Draw(key, decl, testInput(7, 0)))

	if !slices.Equal(after, before) {
		t.Fatalf("dt=0 changed live particles:\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestEmitterSetKeepsIndependentPoolsPerKey(t *testing.T) {
	set := particle.NewEmitterSet(nil)
	decl := testEffectDecl(8)
	firstKey := particle.InstanceKey{Effect: 1, Emitter: 1}
	secondKey := particle.InstanceKey{Effect: 1, Emitter: 2}

	set.BeginFrame()
	first := set.Draw(firstKey, decl, testInput(1, 0.25))
	second := set.Draw(secondKey, decl, testInput(2, 0))

	if len(first.Live) == 0 {
		t.Fatal("first emitter did not spawn any particle")
	}
	if len(second.Live) != 0 {
		t.Fatalf("second emitter (dt=0, never drawn before) spawned particles: %+v", second.Live)
	}

	set.BeginFrame()
	set.Draw(firstKey, decl, testInput(1, 0.25))
	secondAgain := set.Draw(secondKey, decl, testInput(2, 0))
	if len(secondAgain.Live) != 0 {
		t.Fatalf("advancing the first emitter changed the second emitter's live count to %d", len(secondAgain.Live))
	}
}

func TestEmitterSetExpiresAfterTTLFramesWithoutCommand(t *testing.T) {
	decl := testEffectDecl(8)
	key := particle.InstanceKey{Effect: 1, Emitter: 1}
	spawn := testInput(7, 0.25)
	freeze := testInput(7, 0)

	survivor := particle.NewEmitterSet(nil)
	survivor.BeginFrame()
	if len(survivor.Draw(key, decl, spawn).Live) == 0 {
		t.Fatal("setup: emitter did not spawn")
	}
	for range particle.EmitterTTL {
		survivor.BeginFrame()
	}
	if got := survivor.Draw(key, decl, freeze).Live; len(got) == 0 {
		t.Fatalf("emitter expired within %d unseen frames", particle.EmitterTTL)
	}

	expired := particle.NewEmitterSet(nil)
	expired.BeginFrame()
	if len(expired.Draw(key, decl, spawn).Live) == 0 {
		t.Fatal("setup: emitter did not spawn")
	}
	for range particle.EmitterTTL + 1 {
		expired.BeginFrame()
	}
	if got := expired.Draw(key, decl, freeze).Live; len(got) != 0 {
		t.Fatalf("emitter still alive after %d unseen frames; got %d live particles", particle.EmitterTTL+1, len(got))
	}
}

func TestEmitterSetBudgetSharesCapacityAndReportsReduction(t *testing.T) {
	decl := testEffectDecl(8)
	key := particle.InstanceKey{Effect: 1, Emitter: 1}
	in := testInput(1, 0.25)

	unbounded := particle.NewEmitterSet(nil)
	unbounded.BeginFrame()
	full := unbounded.Draw(key, decl, in)
	if full.BudgetReduced {
		t.Fatal("nil budget reported a reduction")
	}
	if len(full.Live) == 0 {
		t.Fatal("nil-budget draw did not spawn any particle")
	}

	tight := particle.NewEmitterSet(particle.NewBudget(0.020, 1, 1))
	tight.BeginFrame()
	reduced := tight.Draw(key, decl, in)
	if !reduced.BudgetReduced {
		t.Fatal("tight budget did not report a reduction")
	}
	if len(reduced.Live) >= len(full.Live) {
		t.Fatalf("tight-budget live count = %d, want fewer than unbounded count %d", len(reduced.Live), len(full.Live))
	}
}

func TestEmitterSetZeroAllocInSteadyState(t *testing.T) {
	set := particle.NewEmitterSet(nil)
	decl := testEffectDecl(64)
	key := particle.InstanceKey{Effect: 1, Emitter: 1}

	set.BeginFrame()
	set.Draw(key, decl, testInput(1, 0.25))

	allocs := testing.AllocsPerRun(50, func() {
		set.BeginFrame()
		set.Draw(key, decl, testInput(1, 1.0/60.0))
	})
	if allocs != 0 {
		t.Fatalf("BeginFrame+Draw in steady state allocated %.1f allocs/op, want 0", allocs)
	}
}
