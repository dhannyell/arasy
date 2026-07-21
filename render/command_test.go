package render_test

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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
