package scene

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

	transforms       store[Transform]
	transformTweens  store[TransformTween]
	circleSprites    store[CircleSprite]
	circleTweens     store[CircleSpriteTween]
	rectSprites      store[RectSprite]
	imageSprites     store[ImageSprite]
	velocities       store[Velocity]
	accelerations    store[Acceleration]
	dampings         store[Damping]
	bodies           store[Body]
	circleColliders  store[CircleCollider]
	rectColliders    store[RectCollider]
	debugGridCells   []DebugGridCell
	contacts         []Contact
	collisionScratch collisionScratch

	entityCountCache int
	entityCountDirty bool

	// renderTransforms stores the previous-tick transform for each entity,
	// used by RenderCommandsInterpolated to produce smooth visual poses
	// between fixed simulation ticks. It is a presentation cache only and
	// is NOT included in Checksum.
	renderTransforms store[Transform]

	// renderIDScratch is reusable render working memory. It is transient:
	// never cloned or checksummed.
	renderIDScratch []EntityID
}

func NewWorld() *World {
	return &World{
		nextID: 1,
	}
}

// Clone returns a deep copy of the world. It is cheap: component stores copy
// only their dense data, not the sparse index, which each store rebuilds lazily
// on its first lookup. As a consequence the returned world is not safe for
// concurrent access until it has been read once on a single goroutine — the
// first getter or Step call rebuilds sparse indexes and thus mutates state.
// See the store type for details.
func (w *World) Clone() *World {
	return &World{
		nextID:           w.nextID,
		stepTick:         w.stepTick,
		transforms:       w.transforms.clone(),
		transformTweens:  w.transformTweens.clone(),
		circleSprites:    w.circleSprites.clone(),
		circleTweens:     w.circleTweens.clone(),
		rectSprites:      w.rectSprites.clone(),
		imageSprites:     w.imageSprites.clone(),
		velocities:       w.velocities.clone(),
		accelerations:    w.accelerations.clone(),
		dampings:         w.dampings.clone(),
		bodies:           w.bodies.clone(),
		circleColliders:  w.circleColliders.clone(),
		rectColliders:    w.rectColliders.clone(),
		debugGridCells:   append([]DebugGridCell(nil), w.debugGridCells...),
		entityCountCache: w.entityCountCache,
		entityCountDirty: w.entityCountDirty,
		renderTransforms: w.renderTransforms.clone(),
		// contacts are transient per-tick — not cloned
		// collisionScratch is transient broad-phase working memory — not cloned
		// renderIDScratch is transient render working memory — not cloned
	}
}

func (w *World) Spawn() EntityID {
	id := w.nextID
	w.nextID++

	return id
}

func (w *World) Despawn(id EntityID) {
	w.markEntityCountDirty()
	w.transforms.remove(id)
	w.transformTweens.remove(id)
	w.circleSprites.remove(id)
	w.circleTweens.remove(id)
	w.rectSprites.remove(id)
	w.imageSprites.remove(id)
	w.velocities.remove(id)
	w.accelerations.remove(id)
	w.dampings.remove(id)
	w.bodies.remove(id)
	w.circleColliders.remove(id)
	w.rectColliders.remove(id)
	w.renderTransforms.remove(id)
}

func (w *World) Transform(id EntityID) (Transform, bool) {
	transform, ok := w.transforms.get(id)
	return transform, ok
}

// SetTransform applies an immediate transform and cancels any active transform tween.
func (w *World) SetTransform(id EntityID, transform Transform) {
	w.markEntityCountDirty()
	w.transformTweens.remove(id)
	w.transforms.set(id, transform)
	// Sync render state immediately to prevent visual smear on teleport.
	w.renderTransforms.set(id, transform)
}

func (w *World) RemoveTransform(id EntityID) {
	w.markEntityCountDirty()
	w.transformTweens.remove(id)
	w.transforms.remove(id)
	w.renderTransforms.remove(id)
}

func (w *World) TransformTween(id EntityID) (TransformTween, bool) {
	transformTween, ok := w.transformTweens.get(id)
	return transformTween, ok
}

// SetTransformTween is a low-level API. Prefer TweenTransform unless you need
// explicit From/Elapsed state, such as loading snapshots or tests.
func (w *World) SetTransformTween(id EntityID, tw TransformTween) {
	w.markEntityCountDirty()
	w.transformTweens.set(id, tw)
}

// TweenTransform animates an entity from its current Transform to target.
// It returns false when the entity has no Transform to animate.
func (w *World) TweenTransform(id EntityID, target Transform, opts TransformTweenOptions) bool {
	from, ok := w.transforms.get(id)
	if !ok {
		return false
	}
	if opts.StopVelocity {
		w.markEntityCountDirty()
		w.velocities.set(id, Velocity{})
	}
	w.markEntityCountDirty()
	w.transformTweens.set(id, TransformTween{
		From:             from,
		To:               target,
		Duration:         opts.Duration,
		Ease:             opts.Ease,
		Loop:             opts.Loop,
		RemoveOnComplete: opts.RemoveOnComplete,
	})
	return true
}

func (w *World) RemoveTransformTween(id EntityID) {
	w.markEntityCountDirty()
	w.transformTweens.remove(id)
}

func (w *World) SetCircleSprite(id EntityID, sprite CircleSprite) {
	w.markEntityCountDirty()
	w.circleTweens.remove(id)
	w.circleSprites.set(id, sprite)
}

func (w *World) RemoveCircleSprite(id EntityID) {
	w.markEntityCountDirty()
	w.circleTweens.remove(id)
	w.circleSprites.remove(id)
}

