package scene

import "maps"

const defaultCellSize = 64

type PhysicsConfig struct {
	CellSize float64
}

type DebugGridCell struct {
	X         int
	Y         int
	Size      float64
	Candidate bool
}

type World struct {
	nextID EntityID

	Transforms      map[EntityID]Transform
	CircleSprites   map[EntityID]CircleSprite
	Velocities      map[EntityID]Velocity
	Accelerations   map[EntityID]Acceleration
	Dampings        map[EntityID]Damping
	Bodies          map[EntityID]Body
	CircleColliders map[EntityID]CircleCollider
	DebugGridCells  []DebugGridCell
}

func NewWorld() *World {
	return &World{
		nextID:          1,
		Transforms:      make(map[EntityID]Transform),
		CircleSprites:   make(map[EntityID]CircleSprite),
		Velocities:      make(map[EntityID]Velocity),
		Accelerations:   make(map[EntityID]Acceleration),
		Dampings:        make(map[EntityID]Damping),
		Bodies:          make(map[EntityID]Body),
		CircleColliders: make(map[EntityID]CircleCollider),
	}
}

func (w *World) Clone() *World {
	return &World{
		nextID:          w.nextID,
		Transforms:      cloneMap(w.Transforms),
		CircleSprites:   cloneMap(w.CircleSprites),
		Velocities:      cloneMap(w.Velocities),
		Accelerations:   cloneMap(w.Accelerations),
		Dampings:        cloneMap(w.Dampings),
		Bodies:          cloneMap(w.Bodies),
		CircleColliders: cloneMap(w.CircleColliders),
		DebugGridCells:  append([]DebugGridCell(nil), w.DebugGridCells...),
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
	delete(w.Accelerations, id)
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

func (w *World) SetAcceleration(id EntityID, acceleration Acceleration) {
	w.Accelerations[id] = acceleration
}

func (w *World) Acceleration(id EntityID) (Acceleration, bool) {
	acceleration, ok := w.Accelerations[id]
	return acceleration, ok
}

func (w *World) RemoveAcceleration(id EntityID) {
	delete(w.Accelerations, id)
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

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	maps.Copy(dst, src)

	return dst
}
