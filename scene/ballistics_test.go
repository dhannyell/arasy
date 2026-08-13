package scene_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

// newProjectile creates an entity with Transform, Velocity, Height, Ballistics,
// and optional Spin at the given position with a launch velocity.
func newProjectile(w *scene.World, pos math2d.Vec2, vx, vy, vz float64, bal scene.Ballistics) scene.EntityID {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: pos, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: vx, Y: vy}})
	w.SetHeight(id, scene.Height{Z: 0, VZ: vz})
	w.SetBallistics(id, bal)
	return id
}

func TestBallisticsFreeFall(t *testing.T) {
	w := scene.NewWorld()
	bal := scene.Ballistics{Gravity: 9.8}
	id := newProjectile(w, math2d.Vec2{X: 0, Y: 0}, 0, 0, 0, bal)
	w.SetHeight(id, scene.Height{Z: 10, VZ: 0})

	dt := 1.0 / 60.0
	const n = 60
	for range n {
		w.AdvanceBallisticsExport(dt)
	}

	h, _ := w.Height(id)
	// Semi-implicit Euler: Z = Z0 - 0.5*g*T*(T + dt)
	//   = 10 - 0.5*9.8*1*(1 + 1/60) = 10 - 4.9*1.01667 = 5.0183
	T := float64(n) * dt
	expected := 10.0 - 0.5*bal.Gravity*T*(T+dt)
	if math.Abs(h.Z-expected) > 1e-6 {
		t.Errorf("free fall: Z = %f, want %f", h.Z, expected)
	}
}

func TestBallisticsDragReducesRange(t *testing.T) {
	// Two projectiles: one with drag, one without. Drag should reduce horizontal speed.
	wDrag := scene.NewWorld()
	balDrag := scene.Ballistics{Gravity: 9.8, DragK: 0.1}
	idDrag := newProjectile(wDrag, math2d.Vec2{}, 10, 0, 5, balDrag)
	wDrag.SetHeight(idDrag, scene.Height{Z: 1, VZ: 5})

	wVac := scene.NewWorld()
	balVac := scene.Ballistics{Gravity: 9.8} // no drag
	idVac := newProjectile(wVac, math2d.Vec2{}, 10, 0, 5, balVac)
	wVac.SetHeight(idVac, scene.Height{Z: 1, VZ: 5})

	dt := 1.0 / 60.0
	for range 60 {
		wDrag.AdvanceBallisticsExport(dt)
		wVac.AdvanceBallisticsExport(dt)
	}

	velDrag, _ := wDrag.Velocity(idDrag)
	velVac, _ := wVac.Velocity(idVac)

	if velDrag.Linear.X >= velVac.Linear.X {
		t.Errorf("drag should reduce horizontal speed: drag=%f, vacuum=%f", velDrag.Linear.X, velVac.Linear.X)
	}
}

func TestBallisticsQuadraticDragStaysFiniteAtHighSpeed(t *testing.T) {
	w := scene.NewWorld()
	bal := scene.Ballistics{DragK: 1}
	id := newProjectile(w, math2d.Vec2{}, 122, 0, 0, bal)

	dt := 1.0 / 60.0
	previous := 122.0
	for tick := 1; tick <= 60; tick++ {
		w.AdvanceBallisticsExport(dt)
		velocity, _ := w.Velocity(id)
		speed := velocity.Linear.X
		if math.IsNaN(speed) || math.IsInf(speed, 0) {
			t.Fatalf("tick %d: quadratic drag produced non-finite velocity %v", tick, speed)
		}
		if speed < 0 {
			t.Fatalf("tick %d: drag reversed velocity to %v", tick, speed)
		}
		if speed > previous {
			t.Fatalf("tick %d: drag increased speed from %v to %v", tick, previous, speed)
		}
		previous = speed
	}
}

func TestBallisticsMagnusStepPreservesHorizontalSpeed(t *testing.T) {
	w := scene.NewWorld()
	bal := scene.Ballistics{MagnusK: 1e6}
	id := newProjectile(w, math2d.Vec2{}, 10, 4, 0, bal)
	w.SetSpin(id, scene.Spin{Omega: 1e6})

	before := math.Sqrt(116)
	for tick := 1; tick <= 60; tick++ {
		w.AdvanceBallisticsExport(1.0 / 60.0)
		velocity, _ := w.Velocity(id)
		after := math.Sqrt(float64(velocity.Linear.X*velocity.Linear.X) +
			float64(velocity.Linear.Y*velocity.Linear.Y))
		if math.IsNaN(after) || math.IsInf(after, 0) {
			t.Fatalf("tick %d: Magnus step produced non-finite speed %v", tick, after)
		}
		if math.Abs(after-before) > 1e-9 {
			t.Fatalf("tick %d: Magnus changed horizontal speed from %v to %v", tick, before, after)
		}
	}
}

