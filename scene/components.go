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

	// Plane selects how the camera's projection treats this sprite. The
	// default, PlaneUpright, draws it facing the camera: the projection moves
	// it but never distorts it. PlaneGround lets the projection have it,
	// because it lies in the plane being projected.
	//
	// Both DepthScale and View count as projection here. Under a tilt the
	// difference is a squash; under an isometric View it is the difference
	// between a net standing across the court and one lying on it like a rug.
	//
	// With a plain 2D camera the two are identical and this field costs
	// nothing to ignore.
	Plane SpritePlane

	// SortOffset moves this sprite's sorting point away from its base, in
	// world units.
	//
	// A sprite sorts by its base, not its centre, because the centre gets the
	// common case wrong: a tall tree and a short bush planted on the same line
	// have centres at different depths while their bases sit together, so
	// sorting by centre puts the tree behind the bush standing next to it.
	//
	// The zero value is that base. Reach for this field when a sprite's art
	// does not stand on the bottom edge of its own footprint, or when a piece
	// of a sliced-up object needs to claim the depth of the slice rather than
	// of the whole.
	SortOffset math2d.Vec2
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

	// Plane behaves as BaseSprite.Plane. ImageSprite carries its own copy
	// because it does not embed BaseSprite: it has no Color to share.
	Plane SpritePlane

	// SortOffset behaves as BaseSprite.SortOffset, carried here for the same
	// reason as Plane.
	SortOffset math2d.Vec2

	// ShadowFootprint define the size o shadow when sprite is z >= 0.
	ShadowFootprint math2d.Vec2
}

// CommandSprite gives an entity's appearance as raw render commands while
// letting it participate in the painter sort like any other sprite. It exists
// for the shape none of the built-in sprites can draw -- an extruded building,
// a faceted mountain -- that still has to order correctly against the units
// walking past it. Before it, such props had to be sliced onto hand-managed
// layers.
//
// The commands are authored in world coordinates, like RectSprite's geometry,
// and are emitted exactly as written except for the layer: Layer here
// overrides each command's own, so the whole group sorts and draws as one
// body. The entity's Transform.Position is the sorting point (plus
// SortOffset), which for a standing prop should be where it meets the ground.
//
// Content is presentation-only: it never enters Checksum, and a moving entity
// does not move its commands -- attach static props to static entities, and
// give moving decoration an Attachment instead.
type CommandSprite struct {
	Layer      int
	SortOffset math2d.Vec2
	Commands   []render.Command
}

// Attachment anchors this entity's sprite to another entity: it draws at the
// parent's position, rides the parent's height, and sorts at the parent's
// depth. It is how a composite object -- a tree with canopies above its trunk,
// a machine with a highlight riding on its body -- stays one thing in the
// painter order while being several sprites.
//
// The attached sprite's geometry is authored relative to the anchor point: a
// RectSprite's Rect is an offset around it rather than absolute world
// coordinates, and a CircleSprite draws centred on it. The anchor is the
// parent's (interpolated) position, moved Offset along the ground and
// HeightOffset up the projection's lift axis on top of the parent's own
// height. An attached entity needs no Transform or Height of its own and
// casts no shadow -- the parent's covers the composite.
type Attachment struct {
	Parent EntityID

	// Offset moves the anchor along the ground plane, in world units.
	Offset math2d.Vec2

	// HeightOffset moves the anchor up the projection's lift axis, in the
	// same units as Height.Z. Under a plain 2D camera there is no lift and
	// the offset stays on the ground; use Offset there instead.
	HeightOffset float64
}

// SpriteTransform is a render-only per-entity affine applied to the
// entity's body draw commands (squash & stretch, lean). It is decorative:
// excluded from Checksum, cloned for presentation continuity only, and
// authored by the consumer each frame. The matrix is applied About the
// body's (possibly lifted) center, so non-uniform scale deforms from the
// center; compose Translate for a different pivot.
type SpriteTransform struct {
	M math2d.Affine
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

// HopTween carries an entity from one ground point to another in an arc:
// Position runs the straight line while Height rises to Apex halfway and
// lands at zero. It is the tactics-game hop -- the third dimension
// TransformTween does not animate -- and it removes itself on landing, so
// "is this entity mid-hop" is simply whether the component exists.
//
// It is simulation state like the tweens: it enters Checksum and replays.
// Prefer World.Hop; set this directly only to restore explicit state, such
// as loading snapshots.
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
