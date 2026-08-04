package outpost

import (
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
)

// hopProbe watches a unit from inside the tick, which is the only place the
// world is reachable without the dev-only accessors.
type hopProbe struct {
	*Game
	position math2d.Vec2
	peakZ    float64
	hopping  bool
}

func (p *hopProbe) Update(ctx *runtime.UpdateContext, dt float64) {
	p.Game.Update(ctx, dt)

	if transform, ok := ctx.World.Transform(p.units[0]); ok {
		p.position = transform.Position
	}
	if height, ok := ctx.World.Height(p.units[0]); ok && height.Z > p.peakZ {
		p.peakZ = height.Z
	}
	_, p.hopping = ctx.World.HopTween(p.units[0])
}

func clickAt(driver *runtime.Driver, screen math2d.Vec2) {
	events := []input.Event{
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
			Kind:   input.EventButtonDown,
			X:      screen.X, Y: screen.Y,
		},
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
			Kind:   input.EventButtonUp,
			X:      screen.X, Y: screen.Y,
		},
	}
	driver.ApplyInput(events)
	driver.Step()
}

// A click travels screen -> ScreenToWorld -> tile, and the whole isometric
// projection sits in between; the unit then has to fly the arc and land on
// the tile's centre. If either the view affine or the hop bookkeeping went
// wrong, the unit ends up on the wrong tile or hanging in the air.
func TestClickSelectsAndHopsAcrossTheProjection(t *testing.T) {
	probe := &hopProbe{Game: NewGame()}
	driver := runtime.NewDriver(probe, slog.New(slog.NewTextHandler(io.Discard, nil)))
	driver.Step() // Init

	camera := probe.Setup().Camera
	start := unitStarts[0]
	target := [2]int{4, 6} // grass, unoccupied

	clickAt(driver, camera.WorldToScreen(tileCenter(start[0], start[1])))
	if probe.State.Selected != 0 {
		t.Fatalf("Selected after clicking unit 0 = %d, want 0", probe.State.Selected)
	}

	clickAt(driver, camera.WorldToScreen(tileCenter(target[0], target[1])))
	if !probe.hopping {
		t.Fatal("unit 0 is not hopping after clicking a grass tile")
	}

	for range 200 {
		driver.Step()
	}

	unit := probe.State.Units[0]
	if probe.hopping {
		t.Fatal("unit 0 is still hopping after two hundred ticks")
	}
	if unit.TileI != target[0] || unit.TileJ != target[1] {
		t.Fatalf("unit 0 landed on tile (%d, %d), want (%d, %d)", unit.TileI, unit.TileJ, target[0], target[1])
	}
	if want := tileCenter(target[0], target[1]); probe.position != want {
		t.Fatalf("unit 0 rests at %v, want tile centre %v", probe.position, want)
	}
	if probe.peakZ <= 0 {
		t.Fatal("the hop never left the ground: peak Z was never positive")
	}
}
