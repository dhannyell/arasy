package render

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestProjectionIsZero(t *testing.T) {
	if !(Projection{}).IsZero() {
		t.Error("zero Projection should be IsZero")
	}

	p := Projection{LiftPerZ: 1}
	if p.IsZero() {
		t.Error("Projection with LiftPerZ=1 should not be IsZero")
	}

	p = Projection{ScalePerZ: 1}
	if p.IsZero() {
		t.Error("Projection with ScalePerZ=1 should not be IsZero")
	}

	p = Projection{ShadowScale: math2d.Vec2{X: 1}}
	if p.IsZero() {
		t.Error("Projection with ShadowScale.X=1 should not be IsZero")
	}

	p = Projection{ShadowOffset: math2d.Vec2{Y: 1}}
	if p.IsZero() {
		t.Error("Projection with ShadowOffset.Y=1 should not be IsZero")
	}

	p = Projection{ShadowAlpha: 0.5}
	if p.IsZero() {
		t.Error("Projection with ShadowAlpha=0.5 should not be IsZero")
	}

	p = Projection{ShadowShrinkPerZ: 1}
	if p.IsZero() {
		t.Error("Projection with ShadowShrinkPerZ=1 should not be IsZero")
	}

	p = Projection{ShadowFadePerZ: 1}
	if p.IsZero() {
		t.Error("Projection with ShadowFadePerZ=1 should not be IsZero")
	}
}

func TestProjectionZeroHeightIdentity(t *testing.T) {
	p := Projection{
		LiftPerZ:    8,
		ScalePerZ:   2,
		ShadowScale: math2d.Vec2{X: 1.5, Y: 0.8},
		ShadowAlpha: 0.7,
	}

	ground := math2d.Vec2{X: 10, Y: 20}
	footprint := math2d.Vec2{X: 3, Y: 4}

	// z == 0 is the contact shadow: body stays put at its authored size and
	// the shadow is the footprint at full alpha.
	r := p.Project(ground, footprint, 0)
	if r.BodyPosition != ground {
		t.Errorf("z=0: BodyPosition = %+v, want %+v", r.BodyPosition, ground)
	}
	if r.BodyScale != 1 {
		t.Errorf("z=0: BodyScale = %v, want 1", r.BodyScale)
	}
	if r.ShadowAlpha != p.ShadowAlpha {
		t.Errorf("z=0: ShadowAlpha = %v, want %v", r.ShadowAlpha, p.ShadowAlpha)
	}
	want := math2d.Vec2{X: footprint.X * p.ShadowScale.X, Y: footprint.Y * p.ShadowScale.Y}
	if r.ShadowRadii != want {
		t.Errorf("z=0: ShadowRadii = %+v, want %+v", r.ShadowRadii, want)
	}

	// Below ground stays outside the projection's domain.
	rNeg := p.Project(ground, footprint, -5)
	if rNeg != (ProjectedHeight{}) {
		t.Errorf("z<0: got %+v, want zero ProjectedHeight", rNeg)
	}
}

func TestProjectionBodyLift(t *testing.T) {
	p := Projection{LiftPerZ: 10}
	ground := math2d.Vec2{X: 50, Y: 100}
	fp := math2d.Vec2{X: 2, Y: 2}

	r1 := p.Project(ground, fp, 2)
	r2 := p.Project(ground, fp, 5)

	if r1.BodyPosition.X != ground.X {
		t.Errorf("BodyPosition.X = %v, want %v", r1.BodyPosition.X, ground.X)
	}
	if r1.BodyPosition.Y != ground.Y-20 {
		t.Errorf("z=2: BodyPosition.Y = %v, want %v", r1.BodyPosition.Y, ground.Y-20)
	}
	if r2.BodyPosition.Y != ground.Y-50 {
		t.Errorf("z=5: BodyPosition.Y = %v, want %v", r2.BodyPosition.Y, ground.Y-50)
	}
	// Monotonicity: higher Z = higher lift (lower Y on screen)
	if r2.BodyPosition.Y >= r1.BodyPosition.Y {
		t.Errorf("lift should be monotonic: z=5 Y=%v < z=2 Y=%v", r2.BodyPosition.Y, r1.BodyPosition.Y)
	}
}

