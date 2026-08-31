package fixedprobe

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/fixed"
)

// sceneFromFixture builds a real scene.World from the same exact float
// values the float mirror reads, so both can be compared bit for bit.
func sceneFromFixture(f *fixture) (*scene.World, []scene.EntityID) {
	w := scene.NewWorld()
	ids := make([]scene.EntityID, f.n)

	for i := range f.n {
		id := w.Spawn()
		ids[i] = id
		w.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: qf(f.posX[i]), Y: qf(f.posY[i])},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetVelocity(id, scene.Velocity{
			Linear: math2d.Vec2{X: qf(f.velX[i]), Y: qf(f.velY[i])},
		})
		if f.isRect[i] {
			w.SetRectCollider(id, scene.RectCollider{
				HalfExtends: math2d.Vec2{X: qf(f.extX[i]), Y: qf(f.extY[i])},
			})
		} else {
			w.SetCircleCollider(id, scene.CircleCollider{Radius: qf(f.extX[i])})
		}
		w.SetBody(id, scene.Body{
			InvMass:     qf(f.invMass[i]),
			Restitution: qf(f.restitution[i]),
		})
		if f.z != nil {
			w.SetHeight(id, scene.Height{Z: qf(f.z[i]), VZ: qf(f.vz[i])})
			w.SetSpin(id, scene.Spin{Omega: qf(f.omega[i])})
			w.SetBallistics(id, scene.Ballistics{
				Gravity:        qf(qGravity),
				DragK:          qf(qDragK),
				MagnusK:        qf(qMagnusK),
				BounceDamping:  qf(qBounceDamping),
				BounceFriction: qf(qBounceFriction),
				SpinDecay:      qf(qSpinDecay),
				RollFriction:   qf(qRollFriction),
				RestVZ:         qf(qRestVZ),
			})
		}
	}

	return w, ids
}

// TestFloatMirrorMatchesScene proves the probe measures the real work: the
// float mirror must reproduce scene.World.Step bit for bit on the mirrored
// subset. A drift here means the probe is a straw man.
func TestFloatMirrorMatchesScene(t *testing.T) {
	const ticks = 120
	const dt = 1.0 / 60.0

	scenarios := []struct {
		name string
		f    *fixture
	}{
		{name: "mixed-contacts", f: spreadFixture(300, 8, 2)},
		{name: "ballistic", f: ballisticFixture(200)},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			world, ids := sceneFromFixture(sc.f)
			mirror := newFloatWorld(sc.f)

			cfg := scene.StepConfig{
				Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldSide, Y: worldSide}),
			}
			for range ticks {
				world.Step(dt, cfg)
				mirror.step(dt, stepCellSize)
			}

			for i, id := range ids {
				transform, _ := world.Transform(id)
				velocity, _ := world.Velocity(id)
				requireSameBits(t, "posX", i, transform.Position.X, mirror.posX[i])
				requireSameBits(t, "posY", i, transform.Position.Y, mirror.posY[i])
				requireSameBits(t, "velX", i, velocity.Linear.X, mirror.velX[i])
				requireSameBits(t, "velY", i, velocity.Linear.Y, mirror.velY[i])

				if sc.f.z != nil {
					height, _ := world.Height(id)
					spin, _ := world.Spin(id)
					requireSameBits(t, "z", i, height.Z, mirror.z[i])
					requireSameBits(t, "vz", i, height.VZ, mirror.vz[i])
					requireSameBits(t, "omega", i, spin.Omega, mirror.omega[i])
				}
			}
		})
	}
}

func requireSameBits(t *testing.T, field string, i int, want, got float64) {
	t.Helper()
	if math.Float64bits(want) != math.Float64bits(got) {
		t.Fatalf("%s[%d]: scene %v (%#x) != mirror %v (%#x)",
			field, i, want, math.Float64bits(want), got, math.Float64bits(got))
	}
}

// Parity budget for the fixed mirror against the float mirror. The budget is
// a design decision: it states how much positional drift over how many ticks
// still proves that both mirrors do the same work.
//
// 120 ticks is two seconds of simulation with dozens of resolved contacts.
// Measured worst drift at 120 ticks is ~1.6e-6; the budget allows ~3x that
// and stays far below any trajectory split.
const (
	parityTicksContacts     = 120  // ticks for the mixed-contacts scenario
	parityToleranceContacts = 5e-6 // max |fixed - float| per position axis

	parityTicksBallistic     = 120  // ticks for the ballistic scenario
	parityToleranceBallistic = 5e-6 // max |fixed - float| per position axis
)

// TestFixedMirrorStaysNearFloatMirror runs both mirrors on the same fixture
// and bounds the positional divergence. It also reports the per-run contact
// totals, which must stay close for the workloads to be comparable.
func TestFixedMirrorStaysNearFloatMirror(t *testing.T) {
	dtF := 1.0 / 60.0
	dtQ := fixed.FromRatio(1, 60)

	scenarios := []struct {
		name      string
		f         *fixture
		ticks     int
		tolerance float64
	}{
		{name: "mixed-contacts", f: spreadFixture(300, 8, 2), ticks: parityTicksContacts, tolerance: parityToleranceContacts},
		{name: "ballistic", f: ballisticFixture(200), ticks: parityTicksBallistic, tolerance: parityToleranceBallistic},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			if sc.ticks == 0 {
				t.Skip("parity budget not set yet (TODO(human) above)")
			}

			floatMirror := newFloatWorld(sc.f)
			fixedMirror := newFixedWorld(sc.f)

			floatContacts, fixedContacts := 0, 0
			for range sc.ticks {
				floatContacts += floatMirror.step(dtF, stepCellSize)
				fixedContacts += fixedMirror.step(dtQ, fixed.FromInt(stepCellSize))
			}
			t.Logf("contacts over %d ticks: float=%d fixed=%d", sc.ticks, floatContacts, fixedContacts)

			worst := 0.0
			for i := range sc.f.n {
				dx := math.Abs(qf(fixedMirror.posX[i]) - floatMirror.posX[i])
				dy := math.Abs(qf(fixedMirror.posY[i]) - floatMirror.posY[i])
				worst = max(worst, dx, dy)
				if dx > sc.tolerance || dy > sc.tolerance {
					t.Fatalf("entity %d drifted: fixed (%v, %v) float (%v, %v)",
						i, qf(fixedMirror.posX[i]), qf(fixedMirror.posY[i]),
						floatMirror.posX[i], floatMirror.posY[i])
				}
			}
			t.Logf("worst position drift: %g (budget %g)", worst, sc.tolerance)
		})
	}
}
