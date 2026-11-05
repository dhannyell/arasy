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
		Src:  math2d.NewRect(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 8, Y: 8}),
		Tint: render.RGBA(1, 0, 0, 0.5),
	}
	records := AppendRetainedSpriteRecord(nil, inst, 0, 0, 0, true)
	if got := binary.LittleEndian.Uint32(records[56:]); got != 0xffffffff {
		t.Fatalf("tint = %#x, want forced white for the diagnostic placeholder", got)
	}
	if got := retainedF32(records, 8); got != 2 {
		t.Fatalf("halfWidth = %v, want missing texture to ignore Src", got)
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

func batchTestSizes() TexSizes {
	return TexSizes{{}, {4, 4}, {4, 4}, {8, 8}}
}

func batchInstance(texture render.TextureID, x, y, w, h float64) render.SpriteInstance {
	return render.SpriteInstance{
		Texture: texture,
		Dest:    math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: w, Y: h}),
	}
}

func recordF32(t *testing.T, records []byte, record, offset int) float32 {
	t.Helper()
	return math.Float32frombits(binary.LittleEndian.Uint32(records[record*SpriteStride+offset:]))
}

func recordU32(t *testing.T, records []byte, record, offset int) uint32 {
	t.Helper()
	return binary.LittleEndian.Uint32(records[record*SpriteStride+offset:])
}

func TestPlanSpriteBatchExactRecordLayout(t *testing.T) {
	inst := batchInstance(1, 2, 4, 6, 12) // min (2,4) size (6,12): center (5,10), half (3,6)
	inst.Rotation = 0.5
	plan := PlanSpriteBatch([]render.SpriteInstance{inst}, batchTestSizes(), true)

	if plan.Count != 1 || len(plan.Records) != SpriteStride {
		t.Fatalf("count = %d, records = %d bytes, want 1 record of %d", plan.Count, len(plan.Records), SpriteStride)
	}
	checks := []struct {
		offset int
		want   float32
	}{
		{0, 5}, {4, 10}, {8, 3}, {12, 6}, // center, half-size
		{16, 0.5},                          // rotation
		{24, 0}, {28, 0}, {32, 1}, {36, 1}, // whole-texture UVs
		{40, -math.MaxFloat32}, {44, -math.MaxFloat32}, // always-open clip sentinel
		{48, math.MaxFloat32}, {52, math.MaxFloat32},
	}
	for _, c := range checks {
		if got := recordF32(t, plan.Records, 0, c.offset); got != c.want {
			t.Fatalf("record offset %d = %v, want %v", c.offset, got, c.want)
		}
	}
	if got := recordU32(t, plan.Records, 0, 56); got != 0xffffffff {
		t.Fatalf("tint = %#x, want 0xffffffff (zero tint = white)", got)
	}
	if got := recordU32(t, plan.Records, 0, 60); got != 0 {
		t.Fatalf("layer = %d, want 0 (texture 1 - 1)", got)
	}
	if plan.Animated {
		t.Fatal("zero-value animation must stay on the static pipeline")
	}
	if len(plan.Anims) != 0 {
		t.Fatalf("static batch wrote %d anim bytes, want 0", len(plan.Anims))
	}
}

func TestPlanSpriteBatchArrayPathSingleRange(t *testing.T) {
	instances := []render.SpriteInstance{
		batchInstance(1, 0, 0, 4, 4),
		batchInstance(1, 4, 0, 4, 4),
		batchInstance(2, 8, 0, 4, 4),
		batchInstance(1, 12, 0, 4, 4),
	}
	plan := PlanSpriteBatch(instances, batchTestSizes(), true)

	if len(plan.Ranges) != 1 {
		t.Fatalf("ranges = %d, want 1 (homogeneous array collapses the batch)", len(plan.Ranges))
	}
	if plan.Ranges[0].Start != 0 || plan.Ranges[0].Count != 4 || plan.Ranges[0].Missing {
		t.Fatalf("range = %+v, want {start 0, count 4}", plan.Ranges[0])
	}
	wantLayers := []uint32{0, 0, 1, 0}
	for i, want := range wantLayers {
		if got := recordU32(t, plan.Records, i, 60); got != want {
			t.Fatalf("record %d layer = %d, want %d", i, got, want)
		}
	}
}

