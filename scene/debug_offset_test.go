package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func debugCircleCentre(t *testing.T, cmd render.Command) math2d.Vec2 {
	t.Helper()

	circle, ok := cmd.(render.DrawCircle)
	if !ok {
		t.Fatalf("debug command is %T, want render.DrawCircle", cmd)
	}

	return circle.Center
}

// A debug mark follows where the sprite went and ignores how it was deformed.
// Squash and stretch is applied About the entity's position, so it moves the
// sprite nowhere: a collider outline that squashed along with the art would be
// claiming the physics shape changed, which is the one thing the layer exists
// to answer.
func TestDebugColliderIgnoresDecorativeDeformation(t *testing.T) {
	centre := math2d.Vec2{X: 30, Y: 40}

	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: centre, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleCollider(id, scene.CircleCollider{Radius: 5})

	before := world.DebugCommands(scene.DebugColliders)

	world.SetSpriteTransform(id, scene.SpriteTransform{
		M: math2d.Scale(math2d.Vec2{X: 2, Y: 0.5}),
	})

	after := world.DebugCommands(scene.DebugColliders)

	if len(before) != 1 || len(after) != 1 {
		t.Fatalf("got %d/%d debug commands, want 1 each", len(before), len(after))
	}
	if before[0] != after[0] {
		t.Errorf("squash & stretch moved the collider outline:\n before %v\n after  %v", before[0], after[0])
	}
}

// A transform that places the sprite somewhere else does move it, and a mark
// left behind at the entity's raw position would describe nothing. This is the
// case a game hits when it owns a projection the camera does not know about.
func TestDebugColliderFollowsPlacement(t *testing.T) {
	centre := math2d.Vec2{X: 4, Y: 4}
	displacement := math2d.Vec2{X: -4, Y: 92}

	world := scene.NewWorld()
	id := world.Spawn()
	world.SetTransform(id, scene.Transform{Position: centre, Scale: math2d.Vec2{X: 1, Y: 1}})
	world.SetCircleCollider(id, scene.CircleCollider{Radius: 5})
	world.SetSpriteTransform(id, scene.SpriteTransform{M: math2d.Translate(displacement)})

	commands := world.DebugCommands(scene.DebugColliders)
	if len(commands) != 1 {
		t.Fatalf("got %d debug commands, want 1", len(commands))
	}

	want := centre.Add(displacement)
	if got := debugCircleCentre(t, commands[0]); got != want {
		t.Errorf("collider outline at %v, want %v (where the sprite is drawn)", got, want)
	}
}
