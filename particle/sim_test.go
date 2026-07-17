package particle

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

// baseDecl returns a minimal effect declaration with the fields every test
// needs isolated (long fixed lifetime, no drag decay, no drift) so each
// test only sets the fields relevant to the behavior it fixes.
func baseDecl() EffectDecl {
	return EffectDecl{
		Capacity: 64,
		Lifetime: Range{Min: 100, Max: 100},
		Drag:     1,
		Shape:    EmitPoint,
		VelMode:  VelCone,
	}
}

func aliveCount(e *Emitter) int {
	return len(e.pool) - len(e.free)
}

func assertVec2Near(t *testing.T, got, want math2d.Vec2) {
	t.Helper()
	const eps = 1e-9
	if math.Abs(got.X-want.X) > eps || math.Abs(got.Y-want.Y) > eps {
		t.Fatalf("got %v, want %v (within %v)", got, want, eps)
	}
}

// onlyAlive fails the test unless exactly one particle is alive in the
// pool, and returns a copy of it for internal-state assertions.
func onlyAlive(t *testing.T, e *Emitter) Particle {
	t.Helper()
	var found *Particle
	count := 0
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age < p.Life {
			count++
			found = p
		}
	}
	if count != 1 {
		t.Fatalf("alive particle count = %d, want exactly 1", count)
	}
	return *found
}

// onlyLiveParticle fails the test unless Alive() reports exactly one
// particle, and returns it.
func onlyLiveParticle(t *testing.T, e *Emitter) LiveParticle {
	t.Helper()
	got := e.Alive(nil)
	if len(got) != 1 {
		t.Fatalf("Alive() len = %d, want 1", len(got))
	}
	return got[0]
}

// --- Emission accounting ---------------------------------------------

func TestStepAccumulatesFractionalEmissionAcrossFrames(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 2.5

	e := NewEmitter(decl, 1)
	pos := math2d.Vec2{}

	e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)
	if got := aliveCount(e); got != 2 {
		t.Fatalf("aliveCount after first Step = %d, want 2 (floor(2.5))", got)
	}
	if diff := math.Abs(e.emission.Acc - 0.5); diff > 1e-9 {
		t.Fatalf("emission.Acc after first Step = %v, want 0.5 remainder", e.emission.Acc)
	}

	e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)
	if got := aliveCount(e); got != 5 {
		t.Fatalf("aliveCount after second Step = %d, want 5 (2 + floor(0.5+2.5))", got)
	}
	if diff := math.Abs(e.emission.Acc - 0.0); diff > 1e-9 {
		t.Fatalf("emission.Acc after second Step = %v, want 0", e.emission.Acc)
	}
}

func TestStepIntensityScalesEmitRate(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 10

	e := NewEmitter(decl, 1)
	e.Step(math2d.Vec2{}, 0, 0.2, 0, 1.0, decl.Capacity)

	if got := aliveCount(e); got != 2 {
		t.Fatalf("aliveCount = %d, want 2 (floor(10*0.2*1.0))", got)
	}
}

// --- Burst semantics ----------------------------------------------------

func TestBurstFiresOnlyOnCumulativeIncreaseNeverRetroactively(t *testing.T) {
	decl := baseDecl()
	decl.BurstSize = 3

	e := NewEmitter(decl, 1)
	pos := math2d.Vec2{}

	// First Step ever: burst starts nonzero (5), but the emitter must not
	// fire a retroactive burst on first sight.
	e.Step(pos, 0, 0, 5, 1.0, decl.Capacity)
	if got := aliveCount(e); got != 0 {
		t.Fatalf("aliveCount after first Step = %d, want 0 (no retroactive burst)", got)
	}

	// Re-sending the same cumulative value fires nothing.
	e.Step(pos, 0, 0, 5, 1.0, decl.Capacity)
	if got := aliveCount(e); got != 0 {
		t.Fatalf("aliveCount after resending same burst value = %d, want 0", got)
	}

	// A cumulative increase of 2 fires 2*BurstSize particles.
	e.Step(pos, 0, 0, 7, 1.0, decl.Capacity)
	if got := aliveCount(e); got != 6 {
		t.Fatalf("aliveCount after burst delta 2 = %d, want 6 (2*BurstSize)", got)
	}
}

