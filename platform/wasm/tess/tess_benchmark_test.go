package tess

import (
	"math"
	"strconv"
	"testing"

	"github.com/dhannyell/frameforge/asset/typeface"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"golang.org/x/image/font/gofont/goregular"
)

// benchWorldSize and benchViewport describe a browser-scale frame: a world
// large enough to spread tens of thousands of sprites without overlap, and a
// 1080p viewport matching a typical WebGPU host.
const benchWorldSize = 4000.0

func benchCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: benchWorldSize, Y: benchWorldSize}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}),
		Aspect:   render.AspectExpand,
	}
}

type benchmarkTypefaceProvider struct {
	face    *typeface.Face
	builtin *Atlas
	atlases map[AtlasKey]*Atlas
}

func newBenchmarkTypefaceProvider(b *testing.B) *benchmarkTypefaceProvider {
	b.Helper()
	face, err := typeface.Parse(goregular.TTF)
	if err != nil {
		b.Fatal(err)
	}
	return &benchmarkTypefaceProvider{face: face, builtin: NewAtlas(), atlases: make(map[AtlasKey]*Atlas)}
}

func (p *benchmarkTypefaceProvider) AtlasFor(fontID render.FontID, px int, text string) (*Atlas, error) {
	if fontID == 0 {
		return p.builtin, nil
	}
	key := AtlasKey{Font: fontID, PX: px}
	if atlas := p.atlases[key]; atlas != nil {
		return atlas, nil
	}
	atlas, err := NewAtlasForFace(fontID, px, p.face, []rune(text))
	if err == nil {
		p.atlases[key] = atlas
	}
	return atlas, err
}

func (p *benchmarkTypefaceProvider) CachedAtlas(fontID render.FontID, px int) (*Atlas, bool) {
	if fontID == 0 {
		return p.builtin, true
	}
	atlas, ok := p.atlases[AtlasKey{Font: fontID, PX: px}]
	return atlas, ok
}

// benchGridPosition places index i on a square grid spanning benchWorldSize,
// spaced enough to avoid full overlap for the largest sprite used in these
// benchmarks. It is a pure function of (i, n): deterministic and independent
// of iteration order.
func benchGridPosition(i, n int) math2d.Vec2 {
	cols := 1
	for cols*cols < n {
		cols++
	}
	spacing := benchWorldSize / float64(cols)
	return math2d.Vec2{
		X: float64(i%cols) * spacing,
		Y: float64(i/cols) * spacing,
	}
}

// benchImageTexSizes describes one 64x64 texture at TextureID 1. Index 0 is
// the reserved "missing texture" slot (see TexSizes.Lookup).
func benchImageTexSizes() TexSizes {
	return TexSizes{{}, {64, 64}}
}

func makeImageCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		commands = append(commands, render.DrawImage{
			Texture: 1,
			Dest:    math2d.NewRect(pos, math2d.Vec2{X: 16, Y: 16}),
		})
	}
	return commands
}

// makeMixedCommands intercalates DrawCircle, DrawRect and DrawImage so every
// third command switches pipeline kind (Solid <-> Textured). LayerValue is
// monotonically increasing so render.Each's stable sort is a no-op and this
// submission order survives into Build unchanged, forcing the run builder to
// split on every image insertion regardless of tie-breaking behavior.
func makeMixedCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		base := render.BaseCommand{LayerValue: i}
		switch i % 3 {
		case 0:
			commands = append(commands, render.DrawCircle{
				BaseCommand: base,
				Center:      pos,
				Radius:      2 + float64(i%49),
				Fill:        render.RGBA(1, 0, 0, 1),
			})
		case 1:
			commands = append(commands, render.DrawRect{
				BaseCommand: base,
				Rect:        math2d.NewRect(pos, math2d.Vec2{X: 8, Y: 8}),
				Fill:        render.RGBA(0, 1, 0, 1),
			})
		default:
			commands = append(commands, render.DrawImage{
				BaseCommand: base,
				Texture:     1,
				Dest:        math2d.NewRect(pos, math2d.Vec2{X: 16, Y: 16}),
			})
		}
	}
	return commands
}

