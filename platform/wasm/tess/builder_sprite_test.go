package tess

import (
	"encoding/binary"
	"math"
	"testing"
	"unsafe"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

type decodedSprite struct {
	center, halfSize [2]float32
	rotation         float32
	uvMin, uvMax     [2]float32
	clipMin, clipMax [2]float32
	tint             uint32
	textureLayer     uint32
}

func decodeSprite(t *testing.T, stream []byte, index int) decodedSprite {
	t.Helper()
	offset := index * SpriteStride
	if len(stream) < offset+SpriteStride {
		t.Fatalf("sprite stream length = %d, cannot decode record %d", len(stream), index)
	}
	readVec := func(at int) [2]float32 {
		return [2]float32{
			math.Float32frombits(binary.LittleEndian.Uint32(stream[offset+at:])),
			math.Float32frombits(binary.LittleEndian.Uint32(stream[offset+at+4:])),
		}
	}
	return decodedSprite{
		center:       readVec(0),
		halfSize:     readVec(8),
		rotation:     math.Float32frombits(binary.LittleEndian.Uint32(stream[offset+16:])),
		uvMin:        readVec(24),
		uvMax:        readVec(32),
		clipMin:      readVec(40),
		clipMax:      readVec(48),
		tint:         binary.LittleEndian.Uint32(stream[offset+56:]),
		textureLayer: binary.LittleEndian.Uint32(stream[offset+60:]),
	}
}

func assertSpriteVec(t *testing.T, got [2]float32, wantX, wantY float32) {
	t.Helper()
	assertFloat32(t, got[0], wantX)
	assertFloat32(t, got[1], wantY)
}

func TestBuilderWritesSpriteRecordsAndSplitsTextureRuns(t *testing.T) {
	first := render.DrawImage{
		BaseCommand: render.BaseCommand{Clip: math2d.NewRect(math2d.Vec2{X: 12, Y: 24}, math2d.Vec2{X: 16, Y: 20})},
		Texture:     1,
		Dest:        math2d.NewRect(math2d.Vec2{X: 10, Y: 20}, math2d.Vec2{X: 20, Y: 30}),
		Src:         math2d.NewRect(math2d.Vec2{X: 16, Y: 8}, math2d.Vec2{X: 32, Y: 16}),
		Tint:        render.RGBA(0.5, 0.25, 1, 0.5),
		Rotation:    math.Pi / 2,
	}
	second := render.DrawImage{
		Texture: 1,
		Dest:    math2d.NewRect(math2d.Vec2{X: 40, Y: 50}, math2d.Vec2{X: 10, Y: 20}),
	}
	third := render.DrawImage{
		Texture: 2,
		Dest:    math2d.NewRect(math2d.Vec2{X: 70, Y: 10}, math2d.Vec2{X: 12, Y: 14}),
	}

	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera:   testCamera(),
		Commands: []render.Command{first, second, third},
	}, 100, 100, nil, TexSizes{{}, {64, 32}, {64, 32}})

	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if runs[0].Kind != DrawRunSprites || runs[0].Texture != 1 || runs[0].ByteOffset != 0 || runs[0].VertexCount != 2 {
		t.Fatalf("runs[0] = %+v, want two texture-1 sprites at offset 0", runs[0])
	}
	if runs[1].Kind != DrawRunSprites || runs[1].Texture != 2 || runs[1].ByteOffset != 2*SpriteStride || runs[1].VertexCount != 1 {
		t.Fatalf("runs[1] = %+v, want one texture-2 sprite after the first two records", runs[1])
	}

	stream := builder.SpriteStream()
	if len(stream) != 3*SpriteStride {
		t.Fatalf("len(SpriteStream) = %d, want %d", len(stream), 3*SpriteStride)
	}

	got := decodeSprite(t, stream, 0)
	assertSpriteVec(t, got.center, 20, 35)
	assertSpriteVec(t, got.halfSize, 10, 15)
	assertFloat32(t, got.rotation, float32(math.Pi/2))
	assertSpriteVec(t, got.uvMin, 0.25, 0.25)
	assertSpriteVec(t, got.uvMax, 0.75, 0.75)
	assertSpriteVec(t, got.clipMin, 12, 24)
	assertSpriteVec(t, got.clipMax, 28, 44)
	if want := uint32(0x80802040); got.tint != want {
		t.Fatalf("first tint = %#08x, want %#08x", got.tint, want)
	}
	if got.textureLayer != 0 {
		t.Fatalf("first texture layer = %d, want 0", got.textureLayer)
	}

	got = decodeSprite(t, stream, 1)
	assertSpriteVec(t, got.center, 45, 60)
	assertSpriteVec(t, got.halfSize, 5, 10)
	assertFloat32(t, got.rotation, 0)
	assertSpriteVec(t, got.uvMin, 0, 0)
	assertSpriteVec(t, got.uvMax, 1, 1)
	assertSpriteVec(t, got.clipMin, 0, 0)
	assertSpriteVec(t, got.clipMax, 100, 100)
	if want := uint32(0xffffffff); got.tint != want {
		t.Fatalf("second tint = %#08x, want %#08x", got.tint, want)
	}
	if got.textureLayer != 0 {
		t.Fatalf("second texture layer = %d, want 0", got.textureLayer)
	}
}

