package ui

import (
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestEndGroupAppliesOffsetToAllCommandTypes(t *testing.T) {
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, nil, 0, 0)
	u.BeginGroup(GroupOptions{Offset: math2d.Vec2{X: 10, Y: 20}})
	u.commands = append(u.commands, sampleCommands()...)
	u.EndGroup()

	if got := u.commands[0].(render.DrawRect).Rect.Min; got != (math2d.Vec2{X: 11, Y: 22}) {
		t.Fatalf("DrawRect min = %+v, want (11,22)", got)
	}
	if got := u.commands[1].(render.DrawCircle).Center; got != (math2d.Vec2{X: 13, Y: 24}) {
		t.Fatalf("DrawCircle center = %+v, want (13,24)", got)
	}
	if got := u.commands[2].(render.DrawPoint).Center; got != (math2d.Vec2{X: 15, Y: 26}) {
		t.Fatalf("DrawPoint center = %+v, want (15,26)", got)
	}
	line := u.commands[3].(render.DrawLine)
	if line.From != (math2d.Vec2{X: 17, Y: 28}) || line.To != (math2d.Vec2{X: 19, Y: 30}) {
		t.Fatalf("DrawLine = %+v, want from (17,28) to (19,30)", line)
	}
	if got := u.commands[4].(render.DrawText).Position; got != (math2d.Vec2{X: 21, Y: 32}) {
		t.Fatalf("DrawText position = %+v, want (21,32)", got)
	}
	if got := u.commands[5].(render.DrawArc).Center; got != (math2d.Vec2{X: 23, Y: 34}) {
		t.Fatalf("DrawArc center = %+v, want (23,34)", got)
	}
}

func TestEndGroupAppliesFadeToColors(t *testing.T) {
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, nil, 0, 0)
	u.BeginGroup(GroupOptions{Fade: 0.5})
	u.commands = append(u.commands, sampleCommands()...)
	u.EndGroup()

	rect := u.commands[0].(render.DrawRect)
	if rect.Fill.A != 0.4 || rect.Stroke.A != 0.3 {
		t.Fatalf("DrawRect alpha = fill %v stroke %v, want 0.4 and 0.3", rect.Fill.A, rect.Stroke.A)
	}
	circle := u.commands[1].(render.DrawCircle)
	if circle.Fill.A != 0.35 || circle.Stroke.A != 0.25 {
		t.Fatalf("DrawCircle alpha = fill %v stroke %v, want 0.35 and 0.25", circle.Fill.A, circle.Stroke.A)
	}
	if got := u.commands[2].(render.DrawPoint).Color.A; got != 0.2 {
		t.Fatalf("DrawPoint alpha = %v, want 0.2", got)
	}
	if got := u.commands[3].(render.DrawLine).Color.A; got != 0.15 {
		t.Fatalf("DrawLine alpha = %v, want 0.15", got)
	}
	if got := u.commands[4].(render.DrawText).Color.A; got != 0.1 {
		t.Fatalf("DrawText alpha = %v, want 0.1", got)
	}
	if got := u.commands[5].(render.DrawArc).Color.A; got != 0.05 {
		t.Fatalf("DrawArc alpha = %v, want 0.05", got)
	}
}

func TestZeroValueGroupIsNoOp(t *testing.T) {
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, nil, 0, 0)
	u.BeginGroup(GroupOptions{})
	u.commands = append(u.commands, sampleCommands()...)
	before := append([]render.Command(nil), u.commands...)
	u.EndGroup()

	if !reflect.DeepEqual(u.commands, before) {
		t.Fatalf("zero-value group changed commands:\n got %#v\nwant %#v", u.commands, before)
	}
}

