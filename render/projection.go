package render

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

// Projection maps scalar height to render-only body and ground-shadow
// presentation. Its zero value is identity with no shadow.
type Projection struct {
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

func (p Projection) IsZero() bool {
	return p.LiftPerZ == 0 &&
		p.ScalePerZ == 0 &&
		p.ShadowScale == (math2d.Vec2{}) &&
		p.ShadowOffset == (math2d.Vec2{}) &&
		p.ShadowAlpha == 0 &&
		p.ShadowShrinkPerZ == 0 &&
		p.ShadowFadePerZ == 0 &&
		p.ShadowSkewPerZ == 0
}
