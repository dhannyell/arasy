package render_test

import (
	"errors"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestBuilderComposesNestedGroups(t *testing.T) {
	var builder render.Builder
	builder.Reset(make([]render.Command, 0, 1))
	builder.Push(render.Group{
		Transform: math2d.Translate(math2d.Vec2{X: 10}),
		Opacity:   0.5, Clip: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		LayerOffset: 2,
	})
	builder.Push(render.Group{
		Transform: math2d.Scale(math2d.Vec2{X: 2, Y: 3}),
		Opacity:   0.5, Clip: math2d.NewRect(math2d.Vec2{X: 5}, math2d.Vec2{X: 10, Y: 5}),
		LayerOffset: 3,
	})
	builder.Add(render.DrawCircle{
		BaseCommand: render.BaseCommand{LayerValue: 7},
		Center:      math2d.Vec2{X: 1, Y: 2},
		Fill:        render.Color{R: 1, A: 0.8},
	})
	builder.Pop()
	builder.Pop()

	commands := builder.Commands()
	if err := builder.Err(); err != nil {
		t.Fatal(err)
	}
	command := commands[0].(render.DrawCircle)
	if got := command.Transform.Apply(command.Center); got != (math2d.Vec2{X: 12, Y: 6}) {
		t.Fatalf("transformed center = %v, want (12,6)", got)
	}
	if command.Fill.A != 0.2 || command.Layer() != 12 {
		t.Fatalf("alpha/layer = %v/%v, want 0.2/12", command.Fill.A, command.Layer())
	}
	if command.Clip != (math2d.Rect{Min: math2d.Vec2{X: 5}, Max: math2d.Vec2{X: 10, Y: 5}}) {
		t.Fatalf("clip = %v", command.Clip)
	}
}

func TestBuilderCoversStreamingCommands(t *testing.T) {
	white := render.Color{R: 1, G: 1, B: 1, A: 1}
	commands := []render.Command{
		render.DrawRect{Fill: white}, render.DrawCircle{Fill: white},
		render.DrawEllipse{Fill: white}, render.DrawPoint{Color: white},
		render.DrawLine{Color: white}, render.DrawText{Color: white},
		render.DrawArc{Color: white}, render.DrawPolygon{Fill: white},
		render.DrawTriangles{Color: white},
		render.DrawImage{}, render.DrawImageQuad{},
	}
	for _, command := range commands {
		var builder render.Builder
		builder.Reset(nil)
		builder.Push(render.Group{Transform: math2d.Translate(math2d.Vec2{X: 1}), Opacity: 0.5})
		builder.Add(command)
		builder.Pop()
		if builder.Err() != nil || len(builder.Commands()) != 1 {
			t.Fatalf("%T: commands=%d err=%v", command, len(builder.Commands()), builder.Err())
		}
	}
}

func TestBuilderRejectsModifiedSpecialCommands(t *testing.T) {
	commands := []render.Command{
		render.DrawParticles{}, render.DrawSpriteBatch{}, render.DrawTextBatch{},
	}
	for _, command := range commands {
		var builder render.Builder
		builder.Reset(nil)
		builder.Push(render.Group{LayerOffset: 1})
		builder.Add(command)
		var got *render.BuildError
		if !errors.As(builder.Err(), &got) || got.Kind != render.ErrUnsupportedGroupedCommand {
			t.Fatalf("%T: err=%v", command, builder.Err())
		}
	}
}

func TestBuilderReportsStackAndFiniteErrors(t *testing.T) {
	var builder render.Builder
	builder.Reset(nil)
	builder.Pop()
	var got *render.BuildError
	if !errors.As(builder.Err(), &got) || got.Kind != render.ErrStackUnderflow {
		t.Fatalf("underflow err=%v", builder.Err())
	}

	builder.Reset(nil)
	builder.Push(render.Group{})
	builder.Commands()
	if !errors.As(builder.Err(), &got) || got.Kind != render.ErrUnclosedGroup {
		t.Fatalf("unclosed err=%v", builder.Err())
	}

	builder.Reset(nil)
	builder.Push(render.Group{Opacity: math.NaN()})
	if !errors.As(builder.Err(), &got) || got.Kind != render.ErrInvalidGroup {
		t.Fatalf("finite err=%v", builder.Err())
	}
}

func TestBuilderDropsEmptyClipAndReusesDestination(t *testing.T) {
	prefix := []render.Command{render.DrawPoint{}}
	var builder render.Builder
	builder.Reset(prefix)
	builder.Push(render.Group{Clip: math2d.NewRect(math2d.Vec2{X: 5}, math2d.Vec2{X: 1, Y: 1})})
	builder.Add(render.DrawRect{
		BaseCommand: render.BaseCommand{Clip: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1})},
	})
	builder.Pop()
	if got := len(builder.Commands()); got != 1 {
		t.Fatalf("len = %d, want prefix only", got)
	}
}
