package scene

import "hash"

// stateClass places a component store in exactly one checksum class.
// SimChecksum hashes simState stores, ContentChecksum hashes contentState
// stores, and renderOnlyState stores are never hashed.
type stateClass byte

const (
	simState stateClass = iota
	contentState
	renderOnlyState
)

// componentDef registers one component store for the world-wide passes:
// Clone, Despawn, DespawnBatch and checksum. Each store has exactly one
// entry in componentDefs; the structural guard tests enforce the count.
type componentDef struct {
	tag       byte
	sim       bool
	content   bool
	skipEmpty bool

	length       func(w *World) int
	clone        func(dst, src *World)
	remove       func(w *World, id EntityID)
	removeMarked func(w *World, marked map[EntityID]struct{})
	// hash is nil for renderOnlyState stores.
	hash func(w *World, h hash.Hash64, buf []byte)
}

// def builds the componentDef for one store. field resolves the store on a
// concrete world, so one table entry serves every World instance.
// hashFields writes the fields of one component value; it is nil for
// renderOnlyState stores, which leaves hash nil.
func def[T any](field func(w *World) *store[T], tag byte, class stateClass,
	hashFields func(h hash.Hash64, buf []byte, v T)) componentDef {
	var isSim, isContent bool
	switch class {
	case simState:
		isSim = true
	case contentState:
		isContent = true
	}

	component := componentDef{
		tag:          tag,
		sim:          isSim,
		content:      isContent,
		clone:        func(dst, src *World) { *field(dst) = field(src).clone() },
		length:       func(w *World) int { return field(w).len() },
		remove:       func(w *World, id EntityID) { field(w).remove(id) },
		removeMarked: func(w *World, marked map[EntityID]struct{}) { field(w).removeMarked(marked) },
	}

	if hashFields != nil {
		component.hash = func(w *World, h hash.Hash64, buf []byte) {
			hashComponent(h, buf, tag, field(w), func(f T) {
				hashFields(h, buf, f)
			})
		}
	}
	return component
}

// renderDef registers a presentation store: cloned and despawned like any
// store, never hashed.
func renderDef[T any](field func(w *World) *store[T]) componentDef {
	return def(field, 0, renderOnlyState, nil)
}

// skipWhenEmpty preserves the historical checksum stream for a store that
// shipped with an empty-store guard. Do not use it for new components.
func skipWhenEmpty(d componentDef) componentDef {
	d.skipEmpty = true
	return d
}

