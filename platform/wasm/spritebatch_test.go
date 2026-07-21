package wasm

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/wasm/tess"
	"github.com/dhannyell/frameforge/render"
)

func batchTestSizes() tess.TexSizes {
	return tess.TexSizes{{}, {4, 4}, {4, 4}, {8, 8}}
}

func batchInstance(texture render.TextureID, x, y, w, h float64) render.SpriteInstance {
	return render.SpriteInstance{
		Texture: texture,
		Dest:    math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: w, Y: h}),
	}
}

func recordF32(t *testing.T, records []byte, record, offset int) float32 {
	t.Helper()
	return math.Float32frombits(binary.LittleEndian.Uint32(records[record*tess.SpriteStride+offset:]))
}

func recordU32(t *testing.T, records []byte, record, offset int) uint32 {
	t.Helper()
	return binary.LittleEndian.Uint32(records[record*tess.SpriteStride+offset:])
}

func TestPlanSpriteBatchExactRecordLayout(t *testing.T) {
	inst := batchInstance(1, 2, 4, 6, 12) // min (2,4) size (6,12): center (5,10), half (3,6)
	inst.Rotation = 0.5
	plan := planSpriteBatch([]render.SpriteInstance{inst}, batchTestSizes(), true)

	if plan.count != 1 || len(plan.records) != tess.SpriteStride {
		t.Fatalf("count = %d, records = %d bytes, want 1 record of %d", plan.count, len(plan.records), tess.SpriteStride)
	}
	checks := []struct {
		offset int
		want   float32
	}{
		{0, 5}, {4, 10}, {8, 3}, {12, 6}, // center, half-size
		{16, 0.5}, // rotation
		{24, 0}, {28, 0}, {32, 1}, {36, 1}, // whole-texture UVs
		{40, -math.MaxFloat32}, {44, -math.MaxFloat32}, // always-open clip sentinel
		{48, math.MaxFloat32}, {52, math.MaxFloat32},
	}
	for _, c := range checks {
		if got := recordF32(t, plan.records, 0, c.offset); got != c.want {
			t.Fatalf("record offset %d = %v, want %v", c.offset, got, c.want)
		}
	}
	if got := recordU32(t, plan.records, 0, 56); got != 0xffffffff {
		t.Fatalf("tint = %#x, want 0xffffffff (zero tint = white)", got)
	}
	if got := recordU32(t, plan.records, 0, 60); got != 0 {
		t.Fatalf("layer = %d, want 0 (texture 1 - 1)", got)
	}
	if plan.animated {
		t.Fatal("zero-value animation must stay on the static pipeline")
	}
	if len(plan.anims) != 0 {
		t.Fatalf("static batch wrote %d anim bytes, want 0", len(plan.anims))
	}
}

