package tess

import (
	"strconv"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
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

// BenchmarkTessBuild measures tess.Build under sprite-heavy loads: pure
// textured images (one contiguous run), circle-only tessellation (fan cost
// per radius), and mixed commands that force a run split on every image.
func BenchmarkTessBuild(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()

	b.Run("images", func(b *testing.B) {
		for _, n := range []int{1000, 5000, 20000, 50000} {
			commands := makeImageCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})

	b.Run("mixed", func(b *testing.B) {
		for _, n := range []int{1000, 20000} {
			commands := makeMixedCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})

	b.Run("circles", func(b *testing.B) {
		for _, n := range []int{1000, 20000} {
			commands := makeCircleCommands(n)
			frame := runtime.Frame{Camera: cam, Commands: commands}
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					_ = Build(frame, 1920, 1080, nil, sizes)
				}
			})
		}
	})
}

// BenchmarkTessBytes measures the second copy of the per-frame vertex
// stream: converting an already-tessellated TexMesh into the WebGPU vertex
// buffer byte layout. The mesh is built once from 20000 images (a single
// contiguous textured run) and reused across iterations, isolating TexBytes
// from Build.
func BenchmarkTessBytes(b *testing.B) {
	cam := benchCamera()
	sizes := benchImageTexSizes()
	frame := runtime.Frame{Camera: cam, Commands: makeImageCommands(20000)}
	runs := Build(frame, 1920, 1080, nil, sizes)
	if len(runs) != 1 || runs[0].Kind != DrawRunTextured {
		b.Fatalf("setup runs = %+v, want one textured run", runs)
	}
	mesh := runs[0].Textured

	b.Run("TexBytes/images_20000", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = TexBytes(mesh)
		}
	})
}