func TestBurstAccumulatedDuringPauseFiresOnResume(t *testing.T) {
	decl := baseDecl()
	decl.BurstSize = 3

	e := NewEmitter(decl, 1)
	pos := math2d.Vec2{}

	e.Step(pos, 0, 0, 1, 1.0, decl.Capacity) // seeds lastBurst=1, no fire
	if got := aliveCount(e); got != 0 {
		t.Fatalf("aliveCount after seeding Step = %d, want 0", got)
	}

	// Paused frame (dt<=0): the emitter must ignore this burst value
	// entirely, not just defer it.
	e.Step(pos, 0, 0, 3, 0.0, decl.Capacity)
	if got := aliveCount(e); got != 0 {
		t.Fatalf("aliveCount after paused Step = %d, want 0 (dt<=0 never emits)", got)
	}

	// Resuming with the burst counter still at 3 must fire the delta
	// against the last value seen while dt>0 (1), i.e. 2*BurstSize.
	e.Step(pos, 0, 0, 3, 1.0, decl.Capacity)
	if got := aliveCount(e); got != 6 {
		t.Fatalf("aliveCount on resume = %d, want 6 (delta 2 * BurstSize accumulated across the pause)", got)
	}
}

// --- Pause / scrub (dt <= 0) ---------------------------------------------

func TestStepWithNonPositiveDtNeitherEmitsIntegratesNorAgesButUpdatesLastPos(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 5
	decl.Gravity = math2d.Vec2{X: 0, Y: -10}
	decl.Speed = Range{Min: 1, Max: 1}

	tests := []struct {
		name string
		dt   float64
	}{
		{name: "zero dt", dt: 0},
		{name: "negative dt", dt: -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEmitter(decl, 5)
			pos0 := math2d.Vec2{X: 0, Y: 0}
			pos1 := math2d.Vec2{X: 9, Y: -9}

			e.Step(pos0, 0, 1, 0, 1.0, decl.Capacity)

			before := make([]Particle, len(e.pool))
			copy(before, e.pool)
			beforeFreeLen := len(e.free)

			e.Step(pos1, 0, 1, 0, tt.dt, decl.Capacity)

			for i := range before {
				if before[i] != e.pool[i] {
					t.Fatalf("particle %d changed during dt<=0 Step: before=%+v after=%+v", i, before[i], e.pool[i])
				}
			}
			if len(e.free) != beforeFreeLen {
				t.Fatalf("free-list length changed during dt<=0 Step: before=%d after=%d", beforeFreeLen, len(e.free))
			}
			if e.emission.LastPos != pos1 {
				t.Fatalf("emission.LastPos = %v, want %v (LastPos updates even when dt<=0)", e.emission.LastPos, pos1)
			}
		})
	}
}

// --- Birth-frame integration order ---------------------------------------

func TestNewbornParticleAgesByDtOnItsBirthFrame(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 10 // *0.1 dt = exactly 1 particle

	e := NewEmitter(decl, 1)
	e.Step(math2d.Vec2{}, 0, 1, 0, 0.1, decl.Capacity)

	p := onlyAlive(t, e)
	if math.Abs(p.Age-0.1) > 1e-9 {
		t.Fatalf("newborn Age = %v, want 0.1 (emission runs before integration)", p.Age)
	}
}