func TestPlanSpriteBatchArrayPathSingleRange(t *testing.T) {
	instances := []render.SpriteInstance{
		batchInstance(1, 0, 0, 4, 4),
		batchInstance(1, 4, 0, 4, 4),
		batchInstance(2, 8, 0, 4, 4),
		batchInstance(1, 12, 0, 4, 4),
	}
	plan := planSpriteBatch(instances, batchTestSizes(), true)

	if len(plan.ranges) != 1 {
		t.Fatalf("ranges = %d, want 1 (homogeneous array collapses the batch)", len(plan.ranges))
	}
	if plan.ranges[0].start != 0 || plan.ranges[0].count != 4 || plan.ranges[0].magenta {
		t.Fatalf("range = %+v, want {start 0, count 4}", plan.ranges[0])
	}
	wantLayers := []uint32{0, 0, 1, 0}
	for i, want := range wantLayers {
		if got := recordU32(t, plan.records, i, 60); got != want {
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
	plan := planSpriteBatch(instances, batchTestSizes(), false)

	want := []spriteBatchRange{
		{texture: 1, start: 0, count: 2},
		{texture: 2, start: 2, count: 1},
		{texture: 1, start: 3, count: 1},
	}
	if len(plan.ranges) != len(want) {
		t.Fatalf("ranges = %+v, want %+v (1→2→1 splits without reordering)", plan.ranges, want)
	}
	for i, w := range want {
		if plan.ranges[i] != w {
			t.Fatalf("range %d = %+v, want %+v", i, plan.ranges[i], w)
		}
	}
	// Fallback records never select an array layer.
	for i := 0; i < plan.count; i++ {
		if got := recordU32(t, plan.records, i, 60); got != 0 {
			t.Fatalf("record %d layer = %d, want 0 on the fallback path", i, got)
		}
	}
}

func TestPlanSpriteBatchMissingTexturePlaceholder(t *testing.T) {
	tinted := batchInstance(1, 0, 0, 4, 4)
	tinted.Tint = render.RGBA(1, 0, 0, 0.5)
	missing := batchInstance(99, 4, 0, 4, 4)
	missing.Tint = render.RGBA(1, 0, 0, 0.5)
	plan := planSpriteBatch([]render.SpriteInstance{tinted, missing}, batchTestSizes(), false)

	if len(plan.ranges) != 2 || plan.ranges[0].magenta || !plan.ranges[1].magenta {
		t.Fatalf("ranges = %+v, want a texture range followed by a magenta range", plan.ranges)
	}
	if got := recordU32(t, plan.records, 1, 56); got != 0xffffffff {
		t.Fatalf("missing-texture tint = %#x, want forced white so the placeholder stays magenta", got)
	}
	if got := recordU32(t, plan.records, 0, 56); got == 0xffffffff {
		t.Fatal("present-texture tint was forced white; only missing textures may be")
	}
}

func TestPlanSpriteBatchDegenerateSrcZeroesQuad(t *testing.T) {
	inst := batchInstance(1, 0, 0, 4, 4)
	inst.Src = math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 12, Y: 12}) // fully outside 4x4
	plan := planSpriteBatch([]render.SpriteInstance{inst}, batchTestSizes(), false)

	if got := recordF32(t, plan.records, 0, 8); got != 0 {
		t.Fatalf("halfWidth = %v, want 0 (degenerate Src draws nothing, like streaming)", got)
	}
	if got := recordF32(t, plan.records, 0, 12); got != 0 {
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

	plan := planSpriteBatch([]render.SpriteInstance{flip, invalidFlip}, batchTestSizes(), false)
	if !plan.animated {
		t.Fatal("non-zero animation must select the animated variant")
	}
	if len(plan.anims) != 2*spriteBatchAnimStride {
		t.Fatalf("anims = %d bytes, want %d", len(plan.anims), 2*spriteBatchAnimStride)
	}

	gotFrames := binary.LittleEndian.Uint32(plan.anims[0:])
	gotCPR := binary.LittleEndian.Uint32(plan.anims[4:])
	gotBase := binary.LittleEndian.Uint32(plan.anims[8:])
	gotRate := math.Float32frombits(binary.LittleEndian.Uint32(plan.anims[16:]))
	gotPhase := math.Float32frombits(binary.LittleEndian.Uint32(plan.anims[20:]))
	gotBounce := math.Float32frombits(binary.LittleEndian.Uint32(plan.anims[24:]))
	if gotFrames != 4 || gotCPR != 4 || gotBase != 5 {
		t.Fatalf("anim record = frames %d cpr %d base %d, want 4/4/5", gotFrames, gotCPR, gotBase)
	}
	if gotRate != 1 || gotPhase != 0.25 || gotBounce != 2 {
		t.Fatalf("anim record = rate %v phase %v bounce %v, want 1/0.25/2", gotRate, gotPhase, gotBounce)
	}

	second := plan.anims[spriteBatchAnimStride:]
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

	plan := planSpriteBatch([]render.SpriteInstance{first, second, nonFinite}, batchTestSizes(), false)

	if plan.count != 2 || plan.nonFinite != 1 {
		t.Fatalf("count = %d nonFinite = %d, want 2/1", plan.count, plan.nonFinite)
	}
	if len(plan.records) != plan.count*tess.SpriteStride {
		t.Fatalf("records = %d bytes, want %d", len(plan.records), plan.count*tess.SpriteStride)
	}
	if plan.maxBounce != 3 {
		t.Fatalf("maxBounce = %v, want 3", plan.maxBounce)
	}
	if !plan.hasBounds || plan.bounds.Min != (math2d.Vec2{}) || plan.bounds.Max != (math2d.Vec2{X: 24, Y: 34}) {
		t.Fatalf("bounds = %+v ok %v, want (0,0)-(24,34)", plan.bounds, plan.hasBounds)
	}
}

func TestPlanSpriteBatchEmptyReplace(t *testing.T) {
	plan := planSpriteBatch(nil, batchTestSizes(), false)
	if plan.count != 0 || len(plan.ranges) != 0 || plan.hasBounds || len(plan.records) != 0 {
		t.Fatalf("empty plan = %+v, want zero content", plan)
	}
}

func TestSpriteBatchInflatedBoundsCulling(t *testing.T) {
	bounds := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})
	anim := render.SpriteBatchAnimation{BounceHz: 1, Intensity: 1}

	inflated, ok := spriteBatchInflatedBounds(bounds, true, 2, anim)
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

	if _, ok := spriteBatchInflatedBounds(bounds, false, 2, anim); ok {
		t.Fatal("batch without finite geometry must report ok=false")
	}
}

