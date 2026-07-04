package scene

import (
	"maps"
)

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

	stepTick uint64

	transforms      map[EntityID]Transform
	transformTweens map[EntityID]TransformTween
	circleSprites   map[EntityID]CircleSprite
	circleTweens    map[EntityID]CircleSpriteTween
	rectSprites     map[EntityID]RectSprite
	velocities      map[EntityID]Velocity
	accelerations   map[EntityID]Acceleration
	dampings        map[EntityID]Damping
	bodies          map[EntityID]Body
	circleColliders map[EntityID]CircleCollider
	rectColliders   map[EntityID]RectCollider
	debugGridCells  []DebugGridCell
	contacts        []Contact
}

func NewWorld() *World {
	return &World{
		nextID:          1,
		transforms:      make(map[EntityID]Transform),
		transformTweens: make(map[EntityID]TransformTween),
		circleSprites:   make(map[EntityID]CircleSprite),
		circleTweens:    make(map[EntityID]CircleSpriteTween),
		rectSprites:     make(map[EntityID]RectSprite),
		velocities:      make(map[EntityID]Velocity),
		accelerations:   make(map[EntityID]Acceleration),
		dampings:        make(map[EntityID]Damping),
		bodies:          make(map[EntityID]Body),
		circleColliders: make(map[EntityID]CircleCollider),
		rectColliders:   make(map[EntityID]RectCollider),
	}
}

