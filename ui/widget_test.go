package ui

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func testCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
}

func TestProgressBarClampsFillWidth(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	u.progressBarIn(math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 40, Y: 8}), 1.5, SpaceRef)

	commands := u.Commands()
	if len(commands) != 2 {
		t.Fatalf("len(commands) = %v, want 2", len(commands))
	}

	fill, ok := commands[1].(render.DrawRect)
	if !ok {
		t.Fatalf("fill command type = %T, want render.DrawRect", commands[1])
	}
	if fill.Rect.Width() != 40 {
		t.Fatalf("fill width = %v, want clamped full width 40", fill.Rect.Width())
	}
}
