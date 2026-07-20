package scene

import (
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestWorldEntityCountCacheInvalidatesOnComponentChanges(t *testing.T) {
	w := NewWorld()
	first := w.Spawn()
	second := w.Spawn()

	w.SetTransform(first, Transform{})
	if got := w.entityCount(); got != 1 {
		t.Fatalf("entityCount after first transform = %d, want 1", got)
	}
	if got := w.entityCount(); got != 1 {
		t.Fatalf("cached entityCount = %d, want 1", got)
	}

	w.SetVelocity(first, Velocity{})
	if got := w.entityCount(); got != 1 {
		t.Fatalf("entityCount after second component on same entity = %d, want 1", got)
	}

	w.SetVelocity(second, Velocity{})
	if got := w.entityCount(); got != 2 {
		t.Fatalf("entityCount after second entity = %d, want 2", got)
	}

	w.RemoveVelocity(second)
	if got := w.entityCount(); got != 1 {
		t.Fatalf("entityCount after removing only component from second entity = %d, want 1", got)
	}

	w.Despawn(first)
	if got := w.entityCount(); got != 0 {
		t.Fatalf("entityCount after despawn = %d, want 0", got)
	}
}

func TestDespawnBatchMatchesSequentialDespawn(t *testing.T) {
	cases := map[string][]EntityID{
		"ascending":          {1, 3, 5, 7},
		"descending":         {7, 5, 3, 1},
		"arbitrary":          {6, 2, 8, 4},
		"duplicates-unknown": {2, 9, 2, 6, 99, 6},
	}

	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			sequential := batchTestWorld()
			batch := batchTestWorld()
			for _, id := range ids {
				sequential.Despawn(id)
			}
			batch.DespawnBatch(ids)
			assertBatchWorldEqual(t, batch, sequential)
		})
	}
}

func TestDespawnBatchEmptyAndAll(t *testing.T) {
	w := batchTestWorld()
	before := w.Clone()
	w.DespawnBatch(nil)
	assertBatchWorldEqual(t, w, before)

	sequential := batchTestWorld()
	batch := batchTestWorld()
	ids := make([]EntityID, 0, 8)
	for id := EntityID(1); id <= 8; id++ {
		ids = append(ids, id)
		sequential.Despawn(id)
	}
	batch.DespawnBatch(ids)
	assertBatchWorldEqual(t, batch, sequential)
}

func TestDespawnBatchCloneIsIndependent(t *testing.T) {
	w := batchTestWorld()
	w.DespawnBatch([]EntityID{2, 4, 6, 8})
	clone := w.Clone()

	w.SetTransform(1, Transform{Position: math2d.Vec2{X: 99, Y: 42}, Scale: math2d.Vec2{X: 1, Y: 1}})
	got, ok := clone.Transform(1)
	if !ok || got.Position == (math2d.Vec2{X: 99, Y: 42}) {
		t.Fatal("Clone after DespawnBatch shares component storage with its source")
	}
	if _, ok := clone.Transform(2); ok {
		t.Fatal("Clone after DespawnBatch restored a removed entity")
	}
}

func batchTestWorld() *World {
	w := NewWorld()
	for i := 1; i <= 8; i++ {
		id := w.Spawn()
		w.SetTransform(id, Transform{
			Position: math2d.Vec2{X: float64(i), Y: float64(i * 2)},
			Rotation: float64(i) * 0.1,
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		w.SetImageSprite(id, ImageSprite{
			Texture: render.TextureID(i),
			Size:    math2d.Vec2{X: float64(i), Y: float64(i + 1)},
			Tint:    render.RGBA(0.2, 0.4, 0.6, 1),
		})
		if i%2 == 0 {
			w.SetVelocity(id, Velocity{Linear: math2d.Vec2{X: float64(i), Y: 1}})
			w.SetDamping(id, Damping{Linear: float64(i)})
		}
		if i%3 == 0 {
			w.SetCircleSprite(id, CircleSprite{Radius: float64(i)})
			w.SetCircleCollider(id, CircleCollider{Radius: float64(i)})
		}
		if i%4 == 0 {
			w.SetRectSprite(id, RectSprite{})
			w.SetBody(id, Body{InvMass: 1})
		}
	}
	return w
}

func assertBatchWorldEqual(t *testing.T, got, want *World) {
	t.Helper()
	if gotChecksum, wantChecksum := got.Checksum(), want.Checksum(); gotChecksum != wantChecksum {
		t.Fatalf("Checksum() = %d, want %d", gotChecksum, wantChecksum)
	}
	if gotCount, wantCount := got.entityCount(), want.entityCount(); gotCount != wantCount {
		t.Fatalf("entityCount() = %d, want %d", gotCount, wantCount)
	}
	if !equalStore(got.transforms, want.transforms) ||
		!equalStore(got.transformTweens, want.transformTweens) ||
		!equalStore(got.circleSprites, want.circleSprites) ||
		!equalStore(got.circleTweens, want.circleTweens) ||
		!equalStore(got.rectSprites, want.rectSprites) ||
		!equalStore(got.imageSprites, want.imageSprites) ||
		!equalStore(got.velocities, want.velocities) ||
		!equalStore(got.accelerations, want.accelerations) ||
		!equalStore(got.dampings, want.dampings) ||
		!equalStore(got.bodies, want.bodies) ||
		!equalStore(got.circleColliders, want.circleColliders) ||
		!equalStore(got.rectColliders, want.rectColliders) ||
		!equalStore(got.renderTransforms, want.renderTransforms) {
		t.Fatal("DespawnBatch stores differ from sequential Despawn")
	}
}

func equalStore[T any](got, want store[T]) bool {
	return reflect.DeepEqual(got.ids, want.ids) && reflect.DeepEqual(got.dense, want.dense)
}
