package scene

import "github.com/dhannyell/frameforge/math2d"

func (w *World) applyAcceleration(dt float64) {
	for id, acceleration := range w.accelerations {
		velocity, ok := w.velocities[id]
		if !ok {
			continue
		}

		velocity.Linear = math2d.Vec2{
			X: velocity.Linear.X + float64(acceleration.Linear.X*dt),
			Y: velocity.Linear.Y + float64(acceleration.Linear.Y*dt),
		}
		w.velocities[id] = velocity
	}
}

func (w *World) integrate(dt float64) {
	for id, velocity := range w.velocities {
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		transform.Position = math2d.Vec2{
			X: transform.Position.X + float64(velocity.Linear.X*dt),
			Y: transform.Position.Y + float64(velocity.Linear.Y*dt),
		}
		w.transforms[id] = transform
	}
}

func (w *World) applyDamping(dt float64) {
	for id, damping := range w.dampings {
		velocity, ok := w.velocities[id]
		if !ok {
			continue
		}

		factor := 1 - float64(damping.Linear*dt)
		if factor < 0 {
			factor = 0
		}

		velocity.Linear = velocity.Linear.Mul(factor)
		w.velocities[id] = velocity
	}
}
