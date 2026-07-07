package scene_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

// TestChecksumGolden asserts that the simulation checksum is bit-identical
// across runs and architectures. The golden file records expected checksums at
// ticks 1, 100, and 600 for a dense, fully deterministic world with all 11
// component maps populated and active collisions.
//
// Regenerate (only on x86-64, in a dedicated commit explaining why):
//
//	go test ./scene/... -run TestChecksumGolden -update
var update = flag.Bool("update", false, "update golden checksum fixture")

type goldenFixture struct {
	Ticks     []int    `json:"ticks"`
	Checksums []string `json:"checksums"`
}

func TestChecksumGolden(t *testing.T) {
	bounds := math2d.Rect{
		Min: math2d.Vec2{X: 0, Y: 0},
		Max: math2d.Vec2{X: 100, Y: 80},
	}

	config := scene.StepConfig{
		Bounds: bounds,
		Bounce: scene.BounceConfig{
			UseBodyRestitution: true,
		},
		Physics: scene.PhysicsConfig{CellSize: 64},
	}

	dt := 1.0 / 60.0
	checkpoints := []int{1, 100, 600}

	// First run: collect checksums.
	w := buildGoldenWorld()
	cs1 := runAndCapture(w, dt, config, 600, checkpoints)

	// Second run: auto-consistency check.
	w2 := buildGoldenWorld()
	cs2 := runAndCapture(w2, dt, config, 600, checkpoints)

	for _, tick := range checkpoints {
		if cs1[tick] != cs2[tick] {
			t.Fatalf("auto-consistency failed at tick %d: %d vs %d", tick, cs1[tick], cs2[tick])
		}
	}

	goldenPath := filepath.Join("testdata", "checksum_golden.json")

	if *update {
		checksums := make([]string, len(checkpoints))
		for i, tick := range checkpoints {
			checksums[i] = "0x" + strconv.FormatUint(cs1[tick], 16)
		}
		gf := goldenFixture{
			Ticks:     checkpoints,
			Checksums: checksums,
		}
		data, err := json.MarshalIndent(gf, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, data, 0644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden fixture written to %s", goldenPath)
		return
	}

	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden fixture: %v (regenerate with -update)", err)
	}
	var gf goldenFixture
	if err := json.Unmarshal(raw, &gf); err != nil {
		t.Fatalf("unmarshal golden fixture: %v", err)
	}
	if len(gf.Ticks) != len(checkpoints) || len(gf.Checksums) != len(checkpoints) {
		t.Fatalf("golden fixture has %d entries, want %d", len(gf.Checksums), len(checkpoints))
	}
	for i, tick := range gf.Ticks {
		got := cs1[tick]
		wantHex := gf.Checksums[i]
		want, err := strconv.ParseUint(strings.TrimPrefix(wantHex, "0x"), 16, 64)
		if err != nil {
			t.Fatalf("parse golden checksum %q: %v", wantHex, err)
		}
		if got != want {
			t.Errorf("tick %d: checksum mismatch\n  got:  0x%016x\n  want: %s\n  (divergence detected)",
				tick, got, wantHex)
		}
	}
}

// runAndCapture runs N ticks and returns checksums at the requested checkpoints.
func runAndCapture(w *scene.World, dt float64, config scene.StepConfig, totalTicks int, checkpoints []int) map[int]uint64 {
	cpSet := make(map[int]bool, len(checkpoints))
	for _, cp := range checkpoints {
		cpSet[cp] = true
	}
	results := make(map[int]uint64, len(checkpoints))

	for tick := 1; tick <= totalTicks; tick++ {
		w.Step(dt, config)
		if cpSet[tick] {
			results[tick] = w.Checksum()
		}
	}
	return results
}