func TestProjectionBodyScale(t *testing.T) {
	p := Projection{ScalePerZ: 0.5}
	ground := math2d.Vec2{X: 0, Y: 0}
	fp := math2d.Vec2{X: 1, Y: 1}

	r0 := p.Project(ground, fp, 0)
	r1 := p.Project(ground, fp, 2)
	r2 := p.Project(ground, fp, 4)

	if r0.BodyScale != 1 {
		t.Errorf("z=0: BodyScale = %v, want 1", r0.BodyScale)
	}
	if r1.BodyScale != 2 {
		t.Errorf("z=2: BodyScale = %v, want 2", r1.BodyScale)
	}
	if r2.BodyScale != 3 {
		t.Errorf("z=4: BodyScale = %v, want 3", r2.BodyScale)
	}
	if r2.BodyScale <= r1.BodyScale {
		t.Error("BodyScale should increase with Z")
	}

	// Negative ScalePerZ: clamped to 0
	pNeg := Projection{ScalePerZ: -0.5}
	rNeg := pNeg.Project(ground, fp, 10)
	if rNeg.BodyScale != 0 {
		t.Errorf("negative ScalePerZ: BodyScale = %v, want 0", rNeg.BodyScale)
	}
}

func TestProjectionShadowAlpha(t *testing.T) {
	p := Projection{
		ShadowAlpha:    0.9,
		ShadowFadePerZ: 0.2,
		ShadowScale:    math2d.Vec2{X: 1, Y: 1},
	}
	ground := math2d.Vec2{X: 0, Y: 0}
	fp := math2d.Vec2{X: 2, Y: 2}

	r0 := p.Project(ground, fp, 0)
	if r0.ShadowAlpha != 0.9 {
		t.Errorf("z=0: ShadowAlpha = %v, want full 0.9 at contact", r0.ShadowAlpha)
	}

	r1 := p.Project(ground, fp, 5)
	want := 0.9 / (1 + 0.2*5)
	if !floatNear(r1.ShadowAlpha, want, 1e-9) {
		t.Errorf("z=5: ShadowAlpha = %v, want %v", r1.ShadowAlpha, want)
	}

	rFar := p.Project(ground, fp, 100)
	if rFar.ShadowAlpha <= 0 || rFar.ShadowAlpha >= 0.5 {
		t.Errorf("z=100: ShadowAlpha = %v, should be small but > 0", rFar.ShadowAlpha)
	}

	// Clamp > 1
	pClamp := Projection{
		ShadowAlpha:    5,
		ShadowFadePerZ: 0,
		ShadowScale:    math2d.Vec2{X: 1, Y: 1},
	}
	rClamp := pClamp.Project(ground, fp, 1)
	if rClamp.ShadowAlpha != 1 {
		t.Errorf("ShadowAlpha should be clamped to 1, got %v", rClamp.ShadowAlpha)
	}
}

func TestProjectionShadowShrink(t *testing.T) {
	p := Projection{
		ShadowScale:      math2d.Vec2{X: 1.5, Y: 0.8},
		ShadowShrinkPerZ: 0.3,
	}
	fp := math2d.Vec2{X: 3, Y: 4}
	ground := math2d.Vec2{X: 0, Y: 0}

	r0 := p.Project(ground, fp, 0)
	// Contact: no shrink, footprint times ShadowScale.
	if !floatNear(r0.ShadowRadii.X, 4.5, 1e-9) || !floatNear(r0.ShadowRadii.Y, 3.2, 1e-9) {
		t.Errorf("z=0: ShadowRadii = %v, want (4.5, 3.2)", r0.ShadowRadii)
	}

	r1 := p.Project(ground, fp, 2)
	// shrink = 1 / (1 + 0.3*2) = 1/1.6 = 0.625
	// ShadowRadii.X = 3 * 1.5 * 0.625 = 2.8125
	// ShadowRadii.Y = 4 * 0.8 * 0.625 = 2.0
	if !floatNear(r1.ShadowRadii.X, 2.8125, 1e-9) {
		t.Errorf("z=2: ShadowRadii.X = %v, want 2.8125", r1.ShadowRadii.X)
	}
	if !floatNear(r1.ShadowRadii.Y, 2.0, 1e-9) {
		t.Errorf("z=2: ShadowRadii.Y = %v, want 2.0", r1.ShadowRadii.Y)
	}

	// Higher Z = smaller shadow
	r2 := p.Project(ground, fp, 5)
	if r2.ShadowRadii.X >= r1.ShadowRadii.X || r2.ShadowRadii.Y >= r1.ShadowRadii.Y {
		t.Error("shadow should shrink as Z increases")
	}
}