// componentDefs is the component registry. Hashed entries keep the
// historical checksum order: an entry may change class, never position,
// and new entries append at the end of their section. The checksum goldens
// pin the resulting byte stream.
var componentDefs = []componentDef{
	def(func(w *World) *store[Transform] { return &w.transforms }, tagTransform, simState,
		func(h hash.Hash64, buf []byte, t Transform) {
			hashF64(h, buf, t.Position.X)
			hashF64(h, buf, t.Position.Y)
			hashF64(h, buf, t.Rotation)
			hashF64(h, buf, t.Scale.X)
			hashF64(h, buf, t.Scale.Y)
		}),
	def(func(w *World) *store[TransformTween] { return &w.transformTweens }, tagTransformTween, simState,
		func(h hash.Hash64, buf []byte, tw TransformTween) {
			hashF64(h, buf, tw.From.Rotation)
			hashF64(h, buf, tw.From.Scale.X)
			hashF64(h, buf, tw.From.Scale.Y)
			hashF64(h, buf, tw.From.Position.X)
			hashF64(h, buf, tw.From.Position.Y)
			hashF64(h, buf, tw.To.Rotation)
			hashF64(h, buf, tw.To.Scale.X)
			hashF64(h, buf, tw.To.Scale.Y)
			hashF64(h, buf, tw.To.Position.X)
			hashF64(h, buf, tw.To.Position.Y)
			hashF64(h, buf, tw.Duration)
			hashF64(h, buf, tw.Elapsed)
			hashU64(h, buf, uint64(tw.Ease))
			hashU64(h, buf, uint64(tw.Loop))
			hashF64(h, buf, boolToFloat(tw.Done))
			hashF64(h, buf, boolToFloat(tw.RemoveOnComplete))
		}),
	def(func(w *World) *store[CircleSprite] { return &w.circleSprites }, tagCircleSprite, contentState,
		func(h hash.Hash64, buf []byte, s CircleSprite) {
			hashCircleSprite(h, buf, s)
		}),
	def(func(w *World) *store[CircleSpriteTween] { return &w.circleTweens }, tagCircleSpriteTween, simState,
		func(h hash.Hash64, buf []byte, tw CircleSpriteTween) {
			hashCircleSprite(h, buf, tw.From)
			hashCircleSprite(h, buf, tw.To)
			hashF64(h, buf, tw.Duration)
			hashF64(h, buf, tw.Elapsed)
			hashU64(h, buf, uint64(tw.Ease))
			hashU64(h, buf, uint64(tw.Loop))
			hashF64(h, buf, boolToFloat(tw.Done))
			hashF64(h, buf, boolToFloat(tw.RemoveOnComplete))
		}),
	def(func(w *World) *store[RectSprite] { return &w.rectSprites }, tagRectSprite, contentState,
		func(h hash.Hash64, buf []byte, s RectSprite) {
			hashU64(h, buf, uint64(int64(s.Layer)))
			hashF64(h, buf, s.Color.R)
			hashF64(h, buf, s.Color.G)
			hashF64(h, buf, s.Color.B)
			hashF64(h, buf, s.Color.A)
			hashF64(h, buf, s.Rect.Min.X)
			hashF64(h, buf, s.Rect.Min.Y)
			hashF64(h, buf, s.Rect.Max.X)
			hashF64(h, buf, s.Rect.Max.Y)
			hashF64(h, buf, s.StrokeWidth)
			hashF64(h, buf, s.Stroke.R)
			hashF64(h, buf, s.Stroke.G)
			hashF64(h, buf, s.Stroke.B)
			hashF64(h, buf, s.Stroke.A)
		}),
	def(func(w *World) *store[Velocity] { return &w.velocities }, tagVelocity, simState,
		func(h hash.Hash64, buf []byte, v Velocity) {
			hashF64(h, buf, v.Linear.X)
			hashF64(h, buf, v.Linear.Y)
		}),
	def(func(w *World) *store[Acceleration] { return &w.accelerations }, tagAcceleration, simState,
		func(h hash.Hash64, buf []byte, a Acceleration) {
			hashF64(h, buf, a.Linear.X)
			hashF64(h, buf, a.Linear.Y)
		}),
	def(func(w *World) *store[Damping] { return &w.dampings }, tagDamping, simState,
		func(h hash.Hash64, buf []byte, d Damping) {
			hashF64(h, buf, d.Linear)
		}),
	def(func(w *World) *store[Body] { return &w.bodies }, tagBody, simState,
		func(h hash.Hash64, buf []byte, b Body) {
			hashF64(h, buf, b.InvMass)
			hashF64(h, buf, b.Restitution)
		}),
	def(func(w *World) *store[CircleCollider] { return &w.circleColliders }, tagCircleCollider, simState,
		func(h hash.Hash64, buf []byte, c CircleCollider) {
			hashF64(h, buf, c.Radius)
		}),
	def(func(w *World) *store[RectCollider] { return &w.rectColliders }, tagRectCollider, simState,
		func(h hash.Hash64, buf []byte, c RectCollider) {
			hashF64(h, buf, c.HalfExtends.X)
			hashF64(h, buf, c.HalfExtends.Y)
		}),
	def(func(w *World) *store[Height] { return &w.heights }, tagHeight, simState,
		func(h hash.Hash64, buf []byte, v Height) {
			hashF64(h, buf, v.Z)
			hashF64(h, buf, v.VZ)
		}),
	def(func(w *World) *store[Spin] { return &w.spins }, tagSpin, simState,
		func(h hash.Hash64, buf []byte, s Spin) {
			hashF64(h, buf, s.Omega)
		}),
	def(func(w *World) *store[Ballistics] { return &w.ballistics }, tagBallistics, simState,
		func(h hash.Hash64, buf []byte, b Ballistics) {
			hashF64(h, buf, b.Gravity)
			hashF64(h, buf, b.DragK)
			hashF64(h, buf, b.MagnusK)
			hashF64(h, buf, b.BounceDamping)
			hashF64(h, buf, b.BounceFriction)
			hashF64(h, buf, b.SpinDecay)
			hashF64(h, buf, b.RollFriction)
			hashF64(h, buf, b.RestVZ)
			hashU64(h, buf, uint64(int64(b.Substeps)))
		}),
	def(func(w *World) *store[Locomotion] { return &w.locomotions }, tagLocomotion, simState,
		func(h hash.Hash64, buf []byte, l Locomotion) {
			hashF64(h, buf, l.Accel)
			hashF64(h, buf, l.Decel)
			hashF64(h, buf, l.MaxSpeed)
			hashF64(h, buf, l.Agility)
		}),
	def(func(w *World) *store[MoveIntent] { return &w.moveIntents }, tagMoveIntent, simState,
		func(h hash.Hash64, buf []byte, m MoveIntent) {
			hashF64(h, buf, m.Target.X)
			hashF64(h, buf, m.Target.Y)
			hashF64(h, buf, m.DesiredSpeed)
		}),
	def(func(w *World) *store[HopTween] { return &w.hopTweens }, tagHopTween, simState,
		func(h hash.Hash64, buf []byte, tw HopTween) {
			hashF64(h, buf, tw.From.X)
			hashF64(h, buf, tw.From.Y)
			hashF64(h, buf, tw.To.X)
			hashF64(h, buf, tw.To.Y)
			hashF64(h, buf, tw.Apex)
			hashF64(h, buf, tw.Duration)
			hashF64(h, buf, tw.Elapsed)
		}),
	skipWhenEmpty(def(func(w *World) *store[ImageSprite] { return &w.imageSprites }, tagImageSprite, contentState,
		func(h hash.Hash64, buf []byte, s ImageSprite) {
			hashU64(h, buf, uint64(int64(s.Layer)))
			hashU64(h, buf, uint64(s.Texture))
			hashF64(h, buf, s.Src.Min.X)
			hashF64(h, buf, s.Src.Min.Y)
			hashF64(h, buf, s.Src.Max.X)
			hashF64(h, buf, s.Src.Max.Y)
			hashF64(h, buf, s.Size.X)
			hashF64(h, buf, s.Size.Y)
			hashF64(h, buf, s.Color.R)
			hashF64(h, buf, s.Color.G)
			hashF64(h, buf, s.Color.B)
			hashF64(h, buf, s.Color.A)
			hashF64(h, buf, s.ShadowFootprint.X)
			hashF64(h, buf, s.ShadowFootprint.Y)
		})),

	// Presentation stores: cloned and despawned, never hashed.
	renderDef(func(w *World) *store[SpriteTransform] { return &w.spriteTransforms }),
	renderDef(func(w *World) *store[ShadowStyle] { return &w.shadowStyles }),
	renderDef(func(w *World) *store[Attachment] { return &w.attachments }),
	renderDef(func(w *World) *store[PointLight] { return &w.pointLights }),
	renderDef(func(w *World) *store[CommandSprite] { return &w.commandSprites }),
	renderDef(func(w *World) *store[Transform] { return &w.renderTransforms }),
	renderDef(func(w *World) *store[Height] { return &w.renderHeights }),
}