// makeCircleCommands generates only DrawCircle commands with radii cycling
// through 2-50px, exercising the per-radius segment-count fan tessellation in
// AppendCircle.
func makeCircleCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		commands = append(commands, render.DrawCircle{
			Center: pos,
			Radius: 2 + float64(i%49),
			Fill:   render.RGBA(0, 0, 1, 1),
		})
	}
	return commands
}

// benchTextSizes cycles through a handful of world-space font sizes so
// resolveFrameAtlases groups commands into several distinct AtlasKeys (one
// per (Font, PX) pair produced by the camera's world-to-screen scale),
// exercising its per-frame map/string bookkeeping under a realistic HUD-like
// mix of label sizes.
var benchTextSizes = []float64{10, 14, 20, 32}

// makeTextCommands generates short HUD-style labels ("Score: 1234") cycling
// through benchTextSizes. All commands use the built-in font (Font 0), the
// only font NewAtlas's AtlasFor accepts, so every command resolves through
// the atlas map built once per frame in resolveFrameAtlases. This measures
// VisitText plus that per-frame resolution cost, including its allocations.
func makeTextCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		commands = append(commands, render.DrawText{
			Position: pos,
			Text:     "Score: " + strconv.Itoa(i%10000),
			Color:    render.RGBA(1, 1, 1, 1),
			Size:     benchTextSizes[i%len(benchTextSizes)],
		})
	}
	return commands
}

// makeClipCommands is makeImageCommands with a non-empty Clip on every
// command, sized to cover most of the destination sprite so the fragment
// clip test has a non-empty region to evaluate.
func makeClipCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		dest := math2d.NewRect(pos, math2d.Vec2{X: 16, Y: 16})
		commands = append(commands, render.DrawImage{
			// Clip covers all but a 1px border of Dest so the command exercises
			// per-sprite screen-space clip packing.
			BaseCommand: render.BaseCommand{
				Clip: math2d.NewRect(
					math2d.Vec2{X: dest.Min.X + 1, Y: dest.Min.Y + 1},
					math2d.Vec2{X: dest.Max.X - 1, Y: dest.Max.Y - 1},
				),
			},
			Texture: 1,
			Dest:    dest,
		})
	}
	return commands
}

// benchLayerShuffle deterministically permutes index i into a LayerValue in
// [0, n) using a Knuth multiplicative hash. It is a pure function of (i, n):
// same output every call, no math/rand involved.
func benchLayerShuffle(i, n int) int {
	return int((uint64(i) * 2654435761) % uint64(n))
}

// makeUnsortedMixedCommands is makeMixedCommands with LayerValue scrambled by
// benchLayerShuffle instead of monotonically increasing. render.Each copies
// its input into a fresh slice before stable-sorting it (see render/dispatch.go
// Each), so the caller's slice is never mutated; no rebuild between
// iterations is needed for a correct measurement. This isolates the cost of
// that per-frame sort plus copy on top of the same command mix as "mixed".
func makeUnsortedMixedCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		base := render.BaseCommand{LayerValue: benchLayerShuffle(i, n)}
		switch i % 3 {
		case 0:
			commands = append(commands, render.DrawCircle{
				BaseCommand: base,
				Center:      pos,
				Radius:      2 + float64(i%49),
				Fill:        render.RGBA(1, 0, 0, 1),
			})
		case 1:
			commands = append(commands, render.DrawRect{
				BaseCommand: base,
				Rect:        math2d.NewRect(pos, math2d.Vec2{X: 8, Y: 8}),
				Fill:        render.RGBA(0, 1, 0, 1),
			})
		default:
			commands = append(commands, render.DrawImage{
				BaseCommand: base,
				Texture:     1,
				Dest:        math2d.NewRect(pos, math2d.Vec2{X: 16, Y: 16}),
			})
		}
	}
	return commands
}