func TestStepScaledScalesSpawnShapePerAxis(t *testing.T) {
	decl := baseDecl()
	decl.Shape = EmitRect
	decl.EmitExtent = math2d.Vec2{X: 2, Y: 3}
	decl.Speed = Range{}
	decl.BurstSize = 1

	spawn := func(scale math2d.Vec2) Particle {
		e := NewEmitter(decl, 99)
		e.StepScaled(math2d.Vec2{}, scale, 0, 0, 0, 0.01, decl.Capacity)
		e.StepScaled(math2d.Vec2{}, scale, 0, 0, 1, 0.01, decl.Capacity)
		return onlyAlive(t, e)
	}

	plain := spawn(math2d.Vec2{X: 1, Y: 1})
	scaled := spawn(math2d.Vec2{X: 2.5, Y: 4})
	if math.Abs(scaled.Pos.X-plain.Pos.X*2.5) > 1e-9 {
		t.Fatalf("scaled X = %v, want %v", scaled.Pos.X, plain.Pos.X*2.5)
	}
	if math.Abs(scaled.Pos.Y-plain.Pos.Y*4) > 1e-9 {
		t.Fatalf("scaled Y = %v, want %v", scaled.Pos.Y, plain.Pos.Y*4)
	}
}

// --- Integration: gravity + drag ------------------------------------------

func TestStepIntegratesGravityAndDragAnalytically(t *testing.T) {
	decl := baseDecl()
	decl.BurstSize = 1
	decl.Gravity = math2d.Vec2{X: 0, Y: -10}
	decl.Drag = 0.5
	decl.Speed = Range{Min: 0, Max: 0}

	e := NewEmitter(decl, 7)
	pos := math2d.Vec2{X: 5, Y: 5}

	e.Step(pos, 0, 0, 0, 0.1, decl.Capacity) // seed, no burst
	e.Step(pos, 0, 0, 1, 1.0, decl.Capacity) // burst delta 1 -> spawn + integrate dt=1

	p := onlyAlive(t, e)
	// vel = (0 + gravity*dt) * drag^dt = (0,-10)*0.5 = (0,-5)
	// pos = origin + vel*dt = (5,5) + (0,-5) = (5,0)
	assertVec2Near(t, p.Vel, math2d.Vec2{X: 0, Y: -5})
	assertVec2Near(t, p.Pos, math2d.Vec2{X: 5, Y: 0})
	if math.Abs(p.Age-1.0) > 1e-9 {
		t.Fatalf("Age = %v, want 1.0 (birth-frame integration)", p.Age)
	}
}

// --- Budget cap on live particles -----------------------------------------

func TestStepDropsEmissionAboveMaxAliveWithoutDeferring(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 1000
	decl.Capacity = 10

	e := NewEmitter(decl, 1)
	pos := math2d.Vec2{}

	e.Step(pos, 0, 1, 0, 1.0, 3)
	if got := aliveCount(e); got != 3 {
		t.Fatalf("aliveCount capped at maxAlive=3 = %d, want 3", got)
	}
	if got := e.emission.Emitted; got != 3 {
		t.Fatalf("emission.Emitted after cap = %d, want 3 accepted spawns", got)
	}
	if diff := math.Abs(e.emission.Acc - 0); diff > 1e-9 {
		t.Fatalf("emission.Acc after capped emission = %v, want 0 (excess discarded, not deferred)", e.emission.Acc)
	}

	e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)
	if got := aliveCount(e); got != decl.Capacity {
		t.Fatalf("aliveCount after raising the cap to capacity = %d, want %d", got, decl.Capacity)
	}
	if got := e.emission.Emitted; got != uint32(decl.Capacity) {
		t.Fatalf("emission.Emitted after filling pool = %d, want %d", got, decl.Capacity)
	}

	// A third frame at full capacity must not overshoot even though the
	// emit rate keeps demanding far more than the pool can hold.
	e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)
	if got := aliveCount(e); got != decl.Capacity {
		t.Fatalf("aliveCount at steady full capacity = %d, want %d (no overflow, no backlog)", got, decl.Capacity)
	}
}

// --- Spawn origin interpolation --------------------------------------------

