package render

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

// iso21Camera is the classic 2:1 view these helpers were built for. Under it
// the lift vector must come out (-1, -1): GroundProjection derives it, and
// everything below leans on that known answer.
func iso21Projection(t *testing.T) Projection {
	t.Helper()
	p := GroundProjection(Camera{
		World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		View:  math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5},
	}, ProjectionLight{})

	lift := p.LiftVector()
	if lift.Sub(math2d.Vec2{X: -1, Y: -1}).Len() > 1e-9 {
		t.Fatalf("LiftVector under the 2:1 view = %v, want (-1, -1)", lift)
	}
	return p
}

// The whole point of ExtrudeRect is choosing the walls the viewer can see and
// sweeping them along the lift. Under the 2:1 view the viewer sits at +X+Y, so
// the Max.X and Max.Y walls come out, and every top corner is the matching
// bottom corner moved one lift vector per unit of height.
func TestExtrudeRectUnderIsometricView(t *testing.T) {
	p := iso21Projection(t)

	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 8, Y: 8})
	commands := p.ExtrudeRect(nil, rect, 0, 2, BlockStyle{}, 7)

	if len(commands) != 3 {
		t.Fatalf("ExtrudeRect emitted %d commands, want 2 walls + 1 top", len(commands))
	}

	wallX, ok := commands[0].(DrawPolygon)
	if !ok {
		t.Fatalf("first command is %T, want the X wall as DrawPolygon", commands[0])
	}
	// The X wall stands on the rect's Max.X edge; its top edge is that edge
	// lifted by z1 * (-1, -1).
	if got, want := wallX.Points[0], (math2d.Vec2{X: 16, Y: 18}); got != want {
		t.Fatalf("X wall top corner = %v, want %v", got, want)
	}
	if got, want := wallX.Points[3], (math2d.Vec2{X: 18, Y: 20}); got != want {
		t.Fatalf("X wall bottom corner = %v, want %v", got, want)
	}

	top, ok := commands[2].(DrawRect)
	if !ok {
		t.Fatalf("last command is %T, want the top as DrawRect", commands[2])
	}
	if got, want := top.Rect.Min, (math2d.Vec2{X: 8, Y: 18}); got != want {
		t.Fatalf("top rect Min = %v, want %v", got, want)
	}
	if top.Layer() != 7 {
		t.Fatalf("top layer = %d, want 7", top.Layer())
	}
}

// A camera that does not lift has no side to show: pretending otherwise would
// paint walls into a flat scene. Only the top may come out.
func TestExtrudeRectWithoutLiftEmitsOnlyTheTop(t *testing.T) {
	commands := Projection{}.ExtrudeRect(nil, math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}), 0, 3, BlockStyle{}, 0)
	if len(commands) != 1 {
		t.Fatalf("flat projection emitted %d commands, want only the top", len(commands))
	}
	if _, ok := commands[0].(DrawRect); !ok {
		t.Fatalf("flat projection emitted %T, want the top DrawRect", commands[0])
	}
}