func (w *World) CircleSprite(id EntityID) (CircleSprite, bool) {
	sprite, ok := w.circleSprites.get(id)
	return sprite, ok
}

func (w *World) CircleSpriteTween(id EntityID) (CircleSpriteTween, bool) {
	tween, ok := w.circleTweens.get(id)
	return tween, ok
}

// SetCircleSpriteTween is a low-level API. Prefer TweenCircleSprite unless you
// need explicit From/Elapsed state, such as loading snapshots or tests.
func (w *World) SetCircleSpriteTween(id EntityID, tw CircleSpriteTween) {
	w.markEntityCountDirty()
	w.circleTweens.set(id, tw)
}

// TweenCircleSprite animates an entity from its current CircleSprite to target.
// It returns false when the entity has no CircleSprite to animate.
func (w *World) TweenCircleSprite(id EntityID, target CircleSprite, opts CircleSpriteTweenOptions) bool {
	from, ok := w.circleSprites.get(id)
	if !ok {
		return false
	}
	w.markEntityCountDirty()
	w.circleTweens.set(id, CircleSpriteTween{
		From:             from,
		To:               target,
		Duration:         opts.Duration,
		Ease:             opts.Ease,
		Loop:             opts.Loop,
		RemoveOnComplete: opts.RemoveOnComplete,
	})
	return true
}

func (w *World) RemoveCircleSpriteTween(id EntityID) {
	w.markEntityCountDirty()
	w.circleTweens.remove(id)
}

func (w *World) SetRectSprite(id EntityID, sprite RectSprite) {
	w.markEntityCountDirty()
	w.rectSprites.set(id, sprite)
}

func (w *World) RemoveRectSprite(id EntityID) {
	w.markEntityCountDirty()
	w.rectSprites.remove(id)
}

func (w *World) RectSprite(id EntityID) (RectSprite, bool) {
	sprite, ok := w.rectSprites.get(id)
	return sprite, ok
}

func (w *World) SetImageSprite(id EntityID, sprite ImageSprite) {
	w.markEntityCountDirty()
	w.imageSprites.set(id, sprite)
}

func (w *World) RemoveImageSprite(id EntityID) {
	w.markEntityCountDirty()
	w.imageSprites.remove(id)
}

func (w *World) ImageSprite(id EntityID) (ImageSprite, bool) {
	sprite, ok := w.imageSprites.get(id)
	return sprite, ok
}

func (w *World) SetVelocity(id EntityID, velocity Velocity) {
	w.markEntityCountDirty()
	w.velocities.set(id, velocity)
}

func (w *World) Velocity(id EntityID) (Velocity, bool) {
	velocity, ok := w.velocities.get(id)
	return velocity, ok
}

func (w *World) RemoveVelocity(id EntityID) {
	w.markEntityCountDirty()
	w.velocities.remove(id)
}

func (w *World) SetAcceleration(id EntityID, acceleration Acceleration) {
	w.markEntityCountDirty()
	w.accelerations.set(id, acceleration)
}

func (w *World) Acceleration(id EntityID) (Acceleration, bool) {
	acceleration, ok := w.accelerations.get(id)
	return acceleration, ok
}

func (w *World) RemoveAcceleration(id EntityID) {
	w.markEntityCountDirty()
	w.accelerations.remove(id)
}

func (w *World) SetDamping(id EntityID, damping Damping) {
	w.markEntityCountDirty()
	w.dampings.set(id, damping)
}

func (w *World) Damping(id EntityID) (Damping, bool) {
	damping, ok := w.dampings.get(id)
	return damping, ok
}

func (w *World) RemoveDamping(id EntityID) {
	w.markEntityCountDirty()
	w.dampings.remove(id)
}

func (w *World) SetBody(id EntityID, body Body) {
	w.markEntityCountDirty()
	w.bodies.set(id, body)
}

func (w *World) Body(id EntityID) (Body, bool) {
	body, ok := w.bodies.get(id)
	return body, ok
}

func (w *World) RemoveBody(id EntityID) {
	w.markEntityCountDirty()
	w.bodies.remove(id)
}

func (w *World) CircleCollider(id EntityID) (CircleCollider, bool) {
	collider, ok := w.circleColliders.get(id)
	return collider, ok
}

func (w *World) SetCircleCollider(id EntityID, collider CircleCollider) {
	w.markEntityCountDirty()
	w.circleColliders.set(id, collider)
}

func (w *World) RemoveCircleCollider(id EntityID) {
	w.markEntityCountDirty()
	w.circleColliders.remove(id)
}

func (w *World) RectCollider(id EntityID) (RectCollider, bool) {
	collider, ok := w.rectColliders.get(id)
	return collider, ok
}

func (w *World) SetRectCollider(id EntityID, collider RectCollider) {
	w.markEntityCountDirty()
	w.rectColliders.set(id, collider)
}

func (w *World) RemoveRectCollider(id EntityID) {
	w.markEntityCountDirty()
	w.rectColliders.remove(id)
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
	if !w.entityCountDirty {
		return w.entityCountCache
	}

	ids := make(map[EntityID]struct{})

	for _, id := range w.transforms.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.circleSprites.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.circleTweens.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.rectSprites.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.imageSprites.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.velocities.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.accelerations.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.dampings.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.bodies.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.circleColliders.ids {
		ids[id] = struct{}{}
	}
	for _, id := range w.rectColliders.ids {
		ids[id] = struct{}{}
	}

	w.entityCountCache = len(ids)
	w.entityCountDirty = false

	return w.entityCountCache
}

func (w *World) markEntityCountDirty() {
	w.entityCountDirty = true
}
