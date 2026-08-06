package scene

import (
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type EntityID uint32

type Transform struct {
	Position math2d.Vec2
	Rotation float64
	Scale    math2d.Vec2
}

// SpritePlane says whether a sprite stands up in the world or lies flat on it.
// It only matters under a tilted camera; see BaseSprite.Plane.
type SpritePlane uint8

const (
	// PlaneUpright draws the sprite facing the camera, like a standing
	// character or a tree.
	PlaneUpright SpritePlane = iota

	// PlaneGround lays the sprite flat on the ground, like a decal, a puddle
	// or a painted marking.
	PlaneGround
)

type BaseSprite struct {
	Layer int
	Color render.Color

	// Plane controls how projection transforms this sprite. PlaneUpright moves and
	// scales the sprite without distorting it. PlaneGround applies the projection
	// to the sprite geometry.
	//
	// DepthScale and View both affect this behavior. A plain 2D camera treats both
	// planes the same.
	//
	// RectSprite upright correction pivots around Transform.Position. Use
	// PlaneGround for ground geometry such as courts, walls, and platforms.
	Plane SpritePlane

	// SortOffset shifts the sprite's base sorting point in world units.
	//
	// Sprites sort by their base rather than their center, so objects planted on
	// the same ground line keep the expected depth order.
	//
	// Use this when the art does not rest on the bottom of its footprint or when a
	// sliced object must sort from a different point.
	SortOffset math2d.Vec2
}

type CircleSprite struct {
	BaseSprite
	Radius float64
}

// RectSprite draws a rectangle in local coordinates, anchored at
// Transform.Position. Rect is translated and scaled around that point.
//
// A centered rectangle places the entity at its center. An asymmetric
// rectangle can place the entity at another anchor, such as the foot of a
// standing object.
//
// Height projection and depth sorting use the anchored rectangle. An unset
// Transform.Scale leaves it unscaled.
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
	BaseSprite
	Texture render.TextureID
	Src     math2d.Rect
	Size    math2d.Vec2

	// ShadowFootprint sets the half-extents used to calculate an ImageSprite's
	// automatic shadow. Use it when transparent padding or an overhang makes
	// Size a poor measure of the sprite's ground contact.
	//
	// The override applies when both axes are positive. The zero value uses half
	// of Size. It is used for airborne shadows and contact shadows enabled by
	// ShadowStyle.
	ShadowFootprint math2d.Vec2
}

// CommandSprite gives an entity an appearance made of raw render commands.
// It participates in painter sorting like other sprites and supports shapes
// such as buildings and mountains.
//
// Commands use world coordinates and are emitted as authored. Layer overrides
// each command's layer so the group sorts and renders as one body. The
// entity's Transform.Position, plus SortOffset, is the sorting point.
//
// CommandSprite is presentation-only and excluded from Checksum. Moving the
// entity does not move its commands. Use Attachment for moving decorations.
type CommandSprite struct {
	Layer      int
	SortOffset math2d.Vec2
	Commands   []render.Command
}

// Attachment anchors an entity's sprite to another entity. It follows the
// parent's interpolated position and height, and uses the parent's depth for
// sorting.
//
// The attached sprite is authored relative to the anchor. RectSprite uses a
// local offset, while CircleSprite is centered on the anchor.
//
// Offset moves the sprite along the ground. HeightOffset moves it along the
// projection's lift axis. The attached entity does not need its own Transform
// or Height, and does not cast a separate shadow.
type Attachment struct {
	Parent EntityID

	// Offset moves the anchor along the ground plane, in world units.
	Offset math2d.Vec2

	// HeightOffset moves the anchor up the projection's lift axis, in the
	// same units as Height.Z. Under a plain 2D camera there is no lift and
	// the offset stays on the ground; use Offset there instead.
	HeightOffset float64
}

// SpriteTransform applies a render-only affine transform to an entity's body
// commands. Use it for squash, stretch, and lean effects.
//
// It is excluded from Checksum and cloned only for presentation continuity.
// The consumer provides it each frame. The transform is applied around the
// body's current center, including its projected height. Compose a translation
// to use a different pivot.
type SpriteTransform struct {
	M math2d.Affine
}

// ShadowStyle controls an entity's automatic ground shadow. Its presence
// enables a contact shadow when Height.Z == 0.
//
// ShadowStyle is presentation-only. It is excluded from Checksum and does not
// affect simulation. The zero value uses the projection defaults.
//
// The sprite footprint defines the shadow geometry. ImageSprite.ShadowFootprint
// can override it. This component only scales, fades, offsets, or disables the
// shadow. It has no effect on PlaneGround sprites because they already lie on
// the ground.
type ShadowStyle struct {
	Scale   float64     // multiplies the shadow radii; 0 means 1 (default)
	Alpha   float64     // multiplies the projection alpha; 0 means 1
	Offset  math2d.Vec2 // ground-space nudge, added to ShadowCenter
	Disable bool        // no automatic shadow for this entity
}

// AmbientLight sets the base color for render-only lighting.
type AmbientLight struct {
	// Color is the base light multiplier. The zero value leaves sprites unchanged.
	Color render.Color
}

// PointLight adds render-only light at the entity's interpolated transform.
type PointLight struct {
	Color render.Color // Color is the light contribution at the source
	// Radius is the finite light range. The contribution is zero outside it.
	Radius float64
	// Falloff sharpens the attenuation. The base curve is 1 - d²/R² (squared
	// distance, so the evaluation stays sqrt-free); each whole step of
	// Falloff multiplies the curve by itself once more. Fractions truncate.
	Falloff float64
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

// HopTween moves an entity between two ground points along an arc. Position
// follows a straight line, while Height reaches Apex at the midpoint and
// returns to zero on landing.
//
// HopTween is simulation state. It contributes to Checksum and replay state.
// The component is removed when the hop completes; its presence indicates an
// active hop.
//
// Use World.Hop to start a hop. Set HopTween directly only when restoring
// explicit state, such as loading a snapshot.
type HopTween struct {
	From, To math2d.Vec2
	Apex     float64
	Duration float64
	Elapsed  float64
}

// HopOptions configures World.Hop.
type HopOptions struct {
	// Duration is the flight time in seconds. Zero lands the hop on the
	// next step.
	Duration float64

	// Apex is the height of the arc's peak, in the same units as Height.Z.
	Apex float64
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
