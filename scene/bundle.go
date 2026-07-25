package scene

import "github.com/dhannyell/arasy/math2d"

type Bundle struct {
	Transform      *Transform
	CircleSprite   *CircleSprite
	RectSprite     *RectSprite
	ImageSprite    *ImageSprite
	Velocity       *Velocity
	Acceleration   *Acceleration
	Damping        *Damping
	Body           *Body
	CircleCollider *CircleCollider
	RectCollider   *RectCollider
}

func (w *World) SpawnBundle(b Bundle) EntityID {
	id := w.Spawn()
	if b.Transform != nil {
		t := *b.Transform
		if t.Scale == (math2d.Vec2{}) {
			t.Scale = math2d.Vec2{X: 1, Y: 1}
		}
		w.SetTransform(id, t)
	}
	if b.CircleSprite != nil {
		w.SetCircleSprite(id, *b.CircleSprite)
	}
	if b.RectSprite != nil {
		w.SetRectSprite(id, *b.RectSprite)
	}
	if b.ImageSprite != nil {
		w.SetImageSprite(id, *b.ImageSprite)
	}
	if b.Velocity != nil {
		w.SetVelocity(id, *b.Velocity)
	}
	if b.Acceleration != nil {
		w.SetAcceleration(id, *b.Acceleration)
	}
	if b.Damping != nil {
		w.SetDamping(id, *b.Damping)
	}
	if b.Body != nil {
		w.SetBody(id, *b.Body)
	}
	if b.CircleCollider != nil {
		w.SetCircleCollider(id, *b.CircleCollider)
	}
	if b.RectCollider != nil {
		w.SetRectCollider(id, *b.RectCollider)
	}
	return id
}