func TestBallisticsMagnusCurvesFlight(t *testing.T) {
	// Ball moving right (vx=10) with positive spin -> should curve in Y direction.
	w := scene.NewWorld()
	bal := scene.Ballistics{Gravity: 9.8, MagnusK: 0.5}
	id := newProjectile(w, math2d.Vec2{}, 10, 0, 5, bal)
	w.SetHeight(id, scene.Height{Z: 1, VZ: 5})
	w.SetSpin(id, scene.Spin{Omega: 10}) // positive spin

	dt := 1.0 / 60.0
	for range 30 {
		w.AdvanceBallisticsExport(dt)
	}

	vel, _ := w.Velocity(id)
	// Magnus with positive omega and vx>0 should add +Y velocity
	if vel.Linear.Y <= 0 {
		t.Errorf("Magnus should curve flight: expected vy > 0, got %f", vel.Linear.Y)
	}

	// Negative spin should curve opposite way
	w2 := scene.NewWorld()
	id2 := newProjectile(w2, math2d.Vec2{}, 10, 0, 5, bal)
	w2.SetHeight(id2, scene.Height{Z: 1, VZ: 5})
	w2.SetSpin(id2, scene.Spin{Omega: -10})

	for range 30 {
		w2.AdvanceBallisticsExport(dt)
	}

	vel2, _ := w2.Velocity(id2)
	if vel2.Linear.Y >= 0 {
		t.Errorf("negative Magnus should curve opposite: expected vy < 0, got %f", vel2.Linear.Y)
	}
}

func TestBallisticsBounceDamping(t *testing.T) {
	w := scene.NewWorld()
	bal := scene.Ballistics{Gravity: 9.8, BounceDamping: 0.5, RestVZ: 0.1}
	id := newProjectile(w, math2d.Vec2{}, 0, 0, -5, bal)
	w.SetHeight(id, scene.Height{Z: 5, VZ: -5})

	dt := 1.0 / 60.0
	for range 300 {
		w.AdvanceBallisticsExport(dt)
	}

	h, _ := w.Height(id)
	if h.Z < 0 {
		t.Error("ball should be on or above ground after bouncing")
	}
	// After many bounces, VZ should settle below RestVZ threshold
	if math.Abs(h.VZ) > bal.RestVZ {
		t.Errorf("VZ should settle: got %f, threshold %f", h.VZ, bal.RestVZ)
	}
}

func TestBallisticsRollingStops(t *testing.T) {
	w := scene.NewWorld()
	bal := scene.Ballistics{RollFriction: 2.0}
	id := newProjectile(w, math2d.Vec2{}, 10, 0, 0, bal)
	w.SetHeight(id, scene.Height{Z: 0, VZ: 0}) // on ground

	dt := 1.0 / 60.0
	for range 120 {
		w.AdvanceBallisticsExport(dt)
	}

	vel, _ := w.Velocity(id)
	speed := math.Sqrt(float64(vel.Linear.X*vel.Linear.X) + float64(vel.Linear.Y*vel.Linear.Y))
	if speed > 2.0 {
		t.Errorf("rolling friction should slow down: speed=%f after 2s", speed)
	}
}

func TestBallisticsSpinDecay(t *testing.T) {
	w := scene.NewWorld()
	// BounceFriction=1 so ground contact doesn't affect spin; only SpinDecay matters.
	bal := scene.Ballistics{BounceFriction: 1.0, SpinDecay: 1.0}
	id := newProjectile(w, math2d.Vec2{}, 5, 0, 0, bal)
	w.SetHeight(id, scene.Height{Z: 0, VZ: 0})
	w.SetSpin(id, scene.Spin{Omega: 10})

	dt := 1.0 / 60.0
	for range 60 {
		w.AdvanceBallisticsExport(dt)
	}

	s, _ := w.Spin(id)
	// Without BounceFriction eating it, spin should be reduced by SpinDecay.
	// sd = 1/(1 + 1/60) = 60/61 per tick; after 60 ticks: ~10 * (60/61)^60 ≈ 10*0.368
	if s.Omega > 5.0 {
		t.Errorf("spin should decay: omega=%f after 1s (expected < 5)", s.Omega)
	}
	if s.Omega < 1.0 {
		t.Errorf("spin decay too fast: omega=%f after 1s (expected > 1)", s.Omega)
	}
}

