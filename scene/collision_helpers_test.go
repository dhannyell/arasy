package scene_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func assertContactPairs(t *testing.T, contacts []scene.Contact, want [][2]scene.EntityID) {
	t.Helper()

	if len(contacts) != len(want) {
		t.Fatalf("got %d contacts, want %d", len(contacts), len(want))
	}

	for i := range want {
		if contacts[i].A != want[i][0] || contacts[i].B != want[i][1] {
			t.Fatalf(
				"contact %d got pair (%d, %d), want (%d, %d)",
				i,
				contacts[i].A,
				contacts[i].B,
				want[i][0],
				want[i][1],
			)
		}
	}
}

func contactPairs(contacts []scene.Contact) [][2]scene.EntityID {
	pairs := make([][2]scene.EntityID, 0, len(contacts))
	for _, contact := range contacts {
		pairs = append(pairs, [2]scene.EntityID{contact.A, contact.B})
	}

	return pairs
}

func spawnCollidableCircle(world *scene.World, position math2d.Vec2, radius float64) scene.EntityID {
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleCollider(entity, scene.CircleCollider{
		Radius: radius,
	})
	world.SetBody(entity, scene.Body{
		InvMass:     1,
		Restitution: 1,
	})

	return entity
}

func spawnCollidableRect(world *scene.World, position, halfExtends math2d.Vec2) scene.EntityID {
	entity := world.Spawn()

	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetRectCollider(entity, scene.RectCollider{
		HalfExtends: halfExtends,
	})
	world.SetBody(entity, scene.Body{
		InvMass:     1,
		Restitution: 1,
	})

	return entity
}

func spawnStaticRect(world *scene.World, position, halfExtends math2d.Vec2) scene.EntityID {
	entity := spawnCollidableRect(world, position, halfExtends)
	world.SetBody(entity, scene.Body{
		InvMass:     0,
		Restitution: 1,
	})

	return entity
}
