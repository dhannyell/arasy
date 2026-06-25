package scene

func (w *World) ApplyAcceleration(dt float64) {
	for id, acceleration := range w.Accelerations {
		velocity, ok := w.Velocities[id]
		if !ok {
			continue
		}

		velocity.Linear = velocity.Linear.Add(acceleration.Linear.Mul(dt))
		w.Velocities[id] = velocity
	}
}

func (w *World) Integrate(dt float64) {
	for id, velocity := range w.Velocities {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		transform.Position = transform.Position.Add(velocity.Linear.Mul(dt))
		w.Transforms[id] = transform
	}
}

func (w *World) ApplyDamping(dt float64) {
	for id, damping := range w.Dampings {
		velocity, ok := w.Velocities[id]
		if !ok {
			continue
		}

		factor := 1 - damping.Linear*dt
		if factor < 0 {
			factor = 0
		}

		velocity.Linear = velocity.Linear.Mul(factor)
		w.Velocities[id] = velocity
	}
}