func TestSpawnOriginInterpolatesBetweenLastPosAndPosAcrossMultipleSpawns(t *testing.T) {
	decl := baseDecl()
	decl.BurstSize = 4
	decl.Speed = Range{Min: 0, Max: 0} // no post-spawn drift

	e := NewEmitter(decl, 1)
	pos0 := math2d.Vec2{X: 0, Y: 0}
	pos1 := math2d.Vec2{X: 8, Y: 0}

	e.Step(pos0, 0, 0, 0, 1.0, decl.Capacity) // seed lastPos=pos0, no burst
	e.Step(pos1, 0, 0, 1, 1.0, decl.Capacity) // burst delta 1 -> 4 spawns

	if got := aliveCount(e); got != 4 {
		t.Fatalf("aliveCount = %d, want 4", got)
	}

	var xs []float64
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age < p.Life {
			xs = append(xs, p.Pos.X)
		}
	}
	want := []float64{2, 4, 6, 8} // lerp(0,8,(n+1)/4) for n=0..3
	if len(xs) != len(want) {
		t.Fatalf("spawned count = %d, want %d", len(xs), len(want))
	}
	// Order across pool slots is an implementation detail; compare as a
	// sorted set of X offsets instead.
	for i := 0; i < len(xs); i++ {
		for j := i + 1; j < len(xs); j++ {
			if xs[j] < xs[i] {
				xs[i], xs[j] = xs[j], xs[i]
			}
		}
	}
	for i, w := range want {
		if math.Abs(xs[i]-w) > 1e-9 {
			t.Fatalf("sorted spawn X offsets = %v, want %v", xs, want)
		}
	}
}

func TestFirstStepNeverInterpolatesSpawnOrigin(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 2 // continuous emission fires on the first Step, unlike burst

	e := NewEmitter(decl, 1)
	pos := math2d.Vec2{X: 3, Y: -4}

	e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)

	if got := aliveCount(e); got != 2 {
		t.Fatalf("aliveCount = %d, want 2", got)
	}
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age >= p.Life {
			continue
		}
		if p.Pos != pos {
			t.Fatalf("particle spawned at %v on the first Step, want exactly %v (no interpolation)", p.Pos, pos)
		}
	}
}

// --- Spawn shapes -----------------------------------------------------------

func TestSpawnShapesConstrainOffsetFromEmitterPosition(t *testing.T) {
	const eps = 1e-9

	tests := []struct {
		name  string
		setup func(*EffectDecl)
		check func(t *testing.T, offset math2d.Vec2)
	}{
		{
			name: "point spawns exactly at the emitter position",
			setup: func(d *EffectDecl) {
				d.Shape = EmitPoint
			},
			check: func(t *testing.T, offset math2d.Vec2) {
				t.Helper()
				if offset.X != 0 || offset.Y != 0 {
					t.Fatalf("offset = %v, want exactly (0,0)", offset)
				}
			},
		},
		{
			name: "disc spawns within the declared radius",
			setup: func(d *EffectDecl) {
				d.Shape = EmitDisc
				d.EmitRadius = 5
			},
			check: func(t *testing.T, offset math2d.Vec2) {
				t.Helper()
				if got := offset.Len(); got > 5+eps {
					t.Fatalf("offset length = %v, want <= 5 (EmitRadius)", got)
				}
			},
		},
		{
			name: "rect spawns within the axis-aligned half-extents",
			setup: func(d *EffectDecl) {
				d.Shape = EmitRect
				d.EmitExtent = math2d.Vec2{X: 3, Y: 2}
			},
			check: func(t *testing.T, offset math2d.Vec2) {
				t.Helper()
				if math.Abs(offset.X) > 3+eps || math.Abs(offset.Y) > 2+eps {
					t.Fatalf("offset = %v, want within half-extents (3,2)", offset)
				}
			},
		},
		{
			name: "ring spawns exactly on the border",
			setup: func(d *EffectDecl) {
				d.Shape = EmitRing
				d.EmitRadius = 4
			},
			check: func(t *testing.T, offset math2d.Vec2) {
				t.Helper()
				if got := offset.Len(); math.Abs(got-4) > eps {
					t.Fatalf("offset length = %v, want exactly 4 (EmitRadius)", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decl := baseDecl()
			decl.Capacity = 200
			decl.EmitRate = float64(decl.Capacity)
			decl.Speed = Range{Min: 0, Max: 0} // no post-spawn drift
			tt.setup(&decl)

			e := NewEmitter(decl, 123)
			pos := math2d.Vec2{X: 10, Y: -10}
			e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)

			if aliveCount(e) == 0 {
				t.Fatal("no particles spawned, test setup is broken")
			}
			for i := range e.pool {
				p := &e.pool[i]
				if p.Age >= p.Life {
					continue
				}
				tt.check(t, p.Pos.Sub(pos))
			}
		})
	}
}

// --- Velocity modes ----------------------------------------------------------

func TestVelRadialPointsAwayFromEmitterOrigin(t *testing.T) {
	decl := baseDecl()
	decl.Capacity = 100
	decl.EmitRate = 100
	decl.Shape = EmitRing // guarantees a nonzero spawn offset every time
	decl.EmitRadius = 5
	decl.VelMode = VelRadial
	decl.Speed = Range{Min: 2, Max: 2}

	e := NewEmitter(decl, 9)
	pos := math2d.Vec2{X: -1, Y: 1}
	e.Step(pos, 0, 1, 0, 1.0, decl.Capacity)

	if aliveCount(e) == 0 {
		t.Fatal("no particles spawned, test setup is broken")
	}
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age >= p.Life {
			continue
		}
		offset := p.Pos.Sub(pos)
		if dot := offset.Dot(p.Vel); dot <= 0 {
			t.Fatalf("dot(offset, vel) = %v, want > 0 (velocity points outward)", dot)
		}
	}
}

