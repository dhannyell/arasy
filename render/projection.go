package render

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

// Projection is how a world is presented in 2.5D: the camera tilt its sprites
// are drawn under, and how scalar height turns into a lifted body and a ground
// shadow. All of it is render-only. Its zero value is a plain top-down 2D
// view: no tilt, no lift, no shadow.
type Projection struct {
	// DepthScale is the Camera.DepthScale this world is drawn under, and it
	// has to be the same number: it is what tells an upright sprite how much
	// foreshortening to stand back up against. GroundProjection fills it from
	// the camera, which is the way to set it without the two drifting apart.
	//
	// Zero means an untilted camera, exactly as it does on Camera.
	DepthScale float64

	LiftPerZ         float64
	ScalePerZ        float64
	ShadowScale      math2d.Vec2 // relative to footprint half-extents
	ShadowOffset     math2d.Vec2 // relative to footprint half-extents
	ShadowAlpha      float64
	ShadowShrinkPerZ float64
	ShadowFadePerZ   float64
	// ShadowSkewPerZ leans the ground shadow horizontally by this amount per
	// unit of height, so it "escapes" opposite the light and grows with z.
	// Zero keeps the shadow axis-aligned. Render-only.
	ShadowSkewPerZ float64
}

type ProjectedHeight struct {
	BodyPosition math2d.Vec2
	BodyScale    float64
	ShadowCenter math2d.Vec2
	ShadowRadii  math2d.Vec2
	ShadowAlpha  float64
	// ShadowTransform is the affine applied to the shadow ellipse (directional
	// skew). Zero value = identity = axis-aligned shadow.
	ShadowTransform math2d.Affine
}

// ProjectionLight describes the scene's lighting in the scene's own terms, not the
// shadow's. The zero value is overhead light: the shadow sits centred under
// the object with no lean.
type ProjectionLight struct {
	// Direction is the horizontal direction the light comes from; the shadow
	// stretches the opposite way. It need not be normalized -- the length is
	// ignored. Irrelevant when Tilt is zero.
	Direction math2d.Vec2

	// Tilt is how far the light leans off vertical: 0 is overhead, 1 grazes
	// the horizon. Values above the internal cap are clamped, because a light
	// exactly at the horizon casts an infinite shadow.
	Tilt float64

	// Softness runs from 0 (a point source: a hard, well-defined shadow) to 1
	// (a diffuse source: a wide shadow that dissolves as the object rises).
	Softness float64
}

const (
	// maxLightTilt keeps the light short of the horizon, where the shadow's
	// lean would go to infinity.
	maxLightTilt = 0.95

	// minShadowOffset nudges the shadow towards the viewer by this fraction of
	// the sprite's footprint. Without it the shadow of an object barely off the
	// ground hides entirely behind the body drawn over it, and the height cue
	// only appears once the object is already high.
	minShadowOffset = 0.35

	// baseShadowAlpha is the opacity of a shadow cast by an overhead light.
	baseShadowAlpha = 0.35
)