// boundsOverlaps keeps the cull tests independent of which helper the
// renderer composes; the renderer calls renderbatch.IntersectsView directly.
func boundsOverlaps(a, b math2d.Rect) bool {
	return a.Overlaps(b)
}

func TestUniformArenaDistinctAlignedSlots(t *testing.T) {
	arena := newUniformArena(spriteBatchAnimUniformSize, 256, 2)
	if arena.slotSize != 256 {
		t.Fatalf("slotSize = %d, want 256 (80 aligned up to 256)", arena.slotSize)
	}

	first, ok := arena.alloc()
	if !ok || first != 0 {
		t.Fatalf("first alloc = (%d, %v), want (0, true)", first, ok)
	}
	second, ok := arena.alloc()
	if !ok || second != 256 {
		t.Fatalf("second alloc = (%d, %v), want (256, true) — two draws of one batch get distinct slots", second, ok)
	}
	if _, ok := arena.alloc(); ok {
		t.Fatal("third alloc beyond capacity must fail so the caller grows the arena")
	}

	arena.reset()
	again, ok := arena.alloc()
	if !ok || again != 0 {
		t.Fatalf("alloc after reset = (%d, %v), want (0, true)", again, ok)
	}
}

func TestUniformArenaAlignmentRoundsUp(t *testing.T) {
	arena := newUniformArena(80, 256, 1)
	if arena.slotSize != 256 {
		t.Fatalf("slotSize = %d, want 256", arena.slotSize)
	}
	arena = newUniformArena(80, 16, 1)
	if arena.slotSize != 80 {
		t.Fatalf("slotSize = %d, want 80 (already 16-aligned)", arena.slotSize)
	}
}

func TestPackSpriteBatchAnimUniformsLayout(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 16, Y: 16}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 16, Y: 16}),
	}
	anim := render.SpriteBatchAnimation{
		Time:      4096.5, // wraps to 0.5
		BounceHz:  1,
		Intensity: 1,
		Pulse: render.SpritePulse{
			Center: math2d.Vec2{X: 17, Y: 47}, Radius: 2, Width: 4, Boost: 1,
		},
	}

	var slot [spriteBatchAnimUniformSize]byte
	packSpriteBatchAnimUniforms(slot[:], camera, 16, 16, anim)

	var cameraOnly [spriteUniformSize]byte
	packSpriteUniforms(cameraOnly[:], camera, 16, 16)
	if string(slot[:spriteUniformSize]) != string(cameraOnly[:]) {
		t.Fatal("first 32 bytes must be the shared camera uniform")
	}

	fields := []struct {
		offset int
		want   float32
	}{
		{32, 0.5},  // wrapped time
		{36, 1},    // bounce Hz
		{40, 1},    // intensity
		{44, 0},    // pad
		{48, 17},   // pulse center X
		{52, 47},   // pulse center Y
		{56, 2},    // pulse radius
		{60, 4},    // pulse width
		{64, 1},    // pulse boost
		{68, 0},    // pad
		{72, 0},    // pad
		{76, 0},    // pad
	}
	for _, f := range fields {
		got := math.Float32frombits(binary.LittleEndian.Uint32(slot[f.offset:]))
		if got != f.want {
			t.Fatalf("uniform offset %d = %v, want %v", f.offset, got, f.want)
		}
	}
}