func TestProjectionShadowOffset(t *testing.T) {
	p := Projection{
		ShadowScale:  math2d.Vec2{X: 1, Y: 1},
		ShadowOffset: math2d.Vec2{X: 2, Y: -1},
		ShadowAlpha:  0.5,
	}
	fp := math2d.Vec2{X: 3, Y: 4}
	ground := math2d.Vec2{X: 10, Y: 20}

	r := p.Project(ground, fp, 1)
	// ShadowCenter = footprintHalfExtents * ShadowOffset
	if r.ShadowCenter.X != 6 {
		t.Errorf("ShadowCenter.X = %v, want 6", r.ShadowCenter.X)
	}
	if r.ShadowCenter.Y != -4 {
		t.Errorf("ShadowCenter.Y = %v, want -4", r.ShadowCenter.Y)
	}
}

func TestProjectionDifferentFootprints(t *testing.T) {
	p := Projection{
		ShadowScale: math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha: 0.5,
	}
	ground := math2d.Vec2{X: 0, Y: 0}

	rWide := p.Project(ground, math2d.Vec2{X: 8, Y: 2}, 2)
	if rWide.ShadowRadii.X <= rWide.ShadowRadii.Y {
		t.Errorf("wide footprint: ShadowRadii.X should be > Y, got %v", rWide.ShadowRadii)
	}

	rTall := p.Project(ground, math2d.Vec2{X: 2, Y: 8}, 2)
	if rTall.ShadowRadii.Y <= rTall.ShadowRadii.X {
		t.Errorf("tall footprint: ShadowRadii.Y should be > X, got %v", rTall.ShadowRadii)
	}

	rZero := p.Project(ground, math2d.Vec2{}, 2)
	if rZero.ShadowRadii.X != 0 || rZero.ShadowRadii.Y != 0 {
		t.Errorf("zero footprint: ShadowRadii = %v, want (0,0)", rZero.ShadowRadii)
	}
}

func TestProjectionZeroIsIdentity(t *testing.T) {
	p := Projection{}
	ground := math2d.Vec2{X: 10, Y: 20}
	fp := math2d.Vec2{X: 5, Y: 5}

	r := p.Project(ground, fp, 10)
	if r.BodyPosition != ground {
		t.Errorf("BodyPosition = %v, want %v", r.BodyPosition, ground)
	}
	if r.BodyScale != 1 {
		t.Errorf("BodyScale = %v, want 1", r.BodyScale)
	}
	if r.ShadowAlpha != 0 {
		t.Errorf("ShadowAlpha = %v, want 0", r.ShadowAlpha)
	}
	if r.ShadowRadii.X != 0 || r.ShadowRadii.Y != 0 {
		t.Errorf("ShadowRadii = %v, want (0,0)", r.ShadowRadii)
	}
}

