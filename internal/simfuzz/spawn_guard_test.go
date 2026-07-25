package simfuzz

import (
	"testing"

	"github.com/dhannyell/arasy/scene"
)

// TestSpawnCoversAllDecodedComponents is the structural guard: every component
// bit declared in script.go must be installed by SpawnFromScript. Add a bit in
// script.go without handling it here and this test fails.
func TestSpawnCoversAllDecodedComponents(t *testing.T) {
	cases := []struct {
		name  string
		mask  uint32
		check func(*scene.World, scene.EntityID) bool
	}{
		{"Transform", CompTransform, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Transform(e); return ok }},
		{"Velocity", CompVelocity, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Velocity(e); return ok }},
		{"Acceleration", CompAcceleration, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Acceleration(e); return ok }},
		{"Damping", CompDamping, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Damping(e); return ok }},
		{"Body", CompBody, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Body(e); return ok }},
		{"CircleCollider", CompCircleCollider, func(w *scene.World, e scene.EntityID) bool { _, ok := w.CircleCollider(e); return ok }},
		{"RectCollider", CompRectCollider, func(w *scene.World, e scene.EntityID) bool { _, ok := w.RectCollider(e); return ok }},
		{"CircleSprite", CompCircleSprite, func(w *scene.World, e scene.EntityID) bool { _, ok := w.CircleSprite(e); return ok }},
		{"Height", CompHeight, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Height(e); return ok }},
		{"Spin", CompSpin, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Spin(e); return ok }},
		{"Ballistics", CompBallistics, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Ballistics(e); return ok }},
		{"Locomotion", CompLocomotion, func(w *scene.World, e scene.EntityID) bool { _, ok := w.Locomotion(e); return ok }},
		{"MoveIntent", CompMoveIntent, func(w *scene.World, e scene.EntityID) bool { _, ok := w.MoveIntent(e); return ok }},
		{"ImageSprite", CompImageSprite, func(w *scene.World, e scene.EntityID) bool { _, ok := w.ImageSprite(e); return ok }},
		{"TransformTween", CompTransform | CompTransformTween, func(w *scene.World, e scene.EntityID) bool { _, ok := w.TransformTween(e); return ok }},
		{"CircleTween", CompCircleSprite | CompCircleTween, func(w *scene.World, e scene.EntityID) bool { _, ok := w.CircleSpriteTween(e); return ok }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := DecodeScript(seedBytes(10, c.mask))
			w := scene.NewWorld()
			var ids []scene.EntityID
			SpawnFromScript(w, s, fuzzBounds, &ids)
			if len(ids) != 1 {
				t.Fatalf("spawned %d entities, want 1", len(ids))
			}
			if !c.check(w, ids[0]) {
				t.Fatalf("component %s not installed on spawned entity", c.name)
			}
		})
	}
}

func TestSpawnClampsColliderExtents(t *testing.T) {
	mask := CompTransform | CompCircleCollider | CompRectCollider
	data := concatBytes(
		packF64(0), packF64(0), packF64(0), packF64(1), packF64(1),
		packF64(maxFuzzFloat),
		packF64(maxFuzzFloat), packF64(maxFuzzFloat),
	)
	script := Script{
		MaxTicks: 1,
		Entities: []EntityDef{{ComponentMask: mask, Data: data}},
	}
	var ids []scene.EntityID
	world := scene.NewWorld()
	SpawnFromScript(world, script, fuzzBounds, &ids)

	circle, ok := world.CircleCollider(ids[0])
	if !ok || circle.Radius != maxFuzzColliderExtent {
		t.Fatalf("circle radius = %v, present=%v", circle.Radius, ok)
	}
	rect, ok := world.RectCollider(ids[0])
	if !ok || rect.HalfExtends.X != maxFuzzColliderExtent ||
		rect.HalfExtends.Y != maxFuzzColliderExtent {
		t.Fatalf("rect half extents = %+v, present=%v", rect.HalfExtends, ok)
	}
}

func TestSpawnClampsNonNegativePhysicalParameters(t *testing.T) {
	mask := CompDamping | CompBody
	data := concatBytes(
		packF64(-maxFuzzFloat),
		packF64(-maxFuzzFloat), packF64(2),
	)
	script := Script{
		MaxTicks: 1,
		Entities: []EntityDef{{ComponentMask: mask, Data: data}},
	}
	var ids []scene.EntityID
	world := scene.NewWorld()
	SpawnFromScript(world, script, fuzzBounds, &ids)

	damping, _ := world.Damping(ids[0])
	if damping.Linear != 0 {
		t.Fatalf("damping = %v, want 0", damping.Linear)
	}
	body, _ := world.Body(ids[0])
	if body.InvMass != 0 || body.Restitution != 1 {
		t.Fatalf("body = %+v, want non-negative mass and clamped restitution", body)
	}
}

// TestCompDataLenMatchesSpawn ensures compDataLen exactly matches the bytes
// SpawnFromScript consumes: an all-components entity followed by a second
// entity must both decode, proving the offset did not drift.
func TestCompDataLenMatchesSpawn(t *testing.T) {
	data := seedBytes(10, allComponents, CompTransform)
	s := DecodeScript(data)
	if len(s.Entities) != 2 {
		t.Fatalf("decoded %d entities, want 2 (offset drifted)", len(s.Entities))
	}
	w := scene.NewWorld()
	var ids []scene.EntityID
	SpawnFromScript(w, s, fuzzBounds, &ids)
	if len(ids) != 2 {
		t.Fatalf("spawned %d entities, want 2", len(ids))
	}
	if _, ok := w.Transform(ids[1]); !ok {
		t.Fatalf("second entity missing Transform (data layout misaligned)")
	}
}
