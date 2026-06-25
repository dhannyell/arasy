package scene

import (
	"math"
	"slices"

	"github.com/dhannyell/frameforge/math2d"
)

type CircleContact struct {
	A EntityID
	B EntityID

	// Normal points from A to B
	Normal math2d.Vec2

	// Depth is how much the two circles overlap.
	Depth float64
}

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

func (w *World) DetectCircleOverlaps() []CircleContact {
	entities := w.circleColliderEntities()
	contacts := make([]CircleContact, 0)

	for i := range entities {
		a := entities[i]

		transformA, ok := w.Transforms[a]
		if !ok {
			continue
		}

		colliderA := w.CircleColliders[a]

		for j := i + 1; j < len(entities); j++ {
			b := entities[j]

			transformB, ok := w.Transforms[b]
			if !ok {
				continue
			}

			colliderB := w.CircleColliders[b]

			delta := transformB.Position.Sub(transformA.Position)
			distanceSq := delta.LenSq()
			radiusSum := colliderA.Radius + colliderB.Radius
			radiusSumSq := radiusSum * radiusSum

			if distanceSq >= radiusSumSq {
				continue
			}

			if distanceSq == 0 {
				contacts = append(contacts, CircleContact{
					A:      a,
					B:      b,
					Normal: math2d.Vec2{X: 1, Y: 0},
					Depth:  radiusSum,
				})
				continue
			}

			distance := math.Sqrt(distanceSq)

			contacts = append(contacts, CircleContact{
				A:      a,
				B:      b,
				Normal: delta.Div(distance),
				Depth:  radiusSum - distance,
			})
		}
	}

	return contacts
}

func (w *World) ResolveCircleOverlaps() {
	contacts := w.DetectCircleOverlaps()

	for _, contact := range contacts {
		transformA, ok := w.Transforms[contact.A]
		if !ok {
			continue
		}

		transformB, ok := w.Transforms[contact.B]
		if !ok {
			continue
		}

		bodyA := w.bodyOrDefault(contact.A)
		bodyB := w.bodyOrDefault(contact.B)
		totalInvMass := bodyA.InvMass + bodyB.InvMass
		if totalInvMass == 0 {
			continue
		}

		correctionA := contact.Depth * (bodyA.InvMass / totalInvMass)
		correctionB := contact.Depth * (bodyB.InvMass / totalInvMass)

		transformA.Position.X -= contact.Normal.X * correctionA
		transformA.Position.Y -= contact.Normal.Y * correctionA
		transformB.Position.X += contact.Normal.X * correctionB
		transformB.Position.Y += contact.Normal.Y * correctionB

		w.Transforms[contact.A] = transformA
		w.Transforms[contact.B] = transformB

		velocityA, ok := w.Velocity(contact.A)
		if !ok {
			continue
		}

		velocityB, ok := w.Velocity(contact.B)
		if !ok {
			continue
		}

		vRel := velocityA.Linear.Sub(velocityB.Linear)
		vRelNormal := (vRel.X * contact.Normal.X) + (vRel.Y * contact.Normal.Y)

		// it means the objects are approaching each other.
		// When the objects are already moving apart, there is no need to update the relative velocity.
		if vRelNormal > 0 {
			restitution := minFloat(bodyA.Restitution, bodyB.Restitution)
			impulseMagnitude := (1 + restitution) * vRelNormal / totalInvMass
			impulse := math2d.Vec2{
				X: impulseMagnitude * contact.Normal.X,
				Y: impulseMagnitude * contact.Normal.Y,
			}

			velocityA.Linear = velocityA.Linear.Sub(impulse.Mul(bodyA.InvMass))
			velocityB.Linear = velocityB.Linear.Add(impulse.Mul(bodyB.InvMass))

			w.Velocities[contact.A] = velocityA
			w.Velocities[contact.B] = velocityB
		}
	}
}

func (w *World) circleColliderEntities() []EntityID {
	entities := make([]EntityID, 0, len(w.CircleColliders))

	for id := range w.CircleColliders {
		entities = append(entities, id)
	}

	slices.Sort(entities)

	return entities
}

func (w *World) bodyOrDefault(id EntityID) Body {
	body, ok := w.Bodies[id]
	if !ok {
		return Body{
			InvMass:     1,
			Restitution: 1,
		}
	}

	if body.InvMass < 0 {
		body.InvMass = 0
	}

	if body.Restitution < 0 {
		body.Restitution = 0
	}

	if body.Restitution > 1 {
		body.Restitution = 1
	}

	return body
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}

	return b
}
