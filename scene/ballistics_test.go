package scene_test

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
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
