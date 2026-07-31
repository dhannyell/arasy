package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/scene"
)

func TestEntitiesUnionsEveryComponentStoreInOrder(t *testing.T) {
	world := scene.NewWorld()

	a := world.Spawn() // 0: transform only
	world.SetTransform(a, scene.Transform{})
	b := world.Spawn() // 1: velocity only — no transform
	world.SetVelocity(b, scene.Velocity{})
	c := world.Spawn() // 2: several components, counted once
	world.SetTransform(c, scene.Transform{})
	world.SetVelocity(c, scene.Velocity{})
	world.SetBody(c, scene.Body{})

	got := world.Entities()
	want := []scene.EntityID{a, b, c}
	if len(got) != len(want) {
		t.Fatalf("Entities() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Entities()[%d] = %d, want %d (full list %v)", i, got[i], want[i], got)
		}
	}
}

func TestEntitiesDropsDespawnedAndIgnoresPresentationStores(t *testing.T) {
	world := scene.NewWorld()

	a := world.Spawn()
	world.SetTransform(a, scene.Transform{})
	b := world.Spawn()
	world.SetVelocity(b, scene.Velocity{})

	// A sprite transform is a render-only cache: it must not keep an entity
	// alive on the list once its simulation components are gone.
	world.SetSpriteTransform(a, scene.SpriteTransform{})
	world.RemoveTransform(a)

	got := world.Entities()
	if len(got) != 1 || got[0] != b {
		t.Fatalf("Entities() = %v, want [%d]", got, b)
	}

	world.Despawn(b)
	if got := world.Entities(); len(got) != 0 {
		t.Fatalf("Entities() after Despawn = %v, want empty", got)
	}
}
