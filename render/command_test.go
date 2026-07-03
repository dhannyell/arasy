package render_test

import (
	"testing"

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
		render.DrawPoint{},
		render.DrawLine{},
		render.DrawText{},
		render.DrawArc{},
	}

	for _, command := range commands {
		if got := command.Layer(); got != 0 {
			t.Fatalf("%T default layer = %d, want 0", command, got)
		}
	}
}
