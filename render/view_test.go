package render

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

// isoView is the 2:1 isometric map: a unit square on the grid comes out as a
// 40 by 20 diamond. Its mean scale is 20, the screen length of one grid unit.
var isoView = math2d.Affine{A: 20, C: -20, B: 10, D: 10}

func viewCamera(view math2d.Affine) Camera {
	frame := math2d.NewRect(math2d.Vec2{X: -300, Y: -150}, math2d.Vec2{X: 600, Y: 300})

	return Camera{World: frame, Viewport: frame, View: view}
}

func TestViewSurvivesTheRoundTripToScreen(t *testing.T) {
	cam := viewCamera(isoView)

	for _, point := range []math2d.Vec2{
		{}, {X: 3.5, Y: 6.25}, {X: -2, Y: 7}, {X: 16, Y: 8},
	} {
		back := cam.ScreenToWorld(cam.WorldToScreen(point))

		if math.Abs(back.X-point.X) > 1e-9 || math.Abs(back.Y-point.Y) > 1e-9 {
			t.Fatalf("round trip moved %v to %v", point, back)
		}
	}
}

// The reason to put the projection on the camera rather than on each sprite is
// that ScreenToWorld can invert it. A pointer landing on a tile has to answer
// with that tile, or hit-testing and recorded input are quietly in a space
// nothing else uses.
func TestViewIsWhatMakesAPointerLandOnATile(t *testing.T) {
	cam := viewCamera(isoView)

	tile := math2d.Vec2{X: 4, Y: 1}
	if got := cam.ScreenToWorld(cam.WorldToScreen(tile)); got.Distance(tile) > 1e-9 {
		t.Fatalf("clicking the centre of tile %v resolved to %v", tile, got)
	}
}

// Height has to go up the screen. Under a view whose axes are both diagonal,
// the world direction that does that is neither of them, and getting it wrong
// makes a rising ball drift along a grid line instead of climbing.
func TestGroundProjectionLiftsAlongTheViewsScreenUp(t *testing.T) {
	projection := GroundProjection(viewCamera(isoView), ProjectionLight{})

	// One unit of height, expressed in the world, then projected.
	lifted := isoView.Apply(projection.LiftAxis.Mul(projection.LiftPerZ))

	if math.Abs(lifted.X) > 1e-9 {
		t.Fatalf("a unit of height drifted sideways by %v", lifted.X)
	}

	// Straight up, by the screen length of one grid unit: the convention that
	// makes a unit cube's three edges come out the same length.
	if math.Abs(lifted.Y+isoView.MeanScale()) > 1e-9 {
		t.Fatalf("a unit of height rose %v, want %v", -lifted.Y, isoView.MeanScale())
	}
}

// A camera with no view keeps the rule it had before views existed: the lift
// is the tangent of the tilt, straight up the screen.
func TestGroundProjectionWithoutAViewKeepsTheTiltRule(t *testing.T) {
	cam := viewCamera(math2d.Affine{})
	cam.DepthScale = 0.5

	projection := GroundProjection(cam, ProjectionLight{})

	if got, want := projection.LiftPerZ, math.Tan(math.Acos(0.5)); math.Abs(got-want) > 1e-9 {
		t.Fatalf("LiftPerZ = %v, want %v", got, want)
	}

	if projection.LiftAxis != (math2d.Vec2{}) {
		t.Fatalf("LiftAxis = %v, want the zero value to keep meaning screen up", projection.LiftAxis)
	}
}

