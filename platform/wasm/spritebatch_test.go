package wasm

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestUniformArenaDistinctAlignedSlots(t *testing.T) {
	arena := newUniformArena(spriteBatchAnimUniformSize, 256, 2)
	if arena.slotSize != 256 {
		t.Fatalf("slotSize = %d, want 256 (96 aligned up to 256)", arena.slotSize)
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

func TestUniformArenaSlotCapacityGrowsBeforeFrameWrites(t *testing.T) {
	tests := []struct {
		name              string
		current, required int
		want              int
	}{
		{"keeps capacity", 64, 64, 64},
		{"grows past initial capacity", 64, 65, 128},
		{"grows more than once", 64, 129, 256},
		{"never shrinks", 128, 1, 128},
		{"initializes empty arena", 0, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniformArenaSlotCapacity(tt.current, tt.required); got != tt.want {
				t.Fatalf("uniformArenaSlotCapacity(%d, %d) = %d, want %d", tt.current, tt.required, got, tt.want)
			}
		})
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
	tess.PackSpriteBatchAnimUniforms(slot[:], camera, 16, 16, anim)

	var cameraOnly [tess.SpriteUniformSize]byte
	tess.PackSpriteUniforms(cameraOnly[:], camera, 16, 16)
	if string(slot[:tess.SpriteUniformSize]) != string(cameraOnly[:]) {
		t.Fatal("first 48 bytes must be the shared camera uniform")
	}
	fields := []struct {
		offset int
		want   float32
	}{
		{48, 0.5}, // wrapped time
		{52, 1},   // bounce Hz
		{56, 1},   // intensity
		{60, 0},   // pad
		{64, 17},  // pulse center X
		{68, 47},  // pulse center Y
		{72, 2},   // pulse radius
		{76, 4},   // pulse width
		{80, 1},   // pulse boost
		{84, 0},   // pad
		{88, 0},   // pad
		{92, 0},   // pad
	}
	for _, f := range fields {
		got := math.Float32frombits(binary.LittleEndian.Uint32(slot[f.offset:]))
		if got != f.want {
			t.Fatalf("uniform offset %d = %v, want %v", f.offset, got, f.want)
		}
	}
}