// buildGoldenWorld creates a dense, fully deterministic world with ~24
// entities covering all 11 component maps. Positions, velocities, and
// accelerations are arranged so that collisions occur within the 100×80
// bounds, exercising the full simulation pipeline.
func buildGoldenWorld() *scene.World {
	w := scene.NewWorld()

	// Use "dirty" non-terminating binary fractions to stress FMA.
	var (
		third   = 1.0 / 3.0
		seventh = 1.0 / 7.0
		ninth   = 1.0 / 9.0
	)

	// ---- Circle entities (8) ----
	// Left group moving right, right group moving left → collide in middle.
	circleData := []struct {
		pos         math2d.Vec2
		vel         math2d.Vec2
		accel       math2d.Vec2
		damp        float64
		radius      float64
		invMass     float64
		restitution float64
	}{
		{pos: math2d.Vec2{X: 10, Y: 10}, vel: math2d.Vec2{X: 3, Y: 0.5}, damp: 0.1, radius: 4, invMass: 1, restitution: 0.3},
		{pos: math2d.Vec2{X: 15, Y: 20}, vel: math2d.Vec2{X: 2.5, Y: third}, damp: 0.05, radius: 3, invMass: 1, restitution: 0.5},
		{pos: math2d.Vec2{X: 12, Y: 35}, vel: math2d.Vec2{X: 4, Y: -0.5}, damp: 0, radius: 5, invMass: 1, restitution: 0.8},
		{pos: math2d.Vec2{X: 20, Y: 50}, vel: math2d.Vec2{X: 2, Y: 1}, damp: 0.2, radius: 3.5, invMass: 1, restitution: 0.4},
		{pos: math2d.Vec2{X: 85, Y: 12}, vel: math2d.Vec2{X: -3, Y: seventh}, damp: 0.08, radius: 4, invMass: 1, restitution: 0.5},
		{pos: math2d.Vec2{X: 80, Y: 25}, vel: math2d.Vec2{X: -2.5, Y: -0.75}, damp: 0.15, radius: 3, invMass: 1, restitution: 0.6},
		{pos: math2d.Vec2{X: 90, Y: 40}, vel: math2d.Vec2{X: -4, Y: 0.25}, damp: 0, radius: 5, invMass: 1, restitution: 0.2},
		{pos: math2d.Vec2{X: 78, Y: 55}, vel: math2d.Vec2{X: -2, Y: -1}, damp: 0.12, radius: 3.5, invMass: 1, restitution: 0.7},
	}

	for _, cd := range circleData {
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{
			Position: cd.pos,
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetVelocity(id, scene.Velocity{Linear: cd.vel})
		w.SetAcceleration(id, scene.Acceleration{Linear: cd.accel})
		w.SetDamping(id, scene.Damping{Linear: cd.damp})
		w.SetBody(id, scene.Body{InvMass: cd.invMass, Restitution: cd.restitution})
		w.SetCircleCollider(id, scene.CircleCollider{Radius: cd.radius})
		w.SetCircleSprite(id, scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.2, 0.4, 0.6, 1)},
			Radius:     cd.radius,
		})
	}



	// ---- Rect entities (6) ----
	rectData := []struct {
		pos         math2d.Vec2
		vel         math2d.Vec2
		halfExt     math2d.Vec2
		invMass     float64
		restitution float64
	}{
		{pos: math2d.Vec2{X: 30, Y: 15}, vel: math2d.Vec2{X: 1.5, Y: 0}, halfExt: math2d.Vec2{X: 3, Y: 2}, invMass: 1, restitution: 0.5},
		{pos: math2d.Vec2{X: 70, Y: 18}, vel: math2d.Vec2{X: -1.5, Y: 0}, halfExt: math2d.Vec2{X: 3, Y: 2}, invMass: 1, restitution: 0.5},
		{pos: math2d.Vec2{X: 45, Y: 65}, vel: math2d.Vec2{X: 0, Y: -1.25}, halfExt: math2d.Vec2{X: 2, Y: 3}, invMass: 1, restitution: 0.4},
		{pos: math2d.Vec2{X: 55, Y: 68}, vel: math2d.Vec2{X: 0, Y: 1.25}, halfExt: math2d.Vec2{X: 2, Y: 3}, invMass: 1, restitution: 0.4},
		{pos: math2d.Vec2{X: 50, Y: 45}, vel: math2d.Vec2{X: 1, Y: 1}, halfExt: math2d.Vec2{X: 4, Y: 4}, invMass: 1, restitution: 0.6},
		{pos: math2d.Vec2{X: 52, Y: 43}, vel: math2d.Vec2{X: -1, Y: -1}, halfExt: math2d.Vec2{X: 4, Y: 4}, invMass: 1, restitution: 0.6},
	}

	for _, rd := range rectData {
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{
			Position: rd.pos,
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetVelocity(id, scene.Velocity{Linear: rd.vel})
		w.SetRectCollider(id, scene.RectCollider{HalfExtends: rd.halfExt})
		w.SetRectSprite(id, scene.RectSprite{
			BaseSprite: scene.BaseSprite{Layer: 2, Color: render.RGBA(0.8, 0.3, 0.2, 1)},
			Rect: math2d.Rect{
				Min: math2d.Vec2{X: -rd.halfExt.X, Y: -rd.halfExt.Y},
				Max: math2d.Vec2{X: rd.halfExt.X, Y: rd.halfExt.Y},
			},
			Stroke: render.RGBA(1, 1, 1, 1),
		})
	}

	// ---- Static entities (3, zero InvMass) ----
	for i := 0; i < 3; i++ {
		id := w.Spawn()
		x := 40.0 + float64(i)*8.0
		w.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: x, Y: 72},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetCircleCollider(id, scene.CircleCollider{Radius: 2})
		w.SetBody(id, scene.Body{InvMass: 0, Restitution: 0.5})
		w.SetCircleSprite(id, scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.5, 0.5, 0.5, 1)},
			Radius:     2,
		})
	}

	// ---- Entity with TransformTween active ----
	{
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: 5, Y: 75},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetTransformTween(id, scene.TransformTween{
			From: scene.Transform{
				Position: math2d.Vec2{X: 5, Y: 75},
				Scale:    math2d.Vec2{X: 1, Y: 1},
			},
			To: scene.Transform{
				Position: math2d.Vec2{X: 15, Y: 75},
				Scale:    math2d.Vec2{X: 1.5, Y: 1.5},
			},
			Duration: 5.0,
			Ease:     ease.Linear,
			Loop:     scene.LoopPingPong,
		})
		w.SetRectSprite(id, scene.RectSprite{
			BaseSprite: scene.BaseSprite{Layer: 3, Color: render.RGBA(0.1, 0.9, 0.3, 1)},
			Rect:       math2d.Rect{Min: math2d.Vec2{X: -2, Y: -2}, Max: math2d.Vec2{X: 2, Y: 2}},
		})
	}

	// ---- Entity with CircleSpriteTween active ----
	{
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: 25, Y: 5},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
			From: scene.CircleSprite{
				BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.9, 0.2, 0.2, 1)},
				Radius:     2,
			},
			To: scene.CircleSprite{
				BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.2, 0.9, 0.2, 1)},
				Radius:     4,
			},
			Duration: 3.0,
			Ease:     ease.OutQuad,
			Loop:     scene.LoopPingPong,
		})
		w.SetCircleSprite(id, scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.9, 0.2, 0.2, 1)},
			Radius:     2,
		})
	}

	// ---- Entity with Body + Damping but no Velocity (damping is no-op) ----
	{
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: 95, Y: 5},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetBody(id, scene.Body{InvMass: 1, Restitution: 0.9})
		w.SetDamping(id, scene.Damping{Linear: 0.5})
		w.SetCircleCollider(id, scene.CircleCollider{Radius: 2})
		w.SetCircleSprite(id, scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.3, 0.3, 0.9, 1)},
			Radius:     2,
		})
	}

	// ---- Entity with only Acceleration (no Velocity) ----
	{
		id := w.Spawn()
		w.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: 3, Y: 40},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetAcceleration(id, scene.Acceleration{Linear: math2d.Vec2{X: 0.5, Y: ninth}})
		w.SetCircleCollider(id, scene.CircleCollider{Radius: 2.5})
		w.SetCircleSprite(id, scene.CircleSprite{
			BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.7, 0.7, 0.2, 1)},
			Radius:     2.5,
		})
	}

	return w
}