// An upright sprite is one the camera is told not to project. Whatever the
// camera does to shape, the correction has to undo it exactly -- what is left
// over may only be a uniform scale, because a scale is a size and not a
// distortion.
func TestUprightCorrectionUndoesTheCamerasDistortion(t *testing.T) {
	for name, cam := range map[string]Camera{
		"isometric view":     viewCamera(isoView),
		"plain tilt":         tilted(viewCamera(math2d.Affine{}), 0.5),
		"view and tilt both": tilted(viewCamera(isoView), 0.5),
	} {
		t.Run(name, func(t *testing.T) {
			projection := GroundProjection(cam, ProjectionLight{})

			correction, ok := projection.UprightCorrection()
			if !ok {
				t.Fatal("a distorting camera should need a correction")
			}

			// The camera applies the view, then the tilt.
			camera := math2d.Scale(math2d.Vec2{X: 1, Y: projection.GroundScaleY()}).
				Mul(linearPart(cam.View))

			left := camera.Mul(correction)

			if math.Abs(left.B) > 1e-9 || math.Abs(left.C) > 1e-9 {
				t.Fatalf("a shear survived the correction: %v", left)
			}

			if math.Abs(left.A-left.D) > 1e-9 {
				t.Fatalf("the sprite came out anisotropic: %v", left)
			}
		})
	}
}

// A game that asks for no projection at all must get no transform, or a plain
// 2D scene stops being byte for byte what it was.
func TestUprightCorrectionIsAbsentForAPlainCamera(t *testing.T) {
	projection := GroundProjection(viewCamera(math2d.Affine{}), ProjectionLight{})

	if _, ok := projection.UprightCorrection(); ok {
		t.Fatal("an undistorting camera asked for a correction")
	}
}

// The shadow shrinks over a length taken from the camera's frame, which under
// a view is measured in view units while height is measured in world ones.
// Comparing them directly makes the shrink wrong by the view's whole scale --
// here a factor of twenty, which reads as a shadow that never changes.
func TestShadowShrinkIsMeasuredInWorldUnits(t *testing.T) {
	light := ProjectionLight{Softness: 1}

	withView := GroundProjection(viewCamera(isoView), light)

	plain := viewCamera(math2d.Affine{})
	plain.World = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 30, Y: 15})
	plain.DepthScale = 0.5

	// Both frame fifteen world units of depth. The lift differs, so compare
	// the shrink per unit of lift rather than per unit of height.
	got := withView.ShadowShrinkPerZ / withView.LiftPerZ
	want := GroundProjection(plain, light).ShadowShrinkPerZ /
		GroundProjection(plain, light).LiftPerZ

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("shrink per unit of lift = %v, want %v", got, want)
	}
}

// A view that only translates, rotates or uniformly scales does not skew, so
// it has no tilt of its own to supply a lift from: the DepthScale rule keeps
// deciding. Before this was checked, any invertible view hijacked the rule --
// a panned camera tilted at 0.5 lifted by 1, and a rotated top-down camera
// showed height at all.
func TestGroundProjectionKeepsTheTiltRuleForViewsThatDoNotSkew(t *testing.T) {
	want := math.Tan(math.Acos(0.5))

	for name, view := range map[string]math2d.Affine{
		"translation":   math2d.Translate(math2d.Vec2{X: 30, Y: -12}),
		"uniform zoom":  math2d.Scale(math2d.Vec2{X: 3, Y: 3}),
		"pure rotation": math2d.Rotate(math.Pi / 6),
	} {
		t.Run(name, func(t *testing.T) {
			projection := GroundProjection(tilted(viewCamera(view), 0.5), ProjectionLight{})

			if math.Abs(projection.LiftPerZ-want) > 1e-9 {
				t.Fatalf("LiftPerZ = %v, want the DepthScale rule's %v", projection.LiftPerZ, want)
			}
		})
	}

	// The extreme case: a rotated camera looking straight down has no tilt
	// anywhere, so height has nowhere on screen to go.
	projection := GroundProjection(viewCamera(math2d.Rotate(math.Pi/6)), ProjectionLight{})
	if projection.LiftPerZ != 0 {
		t.Fatalf("a rotated top-down camera lifts by %v, want 0", projection.LiftPerZ)
	}
}

func tilted(cam Camera, depthScale float64) Camera {
	cam.DepthScale = depthScale

	return cam
}
