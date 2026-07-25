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