// benchChurnTextureCount and benchChurnTexSizes describe 8 distinct 64x64
// textures (IDs 1-8; index 0 stays the reserved "missing texture" slot).
const benchChurnTextureCount = 8

func benchChurnTexSizes() TexSizes {
	sizes := make(TexSizes, benchChurnTextureCount+1)
	for i := 1; i <= benchChurnTextureCount; i++ {
		sizes[i] = [2]int{64, 64}
	}
	return sizes
}

// makeTextureChurnCommands is makeImageCommands cycling through
// benchChurnTextureCount distinct TextureIDs instead of a single one. Every
// command's texture differs from the last, so streamSink.run must split the
// textured run on every command instead of reusing the previous StreamRun.
// This isolates run-splitting overhead from the single-run "images" case.
func makeTextureChurnCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		commands = append(commands, render.DrawImage{
			Texture: render.TextureID(i%benchChurnTextureCount + 1),
			Dest:    math2d.NewRect(pos, math2d.Vec2{X: 16, Y: 16}),
		})
	}
	return commands
}

// makeImageQuadCommands generates render.DrawImageQuad commands whose four
// corners are derived from the grid position, exercising imageQuadProjection
// and the homography path in AppendImageQuad instead of AppendImage's
// axis-aligned destCorners.
func makeImageQuadCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		commands = append(commands, render.DrawImageQuad{
			Texture: 1,
			Corners: [4]math2d.Vec2{
				{X: pos.X, Y: pos.Y},
				{X: pos.X + 16, Y: pos.Y},
				{X: pos.X + 16, Y: pos.Y + 16},
				{X: pos.X, Y: pos.Y + 16},
			},
		})
	}
	return commands
}

// makeRotatedImageCommands is makeImageCommands with a non-zero Rotation on
// every command, exercising destCorners' rotate-around-center branch instead
// of its axis-aligned fast path.
func makeRotatedImageCommands(n int) []render.Command {
	commands := make([]render.Command, 0, n)
	for i := range n {
		pos := benchGridPosition(i, n)
		commands = append(commands, render.DrawImage{
			Texture:  1,
			Dest:     math2d.NewRect(pos, math2d.Vec2{X: 16, Y: 16}),
			Rotation: float64(i%360) * math.Pi / 180,
		})
	}
	return commands
}

// BenchmarkTessBuild measures reusable direct-byte tessellation under
// sprite-heavy loads: instanced sprite records (one contiguous run), circle-only
// tessellation (fan cost per radius), and mixed commands that force a run
// split on every image.
func BenchmarkTessBuild(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()

	b.Run("sprites", func(b *testing.B) {
		for _, n := range []int{1000, 5000, 20000, 50000} {
			commands := makeImageCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				var builder Builder
				b.ReportAllocs()
				for b.Loop() {
					_ = builder.Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})

	b.Run("mixed", func(b *testing.B) {
		for _, n := range []int{1000, 20000} {
			commands := makeMixedCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				var builder Builder
				b.ReportAllocs()
				for b.Loop() {
					_ = builder.Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})

	b.Run("circles", func(b *testing.B) {
		for _, n := range []int{1000, 20000} {
			commands := makeCircleCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				var builder Builder
				b.ReportAllocs()
				for b.Loop() {
					_ = builder.Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})
}

// BenchmarkTessBuildText measures VisitText plus resolveFrameAtlases under a
// HUD-like load: short labels at a handful of distinct sizes, all routed
// through the built-in font atlas. resolveFrameAtlases runs once per Build
// call and groups commands by (Font, PX) before any tessellation happens, so
// this isolates its per-frame map and string-concatenation allocations from
// AppendText itself.
func BenchmarkTessBuildText(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()
	provider := NewAtlas()

	for _, n := range []int{1000, 20000} {
		commands := makeTextCommands(n)
		frame := runtime.Frame{Camera: cam, Commands: commands}
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var builder Builder
			b.ReportAllocs()
			for b.Loop() {
				_ = builder.Build(frame, 1920, 1080, provider, sizes)
			}
		})
	}
}

// BenchmarkTessBuildExternalText measures the steady-state browser path:
// proportional TTF metrics, cached atlases, bounded label-layout reuse, and
// one compact storage record per glyph. The warm-up build deliberately stays
// outside the timer because atlas rasterization is a one-time resource cost.
func BenchmarkTessBuildExternalText(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()
	provider := newBenchmarkTypefaceProvider(b)

	for _, n := range []int{4000, 20000} {
		commands := makeTextCommands(n)
		for i, command := range commands {
			cmd := command.(render.DrawText)
			cmd.Font = 1
			commands[i] = cmd
		}
		frame := runtime.Frame{Camera: cam, Commands: commands}
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var builder Builder
			_ = builder.Build(frame, 1920, 1080, provider, sizes)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = builder.Build(frame, 1920, 1080, provider, sizes)
			}
		})
	}
}

// BenchmarkTessBuildClip measures sprite record generation with a non-empty
// clip on every command. DrawImage stores the screen-space clip in each
// record, so this stays on the direct record path.
func BenchmarkTessBuildClip(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()

	for _, n := range []int{1000, 20000} {
		commands := makeClipCommands(n)
		frame := runtime.Frame{Camera: cam, Commands: commands}
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var builder Builder
			b.ReportAllocs()
			for b.Loop() {
				_ = builder.Build(frame, 1920, 1080, nil, sizes)
			}
		})
	}
}

