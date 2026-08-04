package render

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func groundCamera(depthScale float64) Camera {
	return Camera{
		World:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		DepthScale: depthScale,
	}
}

func TestGroundProjectionLiftComesFromTheCameraTilt(t *testing.T) {
	tests := []struct {
		name       string
		depthScale float64
		wantLift   float64
	}{
		// Straight down: height has nowhere on screen to go.
		{"untilted", 1, 0},
		// The classic 2:1 dimetric view, a 60 degree tilt.
		{"dimetric", 0.5, math.Sqrt(3)},
		{"zero value is untilted", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GroundProjection(groundCamera(tt.depthScale), ProjectionLight{}).LiftPerZ

			if math.Abs(got-tt.wantLift) > 1e-12 {
				t.Errorf("LiftPerZ = %v, want %v", got, tt.wantLift)
			}
		})
	}
}

func TestGroundProjectionCarriesTheCameraDepthScale(t *testing.T) {
	cam := groundCamera(0.5)

	if got := GroundProjection(cam, ProjectionLight{}).DepthScale; got != cam.DepthScale {
		t.Errorf("DepthScale = %v, want the camera's %v", got, cam.DepthScale)
	}
}

func TestGroundProjectionShadowFollowsTheLight(t *testing.T) {
	cam := groundCamera(0.5)
	fromTheLeft := math2d.Vec2{X: -1}

	overhead := GroundProjection(cam, ProjectionLight{Direction: fromTheLeft})
	if overhead.ShadowEscapePerZ != 0 {
		t.Errorf("overhead light moves the shadow by %v per unit z, want 0", overhead.ShadowEscapePerZ)
	}

	low := GroundProjection(cam, ProjectionLight{Direction: fromTheLeft, Tilt: 0.4})
	grazing := GroundProjection(cam, ProjectionLight{Direction: fromTheLeft, Tilt: 0.9})

	// The light comes from the left, so the shadow escapes to the right.
	if low.ShadowEscapeAxis.X <= 0 {
		t.Errorf("escape axis = %v, want it pointing away from the light", low.ShadowEscapeAxis)
	}
	if low.ShadowEscapePerZ <= 0 {
		t.Errorf("shadow travels %v per unit z, want it away from the light", low.ShadowEscapePerZ)
	}
	if grazing.ShadowEscapePerZ <= low.ShadowEscapePerZ {
		t.Errorf("grazing travel %v should exceed low travel %v", grazing.ShadowEscapePerZ, low.ShadowEscapePerZ)
	}
	if grazing.ShadowStretch <= low.ShadowStretch {
		t.Errorf("grazing stretch %v should exceed low stretch %v", grazing.ShadowStretch, low.ShadowStretch)
	}
	if grazing.ShadowAlpha >= low.ShadowAlpha {
		t.Errorf("grazing alpha %v should fall below low alpha %v", grazing.ShadowAlpha, low.ShadowAlpha)
	}

	// Reversing the light mirrors where the shadow goes, and nothing else: the
	// mark is the same shape whichever side the light stands on.
	mirrored := GroundProjection(cam, ProjectionLight{Direction: math2d.Vec2{X: 1}, Tilt: 0.4})
	if math.Abs(mirrored.ShadowEscapeAxis.X+low.ShadowEscapeAxis.X) > 1e-12 {
		t.Errorf("mirrored escape axis = %v, want %v", mirrored.ShadowEscapeAxis, low.ShadowEscapeAxis.Mul(-1))
	}
	if math.Abs(mirrored.ShadowEscapePerZ-low.ShadowEscapePerZ) > 1e-12 {
		t.Errorf("mirrored travel = %v, want the same %v", mirrored.ShadowEscapePerZ, low.ShadowEscapePerZ)
	}
}

// The regression the escape model exists to prevent. A shear leaned the shadow
// by trading width for length -- area is preserved, so at any useful height the
// mark degenerated into a scratch. Length and width are independent facts and
// only the length may grow.
func TestGroundProjectionShadowNeverGetsThinner(t *testing.T) {
	projection := GroundProjection(
		groundCamera(0.5),
		ProjectionLight{Direction: math2d.Vec2{X: -1, Y: -1}, Tilt: 0.6},
	)
	footprint := math2d.Vec2{X: 4, Y: 4}
	axis := projection.ShadowEscapeAxis.Normalize()
	across := math2d.Vec2{X: -axis.Y, Y: axis.X}

	for _, z := range []float64{0.5, 4, 12, 40} {
		result := projection.Project(math2d.Vec2{}, footprint, z)

		// Measure the mark across the light by pushing the ellipse's own radius
		// through the transform, which is where a shear would have eaten it.
		edge := across.Mul(result.ShadowRadii.Y)
		centre := result.ShadowCenter
		width := result.ShadowTransform.Apply(centre.Add(edge)).Sub(
			result.ShadowTransform.Apply(centre)).Len()

		if width < result.ShadowRadii.Y-1e-9 {
			t.Errorf("z=%v: half-width across the light = %v, shrank below the radius %v",
				z, width, result.ShadowRadii.Y)
		}
	}
}