func TestSpriteRecordMatchesWGSLStride(t *testing.T) {
	if got := unsafe.Sizeof(spriteRecord{}); got != SpriteStride {
		t.Fatalf("sprite record size = %d, want %d", got, SpriteStride)
	}
}

func TestBuilderSpriteArrayKeepsInterleavedTexturesInOneRun(t *testing.T) {
	commands := []render.Command{
		render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})},
		render.DrawImage{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 10}, math2d.Vec2{X: 10, Y: 10})},
		render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 20}, math2d.Vec2{X: 10, Y: 10})},
	}
	var builder Builder
	builder.SetSpriteArray(true)
	runs := builder.Build(runtime.Frame{Camera: testCamera(), Commands: commands}, 100, 100, nil, TexSizes{{}, {64, 64}, {64, 64}})
	if len(runs) != 1 || runs[0].Kind != DrawRunSprites || runs[0].VertexCount != 3 {
		t.Fatalf("runs = %+v, want one three-sprite run", runs)
	}
	stream := builder.SpriteStream()
	for index, wantLayer := range []uint32{0, 1, 0} {
		if got := decodeSprite(t, stream, index).textureLayer; got != wantLayer {
			t.Fatalf("sprite %d texture layer = %d, want %d", index, got, wantLayer)
		}
	}
}

func TestBuilderFallbackSplitsInterleavedTexturesIntoPainterOrderedRuns(t *testing.T) {
	commands := []render.Command{
		render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})},
		render.DrawImage{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 10}, math2d.Vec2{X: 10, Y: 10})},
		render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 20}, math2d.Vec2{X: 10, Y: 10})},
	}
	var builder Builder
	runs := builder.Build(runtime.Frame{Camera: testCamera(), Commands: commands}, 100, 100, nil, TexSizes{{}, {64, 32}, {32, 64}})
	if len(runs) != 3 {
		t.Fatalf("len(runs) = %d, want three fallback texture runs: %+v", len(runs), runs)
	}
	for i, wantTexture := range []render.TextureID{1, 2, 1} {
		if runs[i].Kind != DrawRunSprites || runs[i].Texture != wantTexture || runs[i].ByteOffset != i*SpriteStride || runs[i].VertexCount != 1 {
			t.Fatalf("runs[%d] = %+v, want one texture-%d sprite at offset %d", i, runs[i], wantTexture, i*SpriteStride)
		}
	}
}

func TestBuilderKeepsMissingTextureOnSolidPath(t *testing.T) {
	known := render.DrawImage{Texture: 1, Dest: math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 20, Y: 20})}
	missing := render.DrawImage{Texture: 2, Dest: math2d.NewRect(math2d.Vec2{X: 40, Y: 40}, math2d.Vec2{X: 20, Y: 20})}

	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera:   testCamera(),
		Commands: []render.Command{known, missing},
	}, 100, 100, nil, TexSizes{{}, {64, 64}})

	if len(runs) != 2 || runs[0].Kind != DrawRunSprites || runs[1].Kind != DrawRunSolid {
		t.Fatalf("runs = %+v, want sprite then missing-texture solid placeholder", runs)
	}
	if len(builder.SpriteStream()) != SpriteStride {
		t.Fatalf("len(SpriteStream) = %d, want %d", len(builder.SpriteStream()), SpriteStride)
	}
	solid, tex := builder.Streams()
	if len(solid) == 0 || len(tex) != 0 {
		t.Fatalf("streams lengths = solid %d, tex %d; want solid placeholder only", len(solid), len(tex))
	}
}

func TestBuilderSkipsSpriteWithDegenerateClip(t *testing.T) {
	cmd := render.DrawImage{
		BaseCommand: render.BaseCommand{Clip: math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 0, Y: 10})},
		Texture:     1,
		Dest:        math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 20, Y: 20}),
	}

	var builder Builder
	runs := builder.Build(runtime.Frame{
		Camera:   testCamera(),
		Commands: []render.Command{cmd},
	}, 100, 100, nil, TexSizes{{}, {64, 64}})

	if len(runs) != 0 {
		t.Fatalf("runs = %+v, want no draw for degenerate clip", runs)
	}
	if len(builder.SpriteStream()) != 0 {
		t.Fatalf("len(SpriteStream) = %d, want 0", len(builder.SpriteStream()))
	}
}
