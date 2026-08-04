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

	// View is the Camera.View this world is drawn under, and like DepthScale
	// it has to be the same affine: it is what tells an upright sprite which
	// shape to take so the camera's skew lands it facing the viewer.
	// GroundProjection fills it from the camera. The zero value is identity.
	View math2d.Affine

	// LiftAxis is the direction, in world units, that height moves a body. The
	// zero value means straight up the screen, which is what it is whenever
	// the world's axes and the screen's agree.
	//
	// Under a Camera.View they do not. On an isometric grid "up on screen" is
	// the diagonal away from the viewer, and a ball rising along the world's
	// -Y would drift sideways as it climbed. GroundProjection derives this
	// from the camera so a game never has to name the direction itself.
	//
	// It is a direction only: how far is LiftPerZ. A zero-length vector falls
	// back to screen up rather than pinning everything to the ground.
	LiftAxis math2d.Vec2

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
	liftAxis := math2d.Vec2{}

	// A skewing view already encodes a tilt of its own -- an isometric grid is
	// a tilted camera written as an affine -- so it supplies the lift instead,
	// and the rule above would read its untilted DepthScale as "no height".
	//
	// Straight up the screen is -Y in view space; the world direction that
	// lands there is what the inverse view says it is. The distance is the
	// view's mean scale, which is the screen length it gives a unit of ground,
	// so a unit cube comes out with three edges the same length -- the
	// definition of an isometric projection.
	if lift, ok := viewLift(cam.View); ok {
		liftAxis, liftPerZ = lift.Normalize(), lift.Len()
	}

	projection := Projection{
		DepthScale: cam.DepthScale,
		View:       cam.View,
		LiftAxis:   liftAxis,
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
	//
	// The span has to be measured in the same units as Z. Camera.World frames
	// view space, which under a skewing view is not world space, so it is
	// divided back by the view's scale before being compared with a height.
	if span := cam.World.Height() / viewScale(cam.View); span > 0 {
		perZ := 2 * liftPerZ * softness / span
		projection.ShadowShrinkPerZ = perZ
		projection.ShadowFadePerZ = perZ
	}

	if direction := light.Direction.Normalize(); tilt > 0 && direction != (math2d.Vec2{}) {
		projection.ShadowSkewPerZ = math.Tan(tilt*math.Pi/2) * -direction.X
	}

	return projection
}

// linearPart drops an affine's translation, leaving the 2x2 that decides
// shape. Directions and shape corrections both want it: translating a
// direction is meaningless, and a correction applied About a position carries
// its own.
func linearPart(m math2d.Affine) math2d.Affine {
	m.E, m.F = 0, 0
	return m
}

// viewScale is the screen length a view gives one unit of ground, and 1 for a
// view that does nothing. It is the bridge between world units and view units
// wherever the two have to be compared.
func viewScale(view math2d.Affine) float64 {
	if view.IsIdentity() {
		return 1
	}

	if scale := view.MeanScale(); scale > 0 {
		return scale
	}

	// A degenerate view has collapsed the world to a line and has no scale to
	// speak of. Refusing to divide by it keeps a bad camera merely wrong
	// rather than infinite.
	return 1
}

// viewLift is how far and in which world direction one unit of height moves a
// body under view, reported as a single vector. It fails for a view that does
// not skew, which is the signal to keep the DepthScale rule instead.
func viewLift(view math2d.Affine) (math2d.Vec2, bool) {
	if view.IsIdentity() {
		return math2d.Vec2{}, false
	}

	inverse, ok := linearPart(view).Inverse()
	if !ok {
		return math2d.Vec2{}, false
	}

	lift := inverse.Apply(math2d.Vec2{Y: -viewScale(view)})
	if lift == (math2d.Vec2{}) {
		return math2d.Vec2{}, false
	}

	return lift, true
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
		BodyPosition: ground.Add(p.liftAxis().Mul(z * p.LiftPerZ)),
		BodyScale:    max(0, 1+z*p.ScalePerZ),
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

// liftAxis resolves LiftAxis into the unit direction Project uses. A zero or
// degenerate vector means straight up the screen, which is both the historical
// behaviour and the right answer whenever the world's axes and the screen's
// agree.
func (p Projection) liftAxis() math2d.Vec2 {
	if axis := p.LiftAxis.Normalize(); axis != (math2d.Vec2{}) {
		return axis
	}

	return math2d.Vec2{Y: -1}
}

// GroundScaleY is how much of its height a shape lying on the ground keeps on
// screen: 1 under an untilted camera, and DepthScale once it is clamped into
// range otherwise.
//
// It describes the tilt only. A skewing View distorts the ground in ways no
// single number can carry, which is what UprightCorrection exists for.
func (p Projection) GroundScaleY() float64 {
	return clampDepthScale(p.DepthScale)
}

// UprightCorrection is the shape a sprite has to be given so that the camera's
// own distortion lands it facing the viewer at its authored size. It is what
// PlaneUpright means, worked out once for the whole frame. The second result
// is false when the camera distorts nothing and the sprite is already right,
// which is the signal to emit no transform at all rather than a no-op one --
// how a plain 2D scene stays byte for byte what it was.
//
// The camera applies View and then the tilt, so undoing it applies neither
// alone but the inverse of the pair. The mean scale that comes back afterwards
// is what keeps an upright sprite's size quoted in world units like every
// other measurement a game writes: without it a billboard on an isometric grid
// would be authored in pixels while the ground it stands on was authored in
// tiles.
func (p Projection) UprightCorrection() (math2d.Affine, bool) {
	correction := math2d.Scale(math2d.Vec2{X: 1, Y: 1 / p.GroundScaleY()})

	if !p.View.IsIdentity() {
		inverse, ok := linearPart(p.View).Inverse()
		if !ok {
			// A degenerate view leaves nothing to stand back up from.
			return math2d.Affine{}, false
		}

		correction = inverse.Mul(correction).Mul(math2d.Scale(
			math2d.Vec2{X: viewScale(p.View), Y: viewScale(p.View)},
		))
	}

	if correction.IsIdentity() {
		return math2d.Affine{}, false
	}

	return correction, true
}

func (p Projection) IsZero() bool {
	return p.DepthScale == 0 &&
		p.View.IsIdentity() &&
		p.LiftAxis == (math2d.Vec2{}) &&
		p.LiftPerZ == 0 &&
		p.ScalePerZ == 0 &&
		p.ShadowScale == (math2d.Vec2{}) &&
		p.ShadowOffset == (math2d.Vec2{}) &&
		p.ShadowAlpha == 0 &&
		p.ShadowShrinkPerZ == 0 &&
		p.ShadowFadePerZ == 0 &&
		p.ShadowSkewPerZ == 0
}