func floatNear(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

func TestProjectionRoundShadowNeedsNoTransform(t *testing.T) {
	p := Projection{
		LiftPerZ:    1,
		ShadowScale: math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha: 0.5,
	}
	ground := math2d.Vec2{X: 10, Y: 20}
	fp := math2d.Vec2{X: 3, Y: 4}

	for _, z := range []float64{0.5, 2, 10} {
		if r := p.Project(ground, fp, z); !r.ShadowTransform.IsIdentity() {
			t.Errorf("z=%v: ShadowTransform not identity with ShadowStretch=0", z)
		}
	}
}

// The stretch happens about the shadow's own centre, not about the body's
// contact point. Anchoring it anywhere else would slide the mark sideways as it
// lengthens, so a body climbing straight up would cast a shadow that drifts.
func TestProjectionShadowStretchesAboutItsOwnCentre(t *testing.T) {
	p := Projection{
		ShadowScale:      math2d.Vec2{X: 1, Y: 1},
		ShadowOffset:     math2d.Vec2{X: 0.5, Y: 0.25},
		ShadowAlpha:      0.5,
		ShadowEscapeAxis: math2d.Vec2{X: 1},
		ShadowEscapePerZ: 0.3,
		ShadowStretch:    2,
	}
	ground := math2d.Vec2{X: 10, Y: 20}
	fp := math2d.Vec2{X: 3, Y: 4}

	r := p.Project(ground, fp, 5)
	centre := ground.Add(r.ShadowCenter)
	if got := r.ShadowTransform.Apply(centre); !floatNear(got.X, centre.X, 1e-12) ||
		!floatNear(got.Y, centre.Y, 1e-12) {
		t.Errorf("Apply(centre) = %v, want fixed point %v", got, centre)
	}
}

// The height cue: the shadow separates from the body at a constant rate, while
// its shape stays put. The old shear had these backwards -- the mark never
// moved and only ever deformed.
func TestProjectionShadowTravelsWithZButKeepsItsShape(t *testing.T) {
	p := Projection{
		ShadowScale:      math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha:      0.5,
		ShadowEscapeAxis: math2d.Vec2{X: 1},
		ShadowEscapePerZ: 0.3,
		ShadowStretch:    2,
	}
	ground := math2d.Vec2{X: 10, Y: 20}
	fp := math2d.Vec2{X: 3, Y: 4}

	r2 := p.Project(ground, fp, 2)
	r4 := p.Project(ground, fp, 4)

	if !floatNear(r2.ShadowCenter.X, 0.3*2, 1e-12) {
		t.Errorf("travel at z=2 = %v, want ShadowEscapePerZ*z = 0.6", r2.ShadowCenter.X)
	}
	if !floatNear(r4.ShadowCenter.X, 2*r2.ShadowCenter.X, 1e-12) {
		t.Errorf("travel at z=4 (%v) != 2x travel at z=2 (%v): it must grow linearly with z",
			r4.ShadowCenter.X, r2.ShadowCenter.X)
	}

	// A directional light casts the same shape at every height. Measured as the
	// span the transform gives the ellipse along the escape axis.
	span := func(r ProjectedHeight) float64 {
		centre := ground.Add(r.ShadowCenter)
		edge := centre.Add(math2d.Vec2{X: r.ShadowRadii.X})
		return r.ShadowTransform.Apply(edge).Sub(r.ShadowTransform.Apply(centre)).Len()
	}
	if !floatNear(span(r2), span(r4), 1e-12) {
		t.Errorf("shape changed with height: %v at z=2, %v at z=4", span(r2), span(r4))
	}
}

func TestProjectionShadowZeroHeightIsIdentity(t *testing.T) {
	p := Projection{
		ShadowScale:      math2d.Vec2{X: 1, Y: 1},
		ShadowAlpha:      0.5,
		ShadowEscapeAxis: math2d.Vec2{X: 1},
		ShadowEscapePerZ: 0.3,
		ShadowStretch:    2,
	}
	ground := math2d.Vec2{X: 10, Y: 20}
	fp := math2d.Vec2{X: 3, Y: 4}

	// A directional light casts the same shape at every height, contact
	// included: the stretch applies at z == 0 too.
	if r := p.Project(ground, fp, 0); r.ShadowTransform.IsIdentity() {
		t.Error("z=0: contact shadow lost the directional stretch")
	}

	if r := p.Project(ground, fp, -1); !r.ShadowTransform.IsIdentity() {
		t.Error("z<0: ShadowTransform not identity below ground")
	}
}

func TestProjectionIsZeroShadowEscape(t *testing.T) {
	if (Projection{ShadowEscapePerZ: 1}).IsZero() {
		t.Error("Projection with ShadowEscapePerZ=1 should not be IsZero")
	}
	if (Projection{ShadowStretch: 1}).IsZero() {
		t.Error("Projection with ShadowStretch=1 should not be IsZero")
	}
}