func (w *World) Clone() *World {
	return &World{
		nextID:          w.nextID,
		stepTick:        w.stepTick,
		transforms:      cloneMap(w.transforms),
		transformTweens: cloneMap(w.transformTweens),
		circleSprites:   cloneMap(w.circleSprites),
		circleTweens:    cloneMap(w.circleTweens),
		rectSprites:     cloneMap(w.rectSprites),
		velocities:      cloneMap(w.velocities),
		accelerations:   cloneMap(w.accelerations),
		dampings:        cloneMap(w.dampings),
		bodies:          cloneMap(w.bodies),
		circleColliders: cloneMap(w.circleColliders),
		rectColliders:   cloneMap(w.rectColliders),
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
	delete(w.transformTweens, id)
	delete(w.circleSprites, id)
	delete(w.circleTweens, id)
	delete(w.rectSprites, id)
	delete(w.velocities, id)
	delete(w.accelerations, id)
	delete(w.dampings, id)
	delete(w.bodies, id)
	delete(w.circleColliders, id)
	delete(w.rectColliders, id)
}

func (w *World) Transform(id EntityID) (Transform, bool) {
	transform, ok := w.transforms[id]
	return transform, ok
}

// SetTransform applies an immediate transform and cancels any active transform tween.
func (w *World) SetTransform(id EntityID, transform Transform) {
	delete(w.transformTweens, id)
	w.transforms[id] = transform
}

func (w *World) RemoveTransform(id EntityID) {
	delete(w.transformTweens, id)
	delete(w.transforms, id)
}

func (w *World) TransformTween(id EntityID) (TransformTween, bool) {
	transformTween, ok := w.transformTweens[id]
	return transformTween, ok
}

// SetTransformTween is a low-level API. Prefer TweenTransform unless you need
// explicit From/Elapsed state, such as loading snapshots or tests.
func (w *World) SetTransformTween(id EntityID, tw TransformTween) {
	w.transformTweens[id] = tw
}

// TweenTransform animates an entity from its current Transform to target.
// It returns false when the entity has no Transform to animate.
func (w *World) TweenTransform(id EntityID, target Transform, opts TransformTweenOptions) bool {
	from, ok := w.transforms[id]
	if !ok {
		return false
	}
	if opts.StopVelocity {
		w.velocities[id] = Velocity{}
	}
	w.transformTweens[id] = TransformTween{
		From:             from,
		To:               target,
		Duration:         opts.Duration,
		Ease:             opts.Ease,
		Loop:             opts.Loop,
		RemoveOnComplete: opts.RemoveOnComplete,
	}
	return true
}

func (w *World) RemoveTransformTween(id EntityID) {
	delete(w.transformTweens, id)
}

func (w *World) SetCircleSprite(id EntityID, sprite CircleSprite) {
	delete(w.circleTweens, id)
	w.circleSprites[id] = sprite
}

func (w *World) RemoveCircleSprite(id EntityID) {
	delete(w.circleTweens, id)
	delete(w.circleSprites, id)
}

func (w *World) CircleSprite(id EntityID) (CircleSprite, bool) {
	sprite, ok := w.circleSprites[id]
	return sprite, ok
}

func (w *World) CircleSpriteTween(id EntityID) (CircleSpriteTween, bool) {
	tween, ok := w.circleTweens[id]
	return tween, ok
}

// SetCircleSpriteTween is a low-level API. Prefer TweenCircleSprite unless you
// need explicit From/Elapsed state, such as loading snapshots or tests.
func (w *World) SetCircleSpriteTween(id EntityID, tw CircleSpriteTween) {
	w.circleTweens[id] = tw
}

// TweenCircleSprite animates an entity from its current CircleSprite to target.
// It returns false when the entity has no CircleSprite to animate.
func (w *World) TweenCircleSprite(id EntityID, target CircleSprite, opts CircleSpriteTweenOptions) bool {
	from, ok := w.circleSprites[id]
	if !ok {
		return false
	}
	w.circleTweens[id] = CircleSpriteTween{
		From:             from,
		To:               target,
		Duration:         opts.Duration,
		Ease:             opts.Ease,
		Loop:             opts.Loop,
		RemoveOnComplete: opts.RemoveOnComplete,
	}
	return true
}

func (w *World) RemoveCircleSpriteTween(id EntityID) {
	delete(w.circleTweens, id)
}

func (w *World) SetRectSprite(id EntityID, sprite RectSprite) {
	w.rectSprites[id] = sprite
}

func (w *World) RemoveRectSprite(id EntityID) {
	delete(w.rectSprites, id)
}

func (w *World) RectSprite(id EntityID) (RectSprite, bool) {
	sprite, ok := w.rectSprites[id]
	return sprite, ok
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

func (w *World) RectCollider(id EntityID) (RectCollider, bool) {
	collider, ok := w.rectColliders[id]
	return collider, ok
}

func (w *World) SetRectCollider(id EntityID, collider RectCollider) {
	w.rectColliders[id] = collider
}

func (w *World) RemoveRectCollider(id EntityID) {
	delete(w.rectColliders, id)
}

func (w *World) debugGridCellsSnapshot() []DebugGridCell {
	return w.debugGridCells
}

// StepTick returns how many simulation steps this world has run. Runtime uses
// it to detect games that still call Step manually from Update.
func (w *World) StepTick() uint64 {
	return w.stepTick
}

// Contacts returns the contacts resolved during the last Step, across all
// collider shape combinations.
func (w *World) Contacts() []Contact {
	return w.contacts
}

func (w *World) entityCount() int {
	ids := make(map[EntityID]struct{})

	for id := range w.transforms {
		ids[id] = struct{}{}
	}
	for id := range w.circleSprites {
		ids[id] = struct{}{}
	}
	for id := range w.circleTweens {
		ids[id] = struct{}{}
	}
	for id := range w.rectSprites {
		ids[id] = struct{}{}
	}
	for id := range w.velocities {
		ids[id] = struct{}{}
	}
	for id := range w.accelerations {
		ids[id] = struct{}{}
	}
	for id := range w.dampings {
		ids[id] = struct{}{}
	}
	for id := range w.bodies {
		ids[id] = struct{}{}
	}
	for id := range w.circleColliders {
		ids[id] = struct{}{}
	}
	for id := range w.rectColliders {
		ids[id] = struct{}{}
	}

	return len(ids)
}

func cloneMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	maps.Copy(dst, src)

	return dst
}
