package simfuzz

import (
	"testing"

	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// TestRenderDrawsEveryLiveEntity keeps a recorded fuzz case watchable. Nothing
// else in this package exercises Render: the targets drive Driver.Step, and
// Render runs only from Driver.Frame, so without this test the fuzz game could
// silently go back to drawing nothing and every replay of a failing case would
// open on an empty viewport.
func TestRenderDrawsEveryLiveEntity(t *testing.T) {
	script := DecodeScript(seedBytes(10,
		CompTransform|CompVelocity|CompCircleCollider,
		CompTransform|CompBody|CompRectCollider))
	game := NewGame(script)
	driver := runtime.NewDriver(game, quietLog)
	driver.Step()

	live := len(game.trackedIDs())
	if live == 0 {
		t.Fatal("script spawned no entities, so the shape count below proves nothing")
	}

	shapes := 0
	for _, cmd := range driver.Frame(0).Commands {
		switch cmd.(type) {
		case render.DrawCircle, render.DrawRect:
			shapes++
		}
	}
	// One shape per live entity, plus the bounds rectangle.
	if want := live + 1; shapes < want {
		t.Fatalf("Render emitted %d shapes for %d entities, want at least %d",
			shapes, live, want)
	}
}

// TestRenderLiftsEntityByHeight covers the 2.5D half: an entity off the ground
// must be drawn lifted and given a shadow, which is what makes a ballistic
// case readable rather than a dot sliding around the floor. Reading Z here
// depends on WorldView.Height, so this also pins that the view keeps exposing
// it.
func TestRenderLiftsEntityByHeight(t *testing.T) {
	const groundY, z = 0.0, 20.0
	script := DecodeScript(seedBytesWithData(2, CompTransform|CompHeight, concatBytes(
		packF64(0), packF64(groundY), packF64(0), packF64(1), packF64(1), // Transform
		packF64(z), packF64(0), // Height: Z, VZ
	)))
	driver := runtime.NewDriver(NewGame(script), quietLog)
	driver.Step()

	var marker *render.DrawCircle
	shadows := 0
	for _, cmd := range driver.Frame(0).Commands {
		switch shape := cmd.(type) {
		case render.DrawCircle:
			marker = &shape
		case render.DrawEllipse:
			shadows++
		}
	}

	if marker == nil {
		t.Fatal("Render drew no marker for the airborne entity")
	}
	if want := groundY - z*fuzzProjection.LiftPerZ; marker.Center.Y != want {
		t.Errorf("marker drawn at Y=%v, want %v: the entity was not lifted by its height",
			marker.Center.Y, want)
	}
	if shadows != 1 {
		t.Errorf("got %d ground shadows, want exactly 1", shadows)
	}
}
