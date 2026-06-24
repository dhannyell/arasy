package scene

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
