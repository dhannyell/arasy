package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestImageSpriteLifecycle(t *testing.T) {
	w := scene.NewWorld()
	id := w.Spawn()
	want := testImageSprite()

	w.SetImageSprite(id, want)
	if got, ok := w.ImageSprite(id); !ok || got != want {
		t.Fatalf("ImageSprite() = (%+v, %v), want (%+v, true)", got, ok, want)
	}

	clone := w.Clone()
	changed := want
	changed.Size.X++
	clone.SetImageSprite(id, changed)
	if got, _ := w.ImageSprite(id); got != want {
		t.Fatalf("mutating clone changed source ImageSprite: got %+v, want %+v", got, want)
	}

	w.RemoveImageSprite(id)
	if got, ok := w.ImageSprite(id); ok || got != (scene.ImageSprite{}) {
		t.Fatalf("ImageSprite() after remove = (%+v, %v), want zero, false", got, ok)
	}

	w.SetImageSprite(id, want)
	w.Despawn(id)
	if _, ok := w.ImageSprite(id); ok {
		t.Fatal("ImageSprite still present after Despawn")
	}
}

func TestSpawnBundleSetsTransformAndImageSprite(t *testing.T) {
	w := scene.NewWorld()
	transform := scene.Transform{
		Position: math2d.Vec2{X: 3, Y: 4},
		Rotation: 0.25,
	}
	sprite := testImageSprite()
	id := w.SpawnBundle(scene.Bundle{Transform: &transform, ImageSprite: &sprite})

	gotTransform, ok := w.Transform(id)
	if !ok {
		t.Fatal("Bundle did not set Transform")
	}
	if gotTransform.Position != transform.Position || gotTransform.Rotation != transform.Rotation || gotTransform.Scale != (math2d.Vec2{X: 1, Y: 1}) {
		t.Fatalf("Transform = %+v, want position/rotation from bundle and default unit scale", gotTransform)
	}
	if got, ok := w.ImageSprite(id); !ok || got != sprite {
		t.Fatalf("ImageSprite() = (%+v, %v), want (%+v, true)", got, ok, sprite)
	}
}

func TestWorldViewExposesImageSprite(t *testing.T) {
	w := scene.NewWorld()
	id := w.Spawn()
	want := testImageSprite()
	w.SetImageSprite(id, want)

	if got, ok := w.View().ImageSprite(id); !ok || got != want {
		t.Fatalf("WorldView.ImageSprite() = (%+v, %v), want (%+v, true)", got, ok, want)
	}
}

func testImageSprite() scene.ImageSprite {
	return scene.ImageSprite{
		Layer:   7,
		Texture: render.TextureID(3),
		Src:     math2d.NewRect(math2d.Vec2{X: 2, Y: 4}, math2d.Vec2{X: 14, Y: 20}),
		Size:    math2d.Vec2{X: 8, Y: 6},
		Tint:    render.RGBA(0.8, 0.7, 0.6, 0.5),
	}
}