func TestBallisticsSpinNotInjected(t *testing.T) {
	// A ballistic entity without a Spin component must never gain one.
	w := scene.NewWorld()
	bal := scene.Ballistics{Gravity: 9.8, SpinDecay: 1.0}
	id := newProjectile(w, math2d.Vec2{}, 5, 0, 4, bal)

	if _, has := w.Spin(id); has {
		t.Fatal("precondition: projectile should not have Spin")
	}

	dt := 1.0 / 60.0
	for range 120 {
		w.AdvanceBallisticsExport(dt)
	}

	if _, has := w.Spin(id); has {
		t.Error("advanceBallistics injected a Spin component into an entity that never had one")
	}
}

func TestBallisticsHeightOnlyIntegratesZ(t *testing.T) {
	// Height without Ballistics still falls under its own VZ with an inelastic
	// ground, letting a consumer drive flight manually.
	w := scene.NewWorld()
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetHeight(id, scene.Height{Z: 5, VZ: -1})

	if _, has := w.Ballistics(id); has {
		t.Fatal("precondition: entity should not have Ballistics")
	}

	dt := 1.0 / 60.0
	// After one tick: Z = 5 + (-1)*dt.
	w.AdvanceBallisticsExport(dt)
	h, _ := w.Height(id)
	if want := 5.0 - dt; math.Abs(h.Z-want) > 1e-9 {
		t.Errorf("height-only Z = %f, want %f", h.Z, want)
	}

	// Falls to the ground and clamps inelastically.
	for range 600 {
		w.AdvanceBallisticsExport(dt)
	}
	h, _ = w.Height(id)
	if h.Z != 0 || h.VZ != 0 {
		t.Errorf("height-only should settle at Z=0, VZ=0; got Z=%f VZ=%f", h.Z, h.VZ)
	}
}

func TestBallisticsRollingStopsUnderGravity(t *testing.T) {
	// A ball resting on the ground under real gravity must still lose speed to
	// rolling friction (not be clamped by impact friction every tick).
	w := scene.NewWorld()
	// BounceFriction=0 would instantly zero horizontal speed on the very first
	// tick if impact friction leaked into resting contact. It must not: a
	// resting ball's decay is owned by RollFriction alone.
	bal := scene.Ballistics{
		Gravity:        9.8,
		BounceDamping:  0.5,
		BounceFriction: 0.0,
		RollFriction:   1.0,
		RestVZ:         0.1, // > Gravity*dt*BounceDamping ≈ 0.082 so it settles
	}
	id := newProjectile(w, math2d.Vec2{}, 10, 0, 0, bal)
	w.SetHeight(id, scene.Height{Z: 0, VZ: 0}) // on the ground

	dt := 1.0 / 60.0
	for range 60 {
		w.AdvanceBallisticsExport(dt)
	}

	vel, _ := w.Velocity(id)
	speed := math.Sqrt(float64(vel.Linear.X*vel.Linear.X) + float64(vel.Linear.Y*vel.Linear.Y))
	if speed <= 0.5 {
		t.Errorf("impact friction leaked into resting contact (BounceFriction=0 zeroed speed): speed=%f", speed)
	}
	if speed >= 10 {
		t.Errorf("rolling friction under gravity did not slow the ball: speed=%f", speed)
	}
}

