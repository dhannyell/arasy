package scene

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

type World struct {
	nextID EntityID

	Transforms      map[EntityID]Transform
	CircleSprites   map[EntityID]CircleSprite
	Velocities      map[EntityID]Velocity
	CircleColliders map[EntityID]CircleCollider
}

func NewWorld() *World {
	return &World{
		nextID:          1,
		Transforms:      make(map[EntityID]Transform),
		CircleSprites:   make(map[EntityID]CircleSprite),
		Velocities:      make(map[EntityID]Velocity),
		CircleColliders: make(map[EntityID]CircleCollider),
	}
}

func (w *World) Spawn() EntityID {
	id := w.nextID
	w.nextID++

	return id
}

func (w *World) Despawn(id EntityID) {
	delete(w.Transforms, id)
	delete(w.CircleSprites, id)
	delete(w.Velocities, id)
	delete(w.CircleColliders, id)
}

func (w *World) SetTransform(id EntityID, transform Transform) {
	w.Transforms[id] = transform
}

func (w *World) SetCircleSprite(id EntityID, sprite CircleSprite) {
	w.CircleSprites[id] = sprite
}

func (w *World) Transform(id EntityID) (Transform, bool) {
	transform, ok := w.Transforms[id]
	return transform, ok
}

func (w *World) CircleSprite(id EntityID) (CircleSprite, bool) {
	sprite, ok := w.CircleSprites[id]
	return sprite, ok
}

func (w *World) RemoveTransform(id EntityID) {
	delete(w.Transforms, id)
}

func (w *World) RemoveCircleSprite(id EntityID) {
	delete(w.CircleSprites, id)
}

func (w *World) SetVelocity(id EntityID, velocity Velocity) {
	w.Velocities[id] = velocity
}

func (w *World) Velocity(id EntityID) (Velocity, bool) {
	velocity, ok := w.Velocities[id]
	return velocity, ok
}

func (w *World) RemoveVelocity(id EntityID) {
	delete(w.Velocities, id)
}

func (w *World) CircleCollider(id EntityID) (CircleCollider, bool) {
	collider, ok := w.CircleColliders[id]
	return collider, ok
}

func (w *World) SetCircleCollider(id EntityID, collider CircleCollider) {
	w.CircleColliders[id] = collider
}

func (w *World) RemoveCircleCollider(id EntityID) {
	delete(w.CircleColliders, id)
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

func (w *World) RenderCommands() []render.Command {
	commands := make([]render.Command, 0, len(w.CircleSprites))

	for id, sprite := range w.CircleSprites {
		transform, ok := w.Transforms[id]
		if !ok {
			continue
		}

		commands = append(commands, render.DrawCircle{
			Center: transform.Position,
			Radius: sprite.Radius,
			Fill:   sprite.Color,
			Layer:  sprite.Layer,
		})
	}

	return commands
}
