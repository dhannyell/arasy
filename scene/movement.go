package scene

func (w *World) ApplyAcceleration(dt float64) {
	for id, acceleration := range w.accelerations {
		velocity, ok := w.velocities[id]
		if !ok {
			continue
		}

		velocity.Linear = velocity.Linear.Add(acceleration.Linear.Mul(dt))
		w.velocities[id] = velocity
	}
}

func (w *World) Integrate(dt float64) {
	for id, velocity := range w.velocities {
		transform, ok := w.transforms[id]
		if !ok {
			continue
		}

		transform.Position = transform.Position.Add(velocity.Linear.Mul(dt))
		w.transforms[id] = transform
	}
}

func (w *World) ApplyDamping(dt float64) {
	for id, damping := range w.dampings {
		velocity, ok := w.velocities[id]
		if !ok {
			continue
		}

		factor := 1 - damping.Linear*dt
		if factor < 0 {
			factor = 0
		}

		velocity.Linear = velocity.Linear.Mul(factor)
		w.velocities[id] = velocity
	}
}
