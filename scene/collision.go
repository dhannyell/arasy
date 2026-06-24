package scene

import "github.com/dhannyell/frameforge/math2d"

func (w *World) BounceCirclesInside(bounds math2d.Rect) {
	for id, collider := range w.CircleColliders {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		velocity, ok := w.Velocities[id]
		if !ok {
			continue
		}

		radius := collider.Radius

		if transform.Position.X-radius < bounds.Min.X {
			transform.Position.X = bounds.Min.X + radius
			velocity.Linear.X = -velocity.Linear.X
		}

		if transform.Position.X+radius > bounds.Max.X {
			transform.Position.X = bounds.Max.X - radius
			velocity.Linear.X = -velocity.Linear.X
		}

		if transform.Position.Y-radius < bounds.Min.Y {
			transform.Position.Y = bounds.Min.Y + radius
			velocity.Linear.Y = -velocity.Linear.Y
		}

		if transform.Position.Y+radius > bounds.Max.Y {
			transform.Position.Y = bounds.Max.Y - radius
			velocity.Linear.Y = -velocity.Linear.Y
		}

		w.Transforms[id] = transform
		w.Velocities[id] = velocity
	}
}
