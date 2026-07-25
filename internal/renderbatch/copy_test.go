package renderbatch_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestCopySpritesIsDefensive(t *testing.T) {
	input := []render.SpriteInstance{
		{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 1, Y: 2}, math2d.Vec2{X: 3, Y: 4})},
		{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 5, Y: 6}, math2d.Vec2{X: 7, Y: 8})},
	}

	stored := renderbatch.CopySprites(input)

	// Mutating the caller's slice after the copy must not affect the stored
	// copy.
	input[0].Texture = 99
	input[0].Dest.Min.X = -100
	input[1] = render.SpriteInstance{}

	if stored[0].Texture != 1 || stored[0].Dest.Min.X != 1 {
		t.Fatalf("stored[0] = %+v, want the original first instance", stored[0])
	}
	if stored[1].Texture != 2 {
		t.Fatalf("stored[1] = %+v, want the original second instance", stored[1])
	}
}

func TestCopySpritesEmptyIsNil(t *testing.T) {
	if got := renderbatch.CopySprites(nil); got != nil {
		t.Fatalf("CopySprites(nil) = %v, want nil", got)
	}
	if got := renderbatch.CopySprites([]render.SpriteInstance{}); got != nil {
		t.Fatalf("CopySprites(empty) = %v, want nil", got)
	}
}

func TestCopyLabelsIsDefensive(t *testing.T) {
	input := []render.TextLabel{
		{Text: "score", Position: math2d.Vec2{X: 10, Y: 20}, Size: 16},
		{Text: "lives", Position: math2d.Vec2{X: 30, Y: 40}, Size: 12},
	}

	stored := renderbatch.CopyLabels(input)

	input[0].Text = "mutated"
	input[1] = render.TextLabel{}

	if stored[0].Text != "score" {
		t.Fatalf("stored[0] = %+v, want the original first label", stored[0])
	}
	if stored[1].Text != "lives" {
		t.Fatalf("stored[1] = %+v, want the original second label", stored[1])
	}
}

func TestCopyLabelsEmptyIsNil(t *testing.T) {
	if got := renderbatch.CopyLabels(nil); got != nil {
		t.Fatalf("CopyLabels(nil) = %v, want nil", got)
	}
	if got := renderbatch.CopyLabels([]render.TextLabel{}); got != nil {
		t.Fatalf("CopyLabels(empty) = %v, want nil", got)
	}
}
