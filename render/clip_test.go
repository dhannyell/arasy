package render_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestClipCommandsSetsClipOnUnclippedCommands(t *testing.T) {
	clip := math2d.NewRect(math2d.Vec2{X: 1, Y: 2}, math2d.Vec2{X: 3, Y: 4})
	cmds := []render.Command{
		render.DrawRect{},
		render.DrawText{Text: "hi"},
	}

	render.ClipCommands(cmds, clip)

	if got := cmds[0].(render.DrawRect).Clip; got != clip {
		t.Fatalf("DrawRect.Clip = %v, want %v", got, clip)
	}
	if got := cmds[1].(render.DrawText).Clip; got != clip {
		t.Fatalf("DrawText.Clip = %v, want %v", got, clip)
	}
}

func TestClipCommandsDoesNotOverrideExistingClip(t *testing.T) {
	own := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 5, Y: 5})
	cmds := []render.Command{
		render.DrawRect{BaseCommand: render.BaseCommand{Clip: own}},
	}

	render.ClipCommands(cmds, math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1}))

	if got := cmds[0].(render.DrawRect).Clip; got != own {
		t.Fatalf("Clip = %v, want unchanged %v", got, own)
	}
}