func TestPlanSpriteBatchFallbackSplitsConsecutiveTextures(t *testing.T) {
	instances := []render.SpriteInstance{
		batchInstance(1, 0, 0, 4, 4),
		batchInstance(1, 4, 0, 4, 4),
		batchInstance(2, 8, 0, 4, 4),
		batchInstance(1, 12, 0, 4, 4),
	}
	plan := PlanSpriteBatch(instances, batchTestSizes(), false)

	want := []SpriteBatchRange{
		{Texture: 1, Start: 0, Count: 2},
		{Texture: 2, Start: 2, Count: 1},
		{Texture: 1, Start: 3, Count: 1},
	}
	if len(plan.Ranges) != len(want) {
		t.Fatalf("ranges = %+v, want %+v (1→2→1 splits without reordering)", plan.Ranges, want)
	}
	for i, w := range want {
		if plan.Ranges[i] != w {
			t.Fatalf("range %d = %+v, want %+v", i, plan.Ranges[i], w)
		}
	}
	// Fallback records never select an array layer.
	for i := 0; i < plan.Count; i++ {
		if got := recordU32(t, plan.Records, i, 60); got != 0 {
			t.Fatalf("record %d layer = %d, want 0 on the fallback path", i, got)
		}
	}
}

func TestPlanSpriteBatchMissingTexturePlaceholder(t *testing.T) {
	tinted := batchInstance(1, 0, 0, 4, 4)
	tinted.Tint = render.RGBA(1, 0, 0, 0.5)
	missing := batchInstance(99, 4, 0, 4, 4)
	missing.Tint = render.RGBA(1, 0, 0, 0.5)
	plan := PlanSpriteBatch([]render.SpriteInstance{tinted, missing}, batchTestSizes(), false)

	if len(plan.Ranges) != 2 || plan.Ranges[0].Missing || !plan.Ranges[1].Missing {
		t.Fatalf("ranges = %+v, want a texture range followed by a missing-texture range", plan.Ranges)
	}
	if got := recordU32(t, plan.Records, 1, 56); got != 0xffffffff {
		t.Fatalf("missing-texture tint = %#x, want forced white so the placeholder keeps its colors", got)
	}
	if got := recordU32(t, plan.Records, 0, 56); got == 0xffffffff {
		t.Fatal("present-texture tint was forced white; only missing textures may be")
	}
}

func TestPlanSpriteBatchDegenerateSrcZeroesQuad(t *testing.T) {
	inst := batchInstance(1, 0, 0, 4, 4)
	inst.Src = math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 12, Y: 12}) // fully outside 4x4
	plan := PlanSpriteBatch([]render.SpriteInstance{inst}, batchTestSizes(), false)

	if got := recordF32(t, plan.Records, 0, 8); got != 0 {
		t.Fatalf("halfWidth = %v, want 0 (degenerate Src draws nothing, like streaming)", got)
	}
	if got := recordF32(t, plan.Records, 0, 12); got != 0 {
		t.Fatalf("halfHeight = %v, want 0", got)
	}
}