func TestVelTangentialIsPerpendicularToSpawnOffset(t *testing.T) {
	decl := baseDecl()
	decl.Capacity = 100
	decl.BurstSize = 100
	decl.Shape = EmitRing
	decl.EmitRadius = 5
	decl.VelMode = VelTangential
	decl.Speed = Range{Min: 2, Max: 2}

	e := NewEmitter(decl, 9)
	pos := math2d.Vec2{X: 2, Y: -3}
	e.Step(pos, 0, 0, 0, 1.0, decl.Capacity) // seed, no burst

	// A vanishingly small dt keeps birth-frame integration drift (which
	// nudges Pos along Vel and would otherwise inject a small nonzero
	// self-dot term) negligible, so the spawn offset is still exactly
	// what determined the velocity direction.
	e.Step(pos, 0, 0, 1, 1e-6, decl.Capacity)

	if aliveCount(e) == 0 {
		t.Fatal("no particles spawned, test setup is broken")
	}
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age >= p.Life {
			continue
		}
		offset := p.Pos.Sub(pos)
		if dot := offset.Dot(p.Vel); math.Abs(dot) > 1e-3 {
			t.Fatalf("dot(offset, vel) = %v, want ~0 (velocity perpendicular to offset)", dot)
		}
	}
}

// --- Wobble / FadeIn are draw-only ------------------------------------------