func TestGroundProjectionSoftnessDissolvesTheShadowWithHeight(t *testing.T) {
	cam := groundCamera(0.5)

	hard := GroundProjection(cam, ProjectionLight{Softness: 0})
	diffuse := GroundProjection(cam, ProjectionLight{Softness: 1})

	if hard.ShadowShrinkPerZ != 0 || hard.ShadowFadePerZ != 0 {
		t.Errorf("a point source should keep the shadow crisp, got shrink %v fade %v",
			hard.ShadowShrinkPerZ, hard.ShadowFadePerZ)
	}
	if diffuse.ShadowShrinkPerZ <= 0 || diffuse.ShadowFadePerZ <= 0 {
		t.Errorf("a diffuse source should shrink and fade, got shrink %v fade %v",
			diffuse.ShadowShrinkPerZ, diffuse.ShadowFadePerZ)
	}
	if diffuse.ShadowScale.X <= hard.ShadowScale.X {
		t.Errorf("a diffuse source should spread wider: %v vs %v", diffuse.ShadowScale, hard.ShadowScale)
	}
}

// The shadow of an object barely off the ground is drawn under a body that
// covers it. The offset is what keeps the height cue readable from the first
// instant of a jump instead of appearing partway up.
func TestGroundProjectionOffsetsTheShadowClearOfTheBody(t *testing.T) {
	projection := GroundProjection(groundCamera(0.5), ProjectionLight{})

	if projection.ShadowOffset.Y <= 0 {
		t.Errorf("ShadowOffset.Y = %v, want the shadow nudged towards the viewer", projection.ShadowOffset.Y)
	}

	footprint := math2d.Vec2{X: 4, Y: 4}
	justOffTheGround := projection.Project(math2d.Vec2{}, footprint, 0.001)

	if justOffTheGround.ShadowCenter.Y <= 0 {
		t.Errorf("shadow centre at z~0 = %v, want it clear of the body", justOffTheGround.ShadowCenter)
	}
	if justOffTheGround.ShadowAlpha <= 0 || justOffTheGround.ShadowRadii.X <= 0 {
		t.Error("shadow vanishes at the moment of take-off")
	}
}

func TestGroundProjectionStaysFiniteForEveryCamera(t *testing.T) {
	depthScales := []float64{0, 1, 0.5, 1e-12, -1, 2, math.NaN(), math.Inf(1)}
	lights := []ProjectionLight{
		{},
		{Direction: math2d.Vec2{X: 1, Y: 1}, Tilt: 1, Softness: 1},
		{Direction: math2d.Vec2{}, Tilt: math.NaN(), Softness: -1},
	}

	for _, depthScale := range depthScales {
		for _, light := range lights {
			projection := GroundProjection(groundCamera(depthScale), light)

			for name, value := range map[string]float64{
				"LiftPerZ":         projection.LiftPerZ,
				"ScalePerZ":        projection.ScalePerZ,
				"ShadowScale.X":    projection.ShadowScale.X,
				"ShadowScale.Y":    projection.ShadowScale.Y,
				"ShadowOffset.Y":   projection.ShadowOffset.Y,
				"ShadowAlpha":      projection.ShadowAlpha,
				"ShadowShrinkPerZ": projection.ShadowShrinkPerZ,
				"ShadowFadePerZ":   projection.ShadowFadePerZ,
				"ShadowEscapeX":    projection.ShadowEscapeAxis.X,
				"ShadowEscapeY":    projection.ShadowEscapeAxis.Y,
				"ShadowEscapePerZ": projection.ShadowEscapePerZ,
				"ShadowStretch":    projection.ShadowStretch,
			} {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					t.Errorf("depth scale %v, light %+v: %s = %v", depthScale, light, name, value)
				}
			}
		}
	}
}