func TestPlanSpriteBatchAnimRecords(t *testing.T) {
	flip := batchInstance(3, 0, 0, 4, 4)
	flip.Src = math2d.NewRect(math2d.Vec2{X: 2, Y: 2}, math2d.Vec2{X: 2, Y: 2}) // col 1, row 1 → base 5 in 8x8 of 2x2 cells
	flip.Animation = render.SpriteAnimation{Frames: 4, Rate: 1, Phase: 0.25, Bounce: 2}

	invalidFlip := batchInstance(1, 8, 0, 4, 4)
	invalidFlip.Src = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 2, Y: 2})
	invalidFlip.Animation = render.SpriteAnimation{Frames: 8, Rate: 1} // 8 frames > 4 cells

	plan := PlanSpriteBatch([]render.SpriteInstance{flip, invalidFlip}, batchTestSizes(), false)
	if !plan.Animated {
		t.Fatal("non-zero animation must select the animated variant")
	}
	if len(plan.Anims) != 2*SpriteAnimStride {
		t.Fatalf("anims = %d bytes, want %d", len(plan.Anims), 2*SpriteAnimStride)
	}

	gotFrames := binary.LittleEndian.Uint32(plan.Anims[0:])
	gotCPR := binary.LittleEndian.Uint32(plan.Anims[4:])
	gotBase := binary.LittleEndian.Uint32(plan.Anims[8:])
	gotRate := math.Float32frombits(binary.LittleEndian.Uint32(plan.Anims[16:]))
	gotPhase := math.Float32frombits(binary.LittleEndian.Uint32(plan.Anims[20:]))
	gotBounce := math.Float32frombits(binary.LittleEndian.Uint32(plan.Anims[24:]))
	if gotFrames != 4 || gotCPR != 4 || gotBase != 5 {
		t.Fatalf("anim record = frames %d cpr %d base %d, want 4/4/5", gotFrames, gotCPR, gotBase)
	}
	if gotRate != 1 || gotPhase != 0.25 || gotBounce != 2 {
		t.Fatalf("anim record = rate %v phase %v bounce %v, want 1/0.25/2", gotRate, gotPhase, gotBounce)
	}

	second := plan.Anims[SpriteAnimStride:]
	if frames := binary.LittleEndian.Uint32(second[0:]); frames != 0 {
		t.Fatalf("invalid flip-book frames = %d, want 0 (static-Src degradation)", frames)
	}
}

func TestPlanSpriteBatchBoundsBounceAndAccounting(t *testing.T) {
	first := batchInstance(1, 0, 0, 4, 4)
	first.Animation = render.SpriteAnimation{Bounce: -3}
	second := batchInstance(1, 20, 30, 4, 4)
	second.Animation = render.SpriteAnimation{Bounce: 2}
	nonFinite := batchInstance(1, math.NaN(), 0, 4, 4)

	plan := PlanSpriteBatch([]render.SpriteInstance{first, second, nonFinite}, batchTestSizes(), false)

	if plan.Count != 2 || plan.NonFinite != 1 {
		t.Fatalf("count = %d nonFinite = %d, want 2/1", plan.Count, plan.NonFinite)
	}
	if len(plan.Records) != plan.Count*SpriteStride {
		t.Fatalf("records = %d bytes, want %d", len(plan.Records), plan.Count*SpriteStride)
	}
	if plan.MaxBounce != 3 {
		t.Fatalf("maxBounce = %v, want 3", plan.MaxBounce)
	}
	if !plan.HasBounds || plan.Bounds.Min != (math2d.Vec2{}) || plan.Bounds.Max != (math2d.Vec2{X: 24, Y: 34}) {
		t.Fatalf("bounds = %+v ok %v, want (0,0)-(24,34)", plan.Bounds, plan.HasBounds)
	}
}

func TestPlanSpriteBatchEmptyReplace(t *testing.T) {
	plan := PlanSpriteBatch(nil, batchTestSizes(), false)
	if plan.Count != 0 || len(plan.Ranges) != 0 || plan.HasBounds || len(plan.Records) != 0 {
		t.Fatalf("empty plan = %+v, want zero content", plan)
	}
}