func TestNestedGroupsCompose(t *testing.T) {
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, nil, 0, 0)
	u.BeginGroup(GroupOptions{Offset: math2d.Vec2{X: 10}, Fade: 0.25})
	u.commands = append(u.commands, render.DrawPoint{
		Center: math2d.Vec2{X: 1, Y: 2},
		Color:  render.RGBA(1, 1, 1, 0.8),
	})
	u.BeginGroup(GroupOptions{Offset: math2d.Vec2{Y: 5}, Fade: 0.5})
	u.commands = append(u.commands, render.DrawPoint{
		Center: math2d.Vec2{X: 3, Y: 4},
		Color:  render.RGBA(1, 1, 1, 0.8),
	})
	u.EndGroup()
	u.EndGroup()

	outerOnly := u.commands[0].(render.DrawPoint)
	if outerOnly.Center != (math2d.Vec2{X: 11, Y: 2}) || !closeEnough(outerOnly.Color.A, 0.6) {
		t.Fatalf("outer-only command = %+v, want center (11,2) alpha 0.6", outerOnly)
	}
	nested := u.commands[1].(render.DrawPoint)
	if nested.Center != (math2d.Vec2{X: 13, Y: 9}) || !closeEnough(nested.Color.A, 0.3) {
		t.Fatalf("nested command = %+v, want center (13,9) alpha 0.3", nested)
	}
}

func TestEndGroupWithoutBeginIsNoOp(t *testing.T) {
	u := New(&State{}, math2d.Vec2{}, true, testCam(), DefaultTheme(), testCam().Viewport, nil, 0, 0)
	u.commands = append(u.commands, sampleCommands()...)
	before := append([]render.Command(nil), u.commands...)

	u.EndGroup()

	if !reflect.DeepEqual(u.commands, before) {
		t.Fatalf("EndGroup without Begin changed commands:\n got %#v\nwant %#v", u.commands, before)
	}
}

func TestGroupOffsetRespectsSpace(t *testing.T) {
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
	reference := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 100})
	rect := render.DrawPoint{Center: math2d.Vec2{X: 1, Y: 1}, Color: render.RGBA(1, 1, 1, 1)}

	refUI := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), reference, nil, 0, 0)
	refUI.BeginGroup(GroupOptions{Offset: math2d.Vec2{X: 20}, Space: SpaceRef})
	refUI.commands = append(refUI.commands, rect)
	refUI.EndGroup()

	screenUI := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), reference, nil, 0, 0)
	screenUI.BeginGroup(GroupOptions{Offset: math2d.Vec2{X: 20}, Space: SpaceScreen})
	screenUI.commands = append(screenUI.commands, rect)
	screenUI.EndGroup()

	if got := refUI.commands[0].(render.DrawPoint).Center.X; got != 11 {
		t.Fatalf("SpaceRef offset X = %v, want 11", got)
	}
	if got := screenUI.commands[0].(render.DrawPoint).Center.X; got != 21 {
		t.Fatalf("SpaceScreen offset X = %v, want 21", got)
	}
}

func sampleCommands() []render.Command {
	return []render.Command{
		render.DrawRect{
			Rect:   math2d.NewRect(math2d.Vec2{X: 1, Y: 2}, math2d.Vec2{X: 10, Y: 10}),
			Fill:   render.RGBA(1, 0, 0, 0.8),
			Stroke: render.RGBA(0, 1, 0, 0.6),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 3, Y: 4},
			Fill:   render.RGBA(1, 0, 0, 0.7),
			Stroke: render.RGBA(0, 1, 0, 0.5),
		},
		render.DrawPoint{
			Center: math2d.Vec2{X: 5, Y: 6},
			Color:  render.RGBA(1, 1, 1, 0.4),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 7, Y: 8},
			To:    math2d.Vec2{X: 9, Y: 10},
			Color: render.RGBA(1, 1, 1, 0.3),
		},
		render.DrawText{
			Position: math2d.Vec2{X: 11, Y: 12},
			Text:     "hello",
			Color:    render.RGBA(1, 1, 1, 0.2),
		},
		render.DrawArc{
			Center: math2d.Vec2{X: 13, Y: 14},
			Color:  render.RGBA(1, 1, 1, 0.1),
		},
	}
}

func closeEnough(got, want float64) bool {
	return math.Abs(got-want) <= 1e-9
}