func TestWobbleAndFadeInOnlyAffectAliveNeverStepState(t *testing.T) {
	base := baseDecl()
	base.EmitRate = 5
	base.Speed = Range{Min: 1, Max: 1}
	base.Lifetime = Range{Min: 10, Max: 10}
	base.Shape = EmitDisc
	base.EmitRadius = 3

	plain := base
	decorated := base
	decorated.WobbleAmp = 1.5
	decorated.WobbleFreq = 2.0
	decorated.FadeIn = 0.3

	ep := NewEmitter(plain, 99)
	ed := NewEmitter(decorated, 99)

	pos := math2d.Vec2{X: 1, Y: 2}
	for i := 0; i < 5; i++ {
		ep.Step(pos, 0, 1, 0, 0.1, plain.Capacity)
		ed.Step(pos, 0, 1, 0, 0.1, decorated.Capacity)
	}

	if len(ep.free) != len(ed.free) {
		t.Fatalf("free-list length differs: plain=%d decorated=%d", len(ep.free), len(ed.free))
	}
	for i := range ep.pool {
		a, b := ep.pool[i], ed.pool[i]
		// WobbleAxis is the one field whose presence legitimately depends
		// on WobbleAmp != 0 (spawn only writes it when wobble is
		// declared); every other field must be bit-for-bit identical.
		b.WobbleAxis = a.WobbleAxis
		if a != b {
			t.Fatalf("particle %d internal state diverged: plain=%+v decorated=%+v", i, a, b)
		}
	}

	plainAlive := ep.Alive(nil)
	decoratedAlive := ed.Alive(nil)
	if len(plainAlive) == 0 || len(plainAlive) != len(decoratedAlive) {
		t.Fatalf("alive counts = plain:%d decorated:%d, want equal and nonzero", len(plainAlive), len(decoratedAlive))
	}
	identical := true
	for i := range plainAlive {
		if plainAlive[i].Pos != decoratedAlive[i].Pos || plainAlive[i].Color != decoratedAlive[i].Color {
			identical = false
			break
		}
	}
	if identical {
		t.Fatal("Alive() output identical between plain and decorated emitters, want wobble/fade-in to change Pos/Color")
	}
}

// --- Alive(): rotation formula ------------------------------------------

func TestAliveComputesRotationFromInitialRotationPlusSpinTimesAge(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 20
	decl.Spin = 3.0
	decl.Rotation = Range{Min: 0, Max: 6.28318530718}
	decl.Lifetime = Range{Min: 5, Max: 5}

	e := NewEmitter(decl, 17)
	pos := math2d.Vec2{}
	for i := 0; i < 4; i++ {
		e.Step(pos, 0, 1, 0, 0.05, decl.Capacity)
	}

	live := e.Alive(nil)
	if len(live) == 0 {
		t.Fatal("no particles alive, test setup is broken")
	}

	idx := 0
	for i := range e.pool {
		p := &e.pool[i]
		if p.Age >= p.Life {
			continue
		}
		want := p.Rotation + p.Spin*p.Age
		if math.Abs(live[idx].Rotation-want) > 1e-9 {
			t.Fatalf("particle %d Rotation = %v, want %v (rot0 + Spin*Age)", idx, live[idx].Rotation, want)
		}
		idx++
	}
}

func TestAliveComputesFake3DHorizontalFlip(t *testing.T) {
	decl := baseDecl()
	decl.BurstSize = 1
	decl.Lifetime = Range{Min: 10, Max: 10}
	decl.Speed = Range{}
	decl.FlipXFreq = 1

	e := NewEmitter(decl, 17)
	e.Step(math2d.Vec2{}, 0, 0, 0, 0.01, decl.Capacity)
	e.Step(math2d.Vec2{}, 0, 0, 1, 0.1, decl.Capacity)
	live := onlyLiveParticle(t, e)
	p := onlyAlive(t, e)
	want := math.Cos(2*math.Pi*decl.FlipXFreq*p.Age + p.WobblePhase)
	if math.Abs(live.ScaleX-want) > 1e-9 {
		t.Fatalf("ScaleX = %v, want %v", live.ScaleX, want)
	}
}

// --- Alive(): fade-in envelope -----------------------------------------

