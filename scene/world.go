package scene

type World struct {
	nextID EntityID

	Transforms      map[EntityID]Transform
	CircleSprites   map[EntityID]CircleSprite
	Velocities      map[EntityID]Velocity
	Dampings        map[EntityID]Damping
	Bodies          map[EntityID]Body
	CircleColliders map[EntityID]CircleCollider
}

func NewWorld() *World {
	return &World{
		nextID:          1,
		Transforms:      make(map[EntityID]Transform),
		CircleSprites:   make(map[EntityID]CircleSprite),
		Velocities:      make(map[EntityID]Velocity),
		Dampings:        make(map[EntityID]Damping),
		Bodies:          make(map[EntityID]Body),
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
	delete(w.Dampings, id)
	delete(w.Bodies, id)
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

func (w *World) SetDamping(id EntityID, damping Damping) {
	w.Dampings[id] = damping
}

func (w *World) Damping(id EntityID) (Damping, bool) {
	damping, ok := w.Dampings[id]
	return damping, ok
}

func (w *World) RemoveDamping(id EntityID) {
	delete(w.Dampings, id)
}

func (w *World) SetBody(id EntityID, body Body) {
	w.Bodies[id] = body
}

func (w *World) Body(id EntityID) (Body, bool) {
	body, ok := w.Bodies[id]
	return body, ok
}

func (w *World) RemoveBody(id EntityID) {
	delete(w.Bodies, id)
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
