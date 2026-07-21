package scene

import (
	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

type EntityID uint32

type Transform struct {
	Position math2d.Vec2
	Rotation float64
	Scale    math2d.Vec2
}

type BaseSprite struct {
	Layer int
	Color render.Color
}

type CircleSprite struct {
	BaseSprite
	Radius float64
}

type RectSprite struct {
	BaseSprite
	Rect        math2d.Rect
	Stroke      render.Color
	StrokeWidth float64
}

// ImageSprite draws a texture, or a sub-rectangle of it, anchored at the
// entity's Transform. Size is the sprite's width and height in world units at
// Transform.Scale == {1, 1}; Transform.Rotation rotates it around its center.
// Texture and Src must already be resolved through asset.Manifest because the
// scene package stays independent of asset loading.
type ImageSprite struct {
	Layer   int
	Texture render.TextureID
	Src     math2d.Rect
	Size    math2d.Vec2
	Tint    render.Color
}

type Velocity struct {
	Linear math2d.Vec2
}

type Acceleration struct {
	Linear math2d.Vec2
}

type Damping struct {
	Linear float64
}

type Body struct {
	// InvMass is the inverse mass. A value of 0 makes the body static.
	InvMass float64
	// Restitution controls bounciness. 0 absorbs normal velocity, 1 preserves it.
	Restitution float64
}

type CircleCollider struct {
	Radius float64
}

type RectCollider struct {
	// HalfExtends are the half-dimensions of the rectangle, centered at
	// Transform.Position. Transform.Rotation is ignored (AABB).
	HalfExtends math2d.Vec2
}

type LoopMode uint8

const (
	LoopOnce LoopMode = iota
	LoopRepeat
	LoopPingPong
)

// TransformTween animates the Position/Rotation/Scale of a Transform.
type TransformTween struct {
	From, To         Transform
	Elapsed          float64
	Duration         float64
	Ease             ease.Kind
	Loop             LoopMode
	RemoveOnComplete bool
	Done             bool
}

// TransformTweenOptions configures World.TweenTransform, the high-level API
// for animating from an entity's current Transform.
type TransformTweenOptions struct {
	Duration         float64
	Ease             ease.Kind
	Loop             LoopMode
	RemoveOnComplete bool
	// StopVelocity clears Velocity before installing the tween, avoiding the
	// common mistake of driving the same Transform through physics and tweening.
	StopVelocity bool
}

// CircleSpriteTween animates the visual radius/color of a CircleSprite.
type CircleSpriteTween struct {
	From, To         CircleSprite
	Elapsed          float64
	Duration         float64
	Ease             ease.Kind
	Loop             LoopMode
	RemoveOnComplete bool
	Done             bool
}

// CircleSpriteTweenOptions configures World.TweenCircleSprite.
type CircleSpriteTweenOptions struct {
	Duration         float64
	Ease             ease.Kind
	Loop             LoopMode
	RemoveOnComplete bool
}

// Height gives an entity a scalar altitude above the 2D plane. Z is
// simulation state (it participates in checksum and cloning); visual
// presentation derives from it via render.Projection.
type Height struct {
	Z, VZ float64
}

// Spin is the angular velocity around the vertical axis, used by the
// Magnus term of Ballistics to curve horizontal flight.
type Spin struct {
	Omega float64
}

// Ballistics parameterizes projectile flight for entities with Height.
// All terms are algebraic (+ - * / sqrt) to keep cross-arch determinism.
type Ballistics struct {
	Gravity        float64 // downward accel on Z, world units/s²
	DragK          float64 // quadratic drag coefficient (0 = vacuum)
	MagnusK        float64 // spin-curve coupling (0 = no curve)
	BounceDamping  float64 // VZ retention on ground impact [0,1]
	BounceFriction float64 // horizontal velocity retention on impact [0,1]
	SpinDecay      float64 // per-second spin damping
	RollFriction   float64 // horizontal decel while on the ground
	RestVZ         float64 // |VZ| below this on the ground settles to 0

	// Substeps enables opt-in continuous collision for fast projectiles: the
	// horizontal move is split into this many equal sub-moves, each resolved
	// against static colliders, so a projectile faster than its own radius per
	// tick cannot tunnel through thin walls. 0 or 1 keeps the cheap single-step
	// path (the default for slow entities). Requires a CircleCollider and a
	// dynamic Body (InvMass > 0) to be resolved.
	Substeps int
}

// Locomotion moves an agent toward MoveIntent with asymmetric
// acceleration and a speed-dependent turn radius.
type Locomotion struct {
	Accel    float64 // accel toward desired velocity, units/s²
	Decel    float64 // decel when slowing/turning back, units/s²
	MaxSpeed float64
	Agility  float64 // turn responsiveness; effective radius = speed/agility
}

// MoveIntent is the current movement goal of a Locomotion agent.
// Remove the component to stop steering (momentum still decays via
// Locomotion.Decel).
type MoveIntent struct {
	Target       math2d.Vec2
	DesiredSpeed float64
}

