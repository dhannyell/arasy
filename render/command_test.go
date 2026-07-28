package render_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestBaseCommandLayerReturnsConfiguredLayer(t *testing.T) {
	command := render.BaseCommand{LayerValue: 7}

	if got := command.Layer(); got != 7 {
		t.Fatalf("Layer() = %d, want 7", got)
	}
}

func TestDrawCommandsImplementCommand(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{},
		render.DrawCircle{},
		render.DrawEllipse{},
		render.DrawPoint{},
		render.DrawLine{},
		render.DrawText{},
		render.DrawArc{},
		render.DrawPolygon{},
		render.DrawTriangles{},
		render.DrawImage{},
		render.DrawImageQuad{},
	}

	for _, command := range commands {
		if got := command.Layer(); got != 0 {
			t.Fatalf("%T default layer = %d, want 0", command, got)
		}
	}
}

func TestDrawTextHasExpectedZeroValue(t *testing.T) {
	command := render.DrawText{}

	if command.Font != 0 {
		t.Fatalf("Font = %d, want built-in font 0", command.Font)
	}
}

func TestDrawImageHasExpectedZeroValue(t *testing.T) {
	command := render.DrawImage{}

	if got := command.Src; got != (math2d.Rect{}) {
		t.Fatalf("Src = %#v, want zero value", got)
	}
	if got := command.Tint; got != (render.Color{}) {
		t.Fatalf("Tint = %#v, want zero value", got)
	}
}

func TestDrawImageQuadHasExpectedZeroValueAndLayer(t *testing.T) {
	command := render.DrawImageQuad{BaseCommand: render.BaseCommand{LayerValue: 9}}

	if got := command.Layer(); got != 9 {
		t.Fatalf("Layer() = %d, want 9", got)
	}
	if got := command.Src; got != (math2d.Rect{}) {
		t.Fatalf("Src = %#v, want zero value", got)
	}
	if got := command.Tint; got != (render.Color{}) {
		t.Fatalf("Tint = %#v, want zero value", got)
	}
}

func TestDrawImageUsesBaseCommandLayer(t *testing.T) {
	command := render.DrawImage{BaseCommand: render.BaseCommand{LayerValue: 9}}

	if got := command.Layer(); got != 9 {
		t.Fatalf("Layer() = %d, want 9", got)
	}
}

func TestBaseCommandTransformDefaultIsIdentity(t *testing.T) {
	bc := render.BaseCommand{}
	if !bc.Transform.IsIdentity() {
		t.Fatalf("default Transform should be identity, got %+v", bc.Transform)
	}
}

func TestBaseCommandTransformStoresMatrix(t *testing.T) {
	bc := render.BaseCommand{Transform: math2d.Rotate(1)}

	if bc.Transform.IsIdentity() {
		t.Fatal("Rotate(1) should not be identity")
	}

	p := math2d.Vec2{X: 1, Y: 0}
	got := bc.Transform.Apply(p)
	want := math2d.Rotate(1).Apply(p)
	if got != want {
		t.Fatalf("Transform.Apply(%v) = %v, want %v", p, got, want)
	}
}

func TestAllCommandsCanCarryTransform(t *testing.T) {
	transform := math2d.Translate(math2d.Vec2{X: 10, Y: 20})

	commands := []render.Command{
		render.DrawRect{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawCircle{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawEllipse{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawPoint{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawLine{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawText{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawArc{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawPolygon{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawTriangles{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawImage{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawImageQuad{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawParticles{BaseCommand: render.BaseCommand{Transform: transform}},
	}

	for _, cmd := range commands {
		if cmd.Layer() != 0 {
			t.Fatalf("%T default layer should be 0", cmd)
		}
	}
}