func TestSpriteBatchInflatedBoundsCulling(t *testing.T) {
	bounds := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})
	anim := render.SpriteBatchAnimation{BounceHz: 1, Intensity: 1}

	inflated, ok := SpriteBatchInflatedBounds(bounds, true, 2, anim)
	if !ok {
		t.Fatal("hasBounds was lost")
	}
	if inflated.Min.Y != -2 || inflated.Max.Y != 12 {
		t.Fatalf("inflated = %+v, want 2 units of vertical inflation", inflated)
	}

	far := math2d.NewRect(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 10, Y: 10})
	if intersects := boundsOverlaps(inflated, far); intersects {
		t.Fatal("far view must cull the inflated batch")
	}

	near := math2d.NewRect(math2d.Vec2{X: 4, Y: 11}, math2d.Vec2{X: 2, Y: 10})
	if !boundsOverlaps(inflated, near) {
		t.Fatal("view reachable only through bounce inflation must stay visible")
	}
	if boundsOverlaps(bounds, near) {
		t.Fatal("test premise: without inflation the near view culls")
	}

	if _, ok := SpriteBatchInflatedBounds(bounds, false, 2, anim); ok {
		t.Fatal("batch without finite geometry must report ok=false")
	}
}

// boundsOverlaps keeps the cull tests independent of which helper the
// renderer composes; the renderer calls renderbatch.IntersectsView directly.
func boundsOverlaps(a, b math2d.Rect) bool {
	return a.Overlaps(b)
}

func TestPlanSpriteTextureArray(t *testing.T) {
	tests := []struct {
		name      string
		sizes     TexSizes
		maxLayers int
		want      SpriteTextureArrayPlan
	}{
		{
			name:      "empty manifest",
			sizes:     TexSizes{{}},
			maxLayers: 256,
			want:      SpriteTextureArrayPlan{Reason: "fewer than two declared textures"},
		},
		{
			name:      "one texture",
			sizes:     TexSizes{{}, {64, 64}},
			maxLayers: 256,
			want:      SpriteTextureArrayPlan{Reason: "fewer than two declared textures"},
		},
		{
			name:      "matching textures",
			sizes:     TexSizes{{}, {64, 64}, {64, 64}, {64, 64}},
			maxLayers: 256,
			want:      SpriteTextureArrayPlan{Enabled: true, Width: 64, Height: 64, Layers: 3},
		},
		{
			name:      "failed decode keeps stable layer indexes",
			sizes:     TexSizes{{}, {64, 64}, {}, {64, 64}},
			maxLayers: 256,
			want:      SpriteTextureArrayPlan{Enabled: true, Width: 64, Height: 64, Layers: 3},
		},
		{
			name:      "different dimensions",
			sizes:     TexSizes{{}, {64, 64}, {128, 64}},
			maxLayers: 256,
			want:      SpriteTextureArrayPlan{Reason: "texture dimensions differ"},
		},
		{
			name:      "device layer limit",
			sizes:     TexSizes{{}, {64, 64}, {64, 64}, {64, 64}},
			maxLayers: 2,
			want:      SpriteTextureArrayPlan{Reason: "texture count exceeds device layer limit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlanSpriteTextureArray(tt.sizes, tt.maxLayers); got != tt.want {
				t.Fatalf("PlanSpriteTextureArray() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlanSpriteBatchGroupsMissingTexturesOnFallback(t *testing.T) {
	plan := PlanSpriteBatch([]render.SpriteInstance{
		batchInstance(7, 0, 0, 4, 4),
		batchInstance(8, 0, 0, 4, 4),
		batchInstance(1, 0, 0, 4, 4),
	}, batchTestSizes(), false)

	if len(plan.Ranges) != 2 {
		t.Fatalf("ranges = %+v, want one fallback range followed by one texture range", plan.Ranges)
	}
	if got := plan.Ranges[0]; !got.Missing || got.Texture != 0 || got.Start != 0 || got.Count != 2 {
		t.Fatalf("fallback range = %+v, want two missing sprites", got)
	}
	if got := plan.Ranges[1]; got.Missing || got.Texture != 1 || got.Start != 2 || got.Count != 1 {
		t.Fatalf("texture range = %+v, want declared texture 1", got)
	}
}
