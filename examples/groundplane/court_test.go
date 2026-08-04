package groundplane

import (
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
)

// courtProbe watches the ball from inside the tick, which is the only place
// the world is reachable without the dev-only accessors.
type courtProbe struct {
	*Game
	escaped   bool
	escapedAt math2d.Vec2
}

func (p *courtProbe) Update(ctx *runtime.UpdateContext, dt float64) {
	p.Game.Update(ctx, dt)

	transform, ok := ctx.World.Transform(p.ball)
	if !ok || p.escaped {
		return
	}

	position := transform.Position
	if position.X < courtBounds.Min.X || position.X > courtBounds.Max.X ||
		position.Y < courtBounds.Min.Y || position.Y > courtBounds.Max.Y {
		p.escaped = true
		p.escapedAt = position
	}
}

// The ball used to roll off the court and out of the world, because
// Config.Bounds only acts on entities that carry a collider and this one had
// none: the altitude bounce comes from Ballistics and needs nothing, so the
// scene looked right until the ball settled and kept going.
//
// Long enough to cover several serves, since it is the roll after a landing
// that escaped.
func TestBallStaysOnTheCourt(t *testing.T) {
	probe := &courtProbe{Game: &Game{}}
	driver := runtime.NewDriver(probe, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for range 2000 {
		driver.Step()
	}

	if probe.escaped {
		t.Fatalf("the ball left the court at %v, bounds are %v", probe.escapedAt, courtBounds)
	}
}
