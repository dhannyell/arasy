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
	if overhead.ShadowSkewPerZ != 0 {
		t.Errorf("overhead light leans the shadow by %v, want 0", overhead.ShadowSkewPerZ)
	}

	low := GroundProjection(cam, ProjectionLight{Direction: fromTheLeft, Tilt: 0.4})
	grazing := GroundProjection(cam, ProjectionLight{Direction: fromTheLeft, Tilt: 0.9})

	// The light comes from the left, so the shadow escapes to the right.
	if low.ShadowSkewPerZ <= 0 {
		t.Errorf("shadow leans by %v, want it away from the light", low.ShadowSkewPerZ)
	}
	if grazing.ShadowSkewPerZ <= low.ShadowSkewPerZ {
		t.Errorf("grazing lean %v should exceed low lean %v", grazing.ShadowSkewPerZ, low.ShadowSkewPerZ)
	}
	if grazing.ShadowAlpha >= low.ShadowAlpha {
		t.Errorf("grazing alpha %v should fall below low alpha %v", grazing.ShadowAlpha, low.ShadowAlpha)
	}

	// Reversing the light mirrors the lean and nothing else.
	mirrored := GroundProjection(cam, ProjectionLight{Direction: math2d.Vec2{X: 1}, Tilt: 0.4})
	if math.Abs(mirrored.ShadowSkewPerZ+low.ShadowSkewPerZ) > 1e-12 {
		t.Errorf("mirrored lean = %v, want %v", mirrored.ShadowSkewPerZ, -low.ShadowSkewPerZ)
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
				"ShadowSkewPerZ":   projection.ShadowSkewPerZ,
			} {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					t.Errorf("depth scale %v, light %+v: %s = %v", depthScale, light, name, value)
				}
			}
		}
	}
}