// BenchmarkTessBuildUnsortedLayers measures the same command mix as "mixed"
// but with LayerValue scrambled (benchLayerShuffle) instead of monotonically
// increasing, giving render.Each's stable sort real reordering work instead
// of a no-op pass. render.Each copies frame.Commands into a fresh slice
// before sorting (render/dispatch.go), so the caller's slice is never
// mutated between iterations; reusing the same commands/frame across
// b.Loop() iterations is safe and does not need a per-iteration rebuild.
func BenchmarkTessBuildUnsortedLayers(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()

	for _, n := range []int{1000, 20000} {
		commands := makeUnsortedMixedCommands(n)
		frame := runtime.Frame{Camera: cam, Commands: commands}
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var builder Builder
			b.ReportAllocs()
			for b.Loop() {
				_ = builder.Build(frame, 1920, 1080, nil, sizes)
			}
		})
	}
}

// BenchmarkTessBuildTextureChurn measures the sprite load with 8 distinct
// TextureIDs cycling instead of one, so a new StreamRun is appended for every
// command. This isolates run-splitting overhead from the single contiguous
// sprite run.
func BenchmarkTessBuildTextureChurn(b *testing.B) {
	cam := benchCamera()
	sizes := benchChurnTexSizes()

	for _, n := range []int{1000, 20000} {
		commands := makeTextureChurnCommands(n)
		frame := runtime.Frame{Camera: cam, Commands: commands}
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			var builder Builder
			b.ReportAllocs()
			for b.Loop() {
				_ = builder.Build(frame, 1920, 1080, nil, sizes)
			}
		})
	}
}

// BenchmarkTessBuildImageQuadAndRotation covers two paths that the sprite
// benchmark never exercises: DrawImageQuad's homography projection
// (imageQuadProjection/AppendImageQuad) and DrawImage with non-zero Rotation
// (the sprite record's angle-packing branch).
func BenchmarkTessBuildImageQuadAndRotation(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()

	b.Run("quad", func(b *testing.B) {
		for _, n := range []int{1000, 20000} {
			commands := makeImageQuadCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				var builder Builder
				b.ReportAllocs()
				for b.Loop() {
					_ = builder.Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})

	b.Run("rotated", func(b *testing.B) {
		for _, n := range []int{1000, 20000} {
			commands := makeRotatedImageCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				var builder Builder
				b.ReportAllocs()
				for b.Loop() {
					_ = builder.Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})
}