func TestAliveAppliesFadeInEnvelopeToAlpha(t *testing.T) {
	decl := baseDecl()
	decl.BurstSize = 1
	decl.Lifetime = Range{Min: 10, Max: 10}
	decl.ColorStart = [4]float64{1, 1, 1, 1}
	decl.ColorEnd = [4]float64{1, 1, 1, 1}
	decl.FadeIn = 0.5
	decl.Speed = Range{Min: 0, Max: 0}

	e := NewEmitter(decl, 3)
	pos := math2d.Vec2{}

	e.Step(pos, 0, 0, 0, 0.1, decl.Capacity) // seed, no burst
	e.Step(pos, 0, 0, 1, 1.0, decl.Capacity) // spawn 1, Age becomes 1.0

	live := onlyLiveParticle(t, e)
	p := onlyAlive(t, e)
	wantAlpha := min(1.0, (p.Age/p.Life)/decl.FadeIn)
	if math.Abs(live.Color[3]-wantAlpha) > 1e-9 {
		t.Fatalf("Color[3] = %v, want %v (progress/FadeIn)", live.Color[3], wantAlpha)
	}

	// Advance further: progress/FadeIn now exceeds 1 and must clamp.
	e.Step(pos, 0, 0, 1, 5.0, decl.Capacity)
	live2 := onlyLiveParticle(t, e)
	if live2.Color[3] != 1.0 {
		t.Fatalf("Color[3] = %v, want 1.0 (clamped once progress/FadeIn >= 1)", live2.Color[3])
	}
}

// --- Determinism -------------------------------------------------------

func TestSameDeclSeedAndStepSequenceProducesIdenticalAlive(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 7
	decl.BurstSize = 2
	decl.Speed = Range{Min: 1, Max: 4}
	decl.Lifetime = Range{Min: 2, Max: 5}
	decl.Shape = EmitDisc
	decl.EmitRadius = 3
	decl.VelMode = VelRadial
	decl.Gravity = math2d.Vec2{X: 0, Y: -2}
	decl.Drag = 0.9
	decl.Spin = 1.5
	decl.Rotation = Range{Min: 0, Max: 6.28}
	decl.WobbleAmp = 0.4
	decl.WobbleFreq = 1.2
	decl.FadeIn = 0.2

	run := func() []LiveParticle {
		e := NewEmitter(decl, 424242)
		positions := []math2d.Vec2{{X: 0, Y: 0}, {X: 1, Y: 0.5}, {X: 2, Y: 1}, {X: 1.5, Y: 2}}
		var burst uint32
		for i, p := range positions {
			if i == 2 {
				burst = 3
			}
			e.Step(p, 0.1, 1, burst, 0.05, decl.Capacity)
		}
		return e.Alive(nil)
	}

	first := run()
	second := run()

	if len(first) != len(second) {
		t.Fatalf("alive counts differ: first=%d second=%d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("particle %d differs across identical runs: first=%+v second=%+v", i, first[i], second[i])
		}
	}
}

// --- Allocation discipline -----------------------------------------------

func TestStepAndAliveAllocateNothingInSteadyState(t *testing.T) {
	decl := baseDecl()
	decl.EmitRate = 50
	decl.BurstSize = 5
	decl.Lifetime = Range{Min: 1, Max: 2}
	decl.Shape = EmitDisc
	decl.EmitRadius = 2
	decl.WobbleAmp = 0.3
	decl.WobbleFreq = 1.0
	decl.FadeIn = 0.2
	decl.Capacity = 256

	e := NewEmitter(decl, 55)
	pos := math2d.Vec2{X: 1, Y: 1}
	dst := make([]LiveParticle, 0, decl.Capacity)

	// Warm up until the pool reaches steady churn (spawns and deaths
	// overlapping) so the measured calls don't pay for first-time growth.
	var burst uint32
	for i := 0; i < 50; i++ {
		burst = uint32(i / 5)
		e.Step(pos, 0, 1, burst, 0.05, decl.Capacity)
		dst = e.Alive(dst[:0])
	}

	allocsStep := testing.AllocsPerRun(50, func() {
		burst++
		e.Step(pos, 0, 1, burst, 0.05, decl.Capacity)
	})
	if allocsStep != 0 {
		t.Fatalf("Step AllocsPerRun = %v, want 0", allocsStep)
	}

	allocsAlive := testing.AllocsPerRun(50, func() {
		dst = e.Alive(dst[:0])
	})
	if allocsAlive != 0 {
		t.Fatalf("Alive AllocsPerRun = %v, want 0", allocsAlive)
	}
}
