package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/render"
)

// 2D is a mode, not 2.5D with the dial at neutral: a game that never mentions
// depth has to come out of command generation exactly as it did before 2.5D
// existed, and cost the same. These two tests are that promise.

func TestSceneWithoutProjectionEmitsNoTransforms(t *testing.T) {
	world := benchmarkRenderWorld(60)

	for i, cmd := range world.RenderCommands() {
		transform := commandTransform(t, cmd)
		if !transform.IsIdentity() {
			t.Fatalf("command %d carries %v; a plain 2D scene must emit none", i, transform)
		}
	}
}

func TestRenderCommandsWithoutProjectionDoesNotAllocate(t *testing.T) {
	world := benchmarkRenderWorld(60)
	commands := make([]render.Command, 0, 60)

	allocs := testing.AllocsPerRun(100, func() {
		commands = world.RenderCommandsInto(commands[:0], 0)
	})

	// One allocation per command is the interface boxing that has always been
	// there. Anything above it would be the depth path charging a 2D game for
	// machinery it never asked for.
	if want := float64(len(commands)); allocs != want {
		t.Fatalf("got %v allocations per frame, want %v (one boxed command each)", allocs, want)
	}
}
