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

	transforms      map[EntityID]Transform
	circleSprites   map[EntityID]CircleSprite
	velocities      map[EntityID]Velocity
	accelerations   map[EntityID]Acceleration
	dampings        map[EntityID]Damping
	bodies          map[EntityID]Body
	circleColliders map[EntityID]CircleCollider
	debugGridCells  []DebugGridCell
	contacts        []CircleContact
}

func NewWorld() *World {
	return &World{
		nextID:          1,
		transforms:      make(map[EntityID]Transform),
		circleSprites:   make(map[EntityID]CircleSprite),
		velocities:      make(map[EntityID]Velocity),
		accelerations:   make(map[EntityID]Acceleration),
		dampings:        make(map[EntityID]Damping),
		bodies:          make(map[EntityID]Body),
		circleColliders: make(map[EntityID]CircleCollider),
	}
}

func (w *World) Clone() *World {
	return &World{
		nextID:          w.nextID,
		transforms:      cloneMap(w.transforms),
		circleSprites:   cloneMap(w.circleSprites),
		velocities:      cloneMap(w.velocities),
		accelerations:   cloneMap(w.accelerations),
		dampings:        cloneMap(w.dampings),
		bodies:          cloneMap(w.bodies),
		circleColliders: cloneMap(w.circleColliders),
		debugGridCells:  append([]DebugGridCell(nil), w.debugGridCells...),
		// contacts are transient per-tick — not cloned
	}
}

func (w *World) Spawn() EntityID {
	id := w.nextID
	w.nextID++

	return id
}

func (w *World) Despawn(id EntityID) {
	delete(w.transforms, id)
	delete(w.circleSprites, id)
	delete(w.velocities, id)
	delete(w.accelerations, id)
	delete(w.dampings, id)
	delete(w.bodies, id)
	delete(w.circleColliders, id)
}

func (w *World) SetTransform(id EntityID, transform Transform) {
	w.transforms[id] = transform
}

func (w *World) SetCircleSprite(id EntityID, sprite CircleSprite) {
	w.circleSprites[id] = sprite
}

func (w *World) Transform(id EntityID) (Transform, bool) {
	transform, ok := w.transforms[id]
	return transform, ok
}

func (w *World) CircleSprite(id EntityID) (CircleSprite, bool) {
	sprite, ok := w.circleSprites[id]
	return sprite, ok
}

func (w *World) RemoveTransform(id EntityID) {
	delete(w.transforms, id)
}

func (w *World) RemoveCircleSprite(id EntityID) {
	delete(w.circleSprites, id)
}

func (w *World) SetVelocity(id EntityID, velocity Velocity) {
	w.velocities[id] = velocity
}

func (w *World) Velocity(id EntityID) (Velocity, bool) {
	velocity, ok := w.velocities[id]
	return velocity, ok
}

func (w *World) RemoveVelocity(id EntityID) {
	delete(w.velocities, id)
}

func (w *World) SetAcceleration(id EntityID, acceleration Acceleration) {
	w.accelerations[id] = acceleration
}

func (w *World) Acceleration(id EntityID) (Acceleration, bool) {
	acceleration, ok := w.accelerations[id]
	return acceleration, ok
}

func (w *World) RemoveAcceleration(id EntityID) {
	delete(w.accelerations, id)
}

func (w *World) SetDamping(id EntityID, damping Damping) {
	w.dampings[id] = damping
}

func (w *World) Damping(id EntityID) (Damping, bool) {
	damping, ok := w.dampings[id]
	return damping, ok
}

func (w *World) RemoveDamping(id EntityID) {
	delete(w.dampings, id)
}

func (w *World) SetBody(id EntityID, body Body) {
	w.bodies[id] = body
}

func (w *World) Body(id EntityID) (Body, bool) {
	body, ok := w.bodies[id]
	return body, ok
}

func (w *World) RemoveBody(id EntityID) {
	delete(w.bodies, id)
}

func (w *World) CircleCollider(id EntityID) (CircleCollider, bool) {
	collider, ok := w.circleColliders[id]
	return collider, ok
}

func (w *World) SetCircleCollider(id EntityID, collider CircleCollider) {
	w.circleColliders[id] = collider
}

func (w *World) RemoveCircleCollider(id EntityID) {
	delete(w.circleColliders, id)
}

func (w *World) DebugGridCells() []DebugGridCell {
	return w.debugGridCells
}

// Contacts returns the circle contacts resolved during the last Step.
func (w *World) Contacts() []CircleContact {
	return w.contacts
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	maps.Copy(dst, src)

	return dst
}