// GroundProjection derives the height presentation from the camera's tilt and
// the scene's light. It is the API to reach for:
//
//	world.SetProjection(render.GroundProjection(cam, render.ProjectionLight{
//		Direction: math2d.Vec2{X: -1, Y: -1},
//		Tilt:      0.4,
//	}))
//
// The Projection fields stay public, so a game with its own art direction can
// override one of them without giving up the derivation for the rest.
func GroundProjection(cam Camera, light ProjectionLight) Projection {
	depthScale := clampDepthScale(cam.DepthScale)
	// A NaN light is a caller's arithmetic accident, not a request. Clamping
	// alone would carry it through, so it falls back to the neutral value.
	softness := clampOrZero(light.Softness, 1)
	tilt := clampOrZero(light.Tilt, maxLightTilt)

	// One unit of height is worth tan(theta) units of depth on screen, where
	// theta is the camera's tilt from vertical and DepthScale is its cosine.
	//
	// An untilted camera therefore lifts by 0, which reads as a bug until you
	// picture it: looking straight down, height has nowhere on screen to go.
	// Pretending otherwise is the confusion between depth and elevation this
	// derivation exists to end. A game that wants height on a top-down camera
	// wants a tilted camera.
	liftPerZ := math.Tan(math.Acos(depthScale))

	projection := Projection{
		DepthScale: cam.DepthScale,
		LiftPerZ:   liftPerZ,

		// ScalePerZ stays 0: an orthographic camera does not make a rising
		// object bigger. Set it yourself for the cartoon reading where it does.

		// The shadow lies on the ground, and the camera already foreshortens
		// the ground. Flattening it here too would apply the same tilt twice.
		// A diffuse source spreads it wider instead.
		ShadowScale:  math2d.Vec2{X: 1 + 0.5*softness, Y: 1 + 0.5*softness},
		ShadowOffset: math2d.Vec2{Y: minShadowOffset},

		// A grazing light spreads the same shadow over more ground, so it
		// reads fainter as well as longer.
		ShadowAlpha: baseShadowAlpha * (1 - 0.4*tilt),
	}

	// Shrinking and fading with height is what says "this is off the ground".
	// Both are per unit of Z, so they need a length to be measured against:
	// the visible world, scaled so that an object risen half of it has half
	// its shadow left under a fully diffuse light. A point source keeps its
	// shadow crisp all the way up, which is the other half of Softness.
	if span := cam.World.Height(); span > 0 {
		perZ := 2 * liftPerZ * softness / span
		projection.ShadowShrinkPerZ = perZ
		projection.ShadowFadePerZ = perZ
	}

	if direction := light.Direction.Normalize(); tilt > 0 && direction != (math2d.Vec2{}) {
		projection.ShadowSkewPerZ = math.Tan(tilt*math.Pi/2) * -direction.X
	}

	return projection
}

func clampOrZero(value, max float64) float64 {
	if math.IsNaN(value) {
		return 0
	}

	return math2d.Clamp(value, 0, max)
}

func (p Projection) Project(ground math2d.Vec2, footprintHalfExtents math2d.Vec2, z float64) ProjectedHeight {
	if z <= 0 {
		return ProjectedHeight{}
	}

	shadowTransform := math2d.Affine{}
	if p.ShadowSkewPerZ != 0 {
		shadowCenter := math2d.Vec2{
			X: ground.X + footprintHalfExtents.X*p.ShadowOffset.X,
			Y: ground.Y + footprintHalfExtents.Y*p.ShadowOffset.Y,
		}
		shadowTransform = math2d.Shear(p.ShadowSkewPerZ*z, 0).About(shadowCenter)
	}
	shrink := 1 / (1 + max(0, p.ShadowShrinkPerZ)*z)
	return ProjectedHeight{
		BodyPosition: math2d.Vec2{
			X: ground.X,
			Y: ground.Y - z*p.LiftPerZ,
		},
		BodyScale: max(0, 1+z*p.ScalePerZ),
		ShadowCenter: math2d.Vec2{
			X: footprintHalfExtents.X * p.ShadowOffset.X,
			Y: footprintHalfExtents.Y * p.ShadowOffset.Y,
		},
		ShadowRadii: math2d.Vec2{
			X: math.Abs(footprintHalfExtents.X) * max(0, p.ShadowScale.X) * shrink,
			Y: math.Abs(footprintHalfExtents.Y) * max(0, p.ShadowScale.Y) * shrink,
		},
		ShadowAlpha:     math2d.Clamp(p.ShadowAlpha/(1+max(0, p.ShadowFadePerZ)*z), 0, 1),
		ShadowTransform: shadowTransform,
	}
}

// GroundScaleY is how much of its height a shape lying on the ground keeps on
// screen: 1 under an untilted camera, and DepthScale once it is clamped into
// range otherwise. A shape standing up against the tilt wants its reciprocal.
//
// The value 1 is the signal to emit no transform at all rather than a no-op
// one, which is how a plain 2D scene stays byte for byte what it was.
func (p Projection) GroundScaleY() float64 {
	return clampDepthScale(p.DepthScale)
}

func (p Projection) IsZero() bool {
	return p.DepthScale == 0 &&
		p.LiftPerZ == 0 &&
		p.ScalePerZ == 0 &&
		p.ShadowScale == (math2d.Vec2{}) &&
		p.ShadowOffset == (math2d.Vec2{}) &&
		p.ShadowAlpha == 0 &&
		p.ShadowShrinkPerZ == 0 &&
		p.ShadowFadePerZ == 0 &&
		p.ShadowSkewPerZ == 0
}
