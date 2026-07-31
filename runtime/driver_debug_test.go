package runtime

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestDebugStateValueUnwrapsTheBoxedSnapshot(t *testing.T) {
	slot := StateSlot[int]{State: 41}
	if got := DebugStateValue(slot.GameState()); got != 41 {
		t.Fatalf("DebugStateValue = %v (%T), want 41 (int)", got, got)
	}
	if got := DebugStateValue(nil); got != nil {
		t.Fatalf("DebugStateValue(nil) = %v, want nil", got)
	}
}

func TestDriverDebugGameState(t *testing.T) {
	game := &statefulGame{}
	driver := NewDriver(game, nil)
	driver.Step()

	value, ok := driver.DebugGameState()
	if !ok {
		t.Fatal("DebugGameState ok = false for a StatefulGame")
	}
	if value != 1 {
		t.Fatalf("DebugGameState = %v, want 1 after one step", value)
	}

	// The returned value is a snapshot: a later step must not move it.
	driver.Step()
	if value != 1 {
		t.Fatalf("snapshot moved with the live state: %v, want 1", value)
	}
}

func TestDriverDebugGameStateWithoutStatefulGame(t *testing.T) {
	driver := NewDriver(&bareGame{}, nil)
	if _, ok := driver.DebugGameState(); ok {
		t.Fatal("DebugGameState ok = true for a game without StatefulGame")
	}
}

func TestDriverWorldViewReadsTheLiveWorld(t *testing.T) {
	game := &statefulGame{}
	driver := NewDriver(game, nil)
	driver.Step()

	id := driver.world.Spawn()
	driver.world.SetVelocity(id, scene.Velocity{})

	view := driver.WorldView()
	entities := view.Entities()
	if len(entities) != 1 || entities[0] != id {
		t.Fatalf("WorldView().Entities() = %v, want [%d]", entities, id)
	}
}

func TestDriverDebugWorldMutationsAreVisibleThroughTheView(t *testing.T) {
	game := &statefulGame{}
	driver := NewDriver(game, nil)
	driver.Step()

	id := driver.DebugWorld().Spawn()
	driver.DebugWorld().SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 5}})

	transform, ok := driver.WorldView().Transform(id)
	if !ok || transform.Position.X != 5 {
		t.Fatalf("WorldView().Transform() = (%+v, %v), want the DebugWorld write", transform, ok)
	}
}

// bareGame is a Game without StateSlot: DebugGameState must report not-ok
// instead of panicking.
type bareGame struct{}

func (g *bareGame) Setup() Config                              { return Config{} }
func (g *bareGame) Init(ctx *UpdateContext)                    {}
func (g *bareGame) Update(ctx *UpdateContext, dt float64)      {}
func (g *bareGame) Render(ctx *RenderContext) []render.Command { return nil }
