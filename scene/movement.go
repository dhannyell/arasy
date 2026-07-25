package scene

import "github.com/dhannyell/arasy/math2d"

func (w *World) applyAcceleration(dt float64) {
	for i, id := range w.accelerations.ids {
		acceleration := w.accelerations.dense[i]
		velocity, ok := w.velocities.get(id)
		if !ok {
			continue
		}

		velocity.Linear = math2d.Vec2{
			X: velocity.Linear.X + float64(acceleration.Linear.X*dt),
			Y: velocity.Linear.Y + float64(acceleration.Linear.Y*dt),
		}
		w.velocities.set(id, velocity)
	}
}

func (w *World) integrate(dt float64) {
	for i, id := range w.velocities.ids {
		if _, moved := w.ccdMoved[id]; moved {
			// Horizontal motion already integrated with continuous collision.
			continue
		}
		velocity := w.velocities.dense[i]
		transform, ok := w.transforms.get(id)
		if !ok {
			continue
		}

		transform.Position = math2d.Vec2{
			X: transform.Position.X + float64(velocity.Linear.X*dt),
			Y: transform.Position.Y + float64(velocity.Linear.Y*dt),
		}
		w.transforms.set(id, transform)
	}
}

func (w *World) applyDamping(dt float64) {
	for i, id := range w.dampings.ids {
		damping := w.dampings.dense[i]
		velocity, ok := w.velocities.get(id)
		if !ok {
			continue
		}

		// Damping removes energy and never adds it, so the factor is confined
		// to [0, 1]: a negative Damping.Linear would otherwise accelerate the
		// entity exponentially toward Inf. A NaN coefficient fails both
		// comparisons, so it is mapped to 1 (no-op) rather than poisoning the
		// velocity for every later tick.
		factor := 1 - float64(damping.Linear*dt)
		switch {
		case factor < 0:
			factor = 0
		case !(factor <= 1):
			factor = 1
		}

		velocity.Linear = velocity.Linear.Mul(factor)
		w.velocities.set(id, velocity)
	}
}
