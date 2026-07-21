package scene_test

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func newAgent(w *scene.World, pos math2d.Vec2, l scene.Locomotion) scene.EntityID {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: pos, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetVelocity(id, scene.Velocity{})
	w.SetLocomotion(id, l)
	return id
}

func TestLocomotionAcceleratesToMaxSpeed(t *testing.T) {
	w := scene.NewWorld()
	l := scene.Locomotion{Accel: 10, Decel: 5, MaxSpeed: 5, Agility: 5}
	id := newAgent(w, math2d.Vec2{}, l)
	w.SetMoveIntent(id, scene.MoveIntent{Target: math2d.Vec2{X: 100, Y: 0}, DesiredSpeed: 10})

	dt := 1.0 / 60.0
	for range 120 {
		w.ApplyLocomotionExport(dt)
	}

	vel, _ := w.Velocity(id)
	speed := math.Sqrt(float64(vel.Linear.X*vel.Linear.X) + float64(vel.Linear.Y*vel.Linear.Y))
	if speed > l.MaxSpeed+1e-6 {
		t.Errorf("speed should be capped at MaxSpeed: got %f, max %f", speed, l.MaxSpeed)
	}
	if speed < l.MaxSpeed*0.9 {
		t.Errorf("should reach near MaxSpeed after 2s: got %f", speed)
	}
}

func TestLocomotionTurnsToTarget(t *testing.T) {
	w := scene.NewWorld()
	l := scene.Locomotion{Accel: 10, Decel: 5, MaxSpeed: 10, Agility: 10}
	id := newAgent(w, math2d.Vec2{}, l)
	// Start moving right, target is up-right
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 5, Y: 0}})
	w.SetMoveIntent(id, scene.MoveIntent{Target: math2d.Vec2{X: 100, Y: 100}, DesiredSpeed: 5})

	dt := 1.0 / 60.0
	initialVY := 0.0
	for range 30 {
		w.ApplyLocomotionExport(dt)
	}

	vel, _ := w.Velocity(id)
	if vel.Linear.Y <= initialVY {
		t.Errorf("should turn toward target: vy should increase, got %f", vel.Linear.Y)
	}
}

func TestLocomotionAgilityAffectsTurnRate(t *testing.T) {
	// Higher agility -> turns faster (more lateral deviation in same time).
	run := func(agility float64) float64 {
		w := scene.NewWorld()
		l := scene.Locomotion{Accel: 10, Decel: 5, MaxSpeed: 10, Agility: agility}
		id := newAgent(w, math2d.Vec2{}, l)
		w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 5, Y: 0}})
		w.SetMoveIntent(id, scene.MoveIntent{Target: math2d.Vec2{X: 0, Y: 100}, DesiredSpeed: 5})

		dt := 1.0 / 60.0
		for range 30 {
			w.ApplyLocomotionExport(dt)
		}
		vel, _ := w.Velocity(id)
		return vel.Linear.Y // lateral component
	}

	devLow := run(2)
	devHigh := run(10)

	if devHigh <= devLow {
		t.Errorf("higher agility should turn faster: low=%f, high=%f", devLow, devHigh)
	}
}

func TestLocomotionDecelWithoutIntent(t *testing.T) {
	w := scene.NewWorld()
	l := scene.Locomotion{Decel: 5}
	id := newAgent(w, math2d.Vec2{}, l)
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 0}})
	// No MoveIntent — should decelerate

	dt := 1.0 / 60.0
	for range 60 {
		w.ApplyLocomotionExport(dt)
	}

	vel, _ := w.Velocity(id)
	speed := math.Sqrt(float64(vel.Linear.X*vel.Linear.X) + float64(vel.Linear.Y*vel.Linear.Y))
	if speed > 6 {
		t.Errorf("should decelerate without MoveIntent: speed=%f after 1s", speed)
	}
}

func TestLocomotionDeterminism(t *testing.T) {
	run := func() uint64 {
		w := scene.NewWorld()
		l := scene.Locomotion{Accel: 8, Decel: 4, MaxSpeed: 6, Agility: 3}
		id := newAgent(w, math2d.Vec2{X: 10, Y: 20}, l)
		w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})
		w.SetMoveIntent(id, scene.MoveIntent{
			Target:       math2d.Vec2{X: 50, Y: 80},
			DesiredSpeed: 4,
		})

		dt := 1.0 / 60.0
		for range 600 {
			w.ApplyLocomotionExport(dt)
		}
		return w.Checksum()
	}

	a := run()
	b := run()
	if a != b {
		t.Errorf("locomotion must be deterministic: %d != %d", a, b)
	}
}
