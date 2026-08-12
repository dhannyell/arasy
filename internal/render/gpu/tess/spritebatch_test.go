package tess

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func retainedF32(records []byte, offset int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(records[offset:]))
}

func TestAppendRetainedSpriteRecordLayout(t *testing.T) {
	inst := render.SpriteInstance{
		Texture:  1,
		Dest:     math2d.NewRect(math2d.Vec2{X: 2, Y: 4}, math2d.Vec2{X: 6, Y: 12}),
		Src:      math2d.NewRect(math2d.Vec2{X: 2, Y: 0}, math2d.Vec2{X: 2, Y: 2}),
		Rotation: 0.25,
		Tint:     render.RGBA(1, 0, 0, 1),
	}
	records := AppendRetainedSpriteRecord(nil, inst, 4, 4, 2, false)
	if len(records) != SpriteStride {
		t.Fatalf("records = %d bytes, want %d", len(records), SpriteStride)
	}

	if got := retainedF32(records, 0); got != 5 {
		t.Fatalf("centerX = %v, want 5", got)
	}
	if got := retainedF32(records, 16); got != 0.25 {
		t.Fatalf("rotation = %v, want 0.25", got)
	}
	if got := binary.LittleEndian.Uint32(records[20:]); got != 0 {
		t.Fatalf("flags = %d, want zero for a billboard sprite", got)
	}
	// Src (2,0)-(4,2) in a 4x4 texture → uv (0.5,0)-(1,0.5).
	if got := retainedF32(records, 24); got != 0.5 {
		t.Fatalf("uvMinX = %v, want 0.5", got)
	}
	if got := retainedF32(records, 36); got != 0.5 {
		t.Fatalf("uvMaxY = %v, want 0.5", got)
	}
	// Always-open clip sentinel: retained clips are draw-time scissors.
	if got := retainedF32(records, 40); got != -math.MaxFloat32 {
		t.Fatalf("clipMinX = %v, want -MaxFloat32", got)
	}
	if got := retainedF32(records, 48); got != math.MaxFloat32 {
		t.Fatalf("clipMaxX = %v, want MaxFloat32", got)
	}
	if got := binary.LittleEndian.Uint32(records[56:]); got != 0xff0000ff {
		t.Fatalf("tint = %#x, want 0xff0000ff (packed red)", got)
	}
	if got := binary.LittleEndian.Uint32(records[60:]); got != 2 {
		t.Fatalf("layer = %d, want 2", got)
	}
}

func TestAppendRetainedSpriteRecordProjectsCorners(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:           math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 8, Y: 6}),
		ProjectCorners: true,
	}
	records := AppendRetainedSpriteRecord(nil, inst, 8, 6, 0, false)
	if got := binary.LittleEndian.Uint32(records[20:]); got != 1 {
		t.Fatalf("flags = %d, want projected-corners flag", got)
	}
}

func TestAppendRetainedSpriteRecordWritesFlipFlags(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:           math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 8, Y: 6}),
		ProjectCorners: true,
		Flip:           render.FlipX | render.FlipY,
	}
	records := AppendRetainedSpriteRecord(nil, inst, 8, 6, 0, false)
	if got := binary.LittleEndian.Uint32(records[20:]); got != projectedSpriteFlag|flipXSpriteFlag|flipYSpriteFlag {
		t.Fatalf("flags = %d, want projected and both flip flags", got)
	}
}

func TestAppendRetainedSpriteRecordMissingTextureWhiteTint(t *testing.T) {
	inst := render.SpriteInstance{
		Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		Tint: render.RGBA(1, 0, 0, 0.5),
	}
	records := AppendRetainedSpriteRecord(nil, inst, 0, 0, 0, true)
	if got := binary.LittleEndian.Uint32(records[56:]); got != 0xffffffff {
		t.Fatalf("tint = %#x, want forced white for the magenta placeholder", got)
	}
}

func TestAppendRetainedSpriteRecordDegenerateSrcZeroesQuad(t *testing.T) {
	inst := render.SpriteInstance{
		Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4}),
		Src:  math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 2, Y: 2}),
	}
	records := AppendRetainedSpriteRecord(nil, inst, 4, 4, 0, false)
	if got := retainedF32(records, 8); got != 0 {
		t.Fatalf("halfWidth = %v, want 0 (degenerate Src draws nothing)", got)
	}
	if got := retainedF32(records, 12); got != 0 {
		t.Fatalf("halfHeight = %v, want 0", got)
	}
}

// TestStreamRunSpriteBatchBreaksMerge: a retained batch command is its own
// run and a painter-order boundary, like particles — sprite streams on
// either side must not merge across it.
func TestStreamRunSpriteBatchBreaksMerge(t *testing.T) {
	sizes := TexSizes{{}, {4, 4}}
	frame := Frame{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 64, Y: 64}),
		},
		Commands: []render.Command{
			render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})},
			render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: 1}, Batch: 7},
			render.DrawImage{BaseCommand: render.BaseCommand{LayerValue: 2}, Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 8, Y: 8}, math2d.Vec2{X: 4, Y: 4})},
		},
	}

	var builder Builder
	runs := builder.Build(frame, 64, 64, NewAtlas(), sizes)
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3 (sprite, batch, sprite)", len(runs))
	}
	if runs[0].Kind != DrawRunSprites || runs[1].Kind != DrawRunSpriteBatch || runs[2].Kind != DrawRunSprites {
		t.Fatalf("kinds = %v/%v/%v, want sprites/batch/sprites", runs[0].Kind, runs[1].Kind, runs[2].Kind)
	}
	if runs[1].SpriteBatch.Batch != 7 {
		t.Fatalf("run carries batch %d, want 7", runs[1].SpriteBatch.Batch)
	}
	// The batch run carries no instance payload: content lives in the
	// backend's retained registry, so the sprite stream must not grow for it.
	if got := len(builder.SpriteStream()) / SpriteStride; got != 2 {
		t.Fatalf("sprite stream = %d records, want 2 (only the streaming sprites)", got)
	}
}