func TestBallisticsAntiTunneling(t *testing.T) {
	// A fast projectile with Substeps must not tunnel through a thin static
	// wall; the same projectile without Substeps does (proving the guard bites).
	build := func(substeps int) *scene.World {
		w := scene.NewWorld()
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 0, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 40, Y: 0}}) // 0.667/tick, 3× radius
		w.SetHeight(id, scene.Height{})
		w.SetBallistics(id, scene.Ballistics{Substeps: substeps})
		w.SetCircleCollider(id, scene.CircleCollider{Radius: 0.11})
		w.SetBody(id, scene.Body{InvMass: 1, Restitution: 0})

		// Thin static wall at x=5.
		wall := w.Spawn()
		w.SetTransform(wall, scene.Transform{Position: math2d.Vec2{X: 5, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetRectCollider(wall, scene.RectCollider{HalfExtends: math2d.Vec2{X: 0.05, Y: 2}})
		w.SetBody(wall, scene.Body{InvMass: 0})
		return w
	}

	config := scene.StepConfig{
		Bounds: math2d.Rect{Min: math2d.Vec2{X: -100, Y: -100}, Max: math2d.Vec2{X: 100, Y: 100}},
		Bounce: scene.BounceConfig{OpenSides: scene.SideLeft | scene.SideRight | scene.SideTop | scene.SideBottom},
	}
	dt := 1.0 / 60.0

	// With substeps: stays on the near side of the wall.
	wOn := build(8)
	projID := scene.EntityID(1)
	for range 60 {
		wOn.Step(dt, config)
	}
	on, _ := wOn.Transform(projID)
	if on.Position.X > 5 {
		t.Errorf("with substeps the projectile tunneled: x=%f, wall at 5", on.Position.X)
	}

	// Without substeps: tunnels straight through (control).
	wOff := build(0)
	for range 60 {
		wOff.Step(dt, config)
	}
	off, _ := wOff.Transform(projID)
	if off.Position.X <= 5 {
		t.Errorf("control failed: projectile without substeps should tunnel, x=%f", off.Position.X)
	}
}

func TestBallisticsAntiTunnelingAgainstCollisionGrid(t *testing.T) {
	build := func(shape string, substeps int) *scene.World {
		w := scene.NewWorld()
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 0.5, Y: 0.5}, Scale: math2d.Vec2{X: 1, Y: 1}})
		w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 120}})
		w.SetHeight(id, scene.Height{})
		w.SetBallistics(id, scene.Ballistics{Substeps: substeps})
		if shape == "circle" {
			w.SetCircleCollider(id, scene.CircleCollider{Radius: 0.1})
		} else {
			w.SetRectCollider(id, scene.RectCollider{HalfExtends: math2d.Vec2{X: 0.1, Y: 0.1}})
		}
		w.SetBody(id, scene.Body{InvMass: 1})

		grid := scene.NewCollisionGrid(4, 1, math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1})
		grid.SetSolid(1, 0, true)
		w.SetCollisionGrid(grid)
		return w
	}

	config := scene.StepConfig{
		Bounds: math2d.Rect{Min: math2d.Vec2{X: -100, Y: -100}, Max: math2d.Vec2{X: 100, Y: 100}},
		Bounce: scene.BounceConfig{OpenSides: scene.SideLeft | scene.SideRight | scene.SideTop | scene.SideBottom},
	}

	for _, shape := range []string{"circle", "rect"} {
		t.Run(shape, func(t *testing.T) {
			withSubsteps := build(shape, 16)
			withSubsteps.Step(1.0/60.0, config)
			transform, _ := withSubsteps.Transform(1)
			if transform.Position.X >= 1 {
				t.Fatalf("projectile with substeps crossed the grid wall: x=%v", transform.Position.X)
			}
			if len(withSubsteps.GridContacts()) == 0 {
				t.Fatal("substep collision did not retain its grid contact")
			}

			withoutSubsteps := build(shape, 0)
			withoutSubsteps.Step(1.0/60.0, config)
			transform, _ = withoutSubsteps.Transform(1)
			if transform.Position.X <= 2 {
				t.Fatalf("control projectile did not cross the grid wall: x=%v", transform.Position.X)
			}
		})
	}
}

func TestBallisticsDeterminism(t *testing.T) {
	run := func() uint64 {
		w := scene.NewWorld()
		bal := scene.Ballistics{Gravity: 9.8, DragK: 0.05, MagnusK: 0.3,
			BounceDamping: 0.6, BounceFriction: 0.8, SpinDecay: 0.5, RollFriction: 1.0, RestVZ: 0.01}
		id := newProjectile(w, math2d.Vec2{X: 1, Y: 2}, 10, 2, 8, bal)
		w.SetHeight(id, scene.Height{Z: 0.1, VZ: 8})
		w.SetSpin(id, scene.Spin{Omega: 5})

		dt := 1.0 / 60.0
		for range 600 {
			w.AdvanceBallisticsExport(dt)
		}
		return w.Checksum()
	}

	a := run()
	b := run()
	if a != b {
		t.Errorf("ballistics must be deterministic: %d != %d", a, b)
	}
}
