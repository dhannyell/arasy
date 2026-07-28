package vectoranim

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/internal/svggen"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/render"
)

const (
	benchmarkVectorFrameCount = 180
	benchmarkVectorFrameRate  = 60
)

var (
	//go:embed scene.svg
	benchmarkVectorSVG []byte

	benchmarkVectorViewport = math2d.NewRect(
		math2d.Vec2{},
		math2d.Vec2{X: 320, Y: 180},
	)
	benchmarkVectorCamera = render.Camera{
		World:    benchmarkVectorViewport,
		Viewport: benchmarkVectorViewport,
		Aspect:   render.AspectFit,
	}

	benchmarkVectorSourceSink   []byte
	benchmarkVectorCommandsSink []render.Command
	benchmarkVectorRunsSink     []tess.StreamRun
	benchmarkVectorPixelSink    uint8
)

type vectorBenchmarkWorkload struct {
	name     string
	commands [][]render.Command
}

func BenchmarkVectorAnimationCompile(b *testing.B) {
	b.ReportAllocs()
	options := svggen.GenerateOptions{Package: "vectoranim", Name: "VectorScene"}

	var generated []byte
	for b.Loop() {
		document, err := svggen.Parse(benchmarkVectorSVG, svggen.Options{Strict: true})
		if err != nil {
			b.Fatal(err)
		}
		generated, err = svggen.Generate(document, options)
		if err != nil {
			b.Fatal(err)
		}
	}
	benchmarkVectorSourceSink = generated
}

func BenchmarkVectorAnimationCommands(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			b.ReportAllocs()

			var commands []render.Command
			for i := 0; b.Loop(); i++ {
				frame := i % len(workload.commands)
				commands = AppendVectorScene(
					nil,
					anim.At(workload.elapsed(frame)),
					benchmarkVectorViewport,
					VectorSceneResources{},
				)
			}
			b.ReportMetric(workload.averageCommandCount(), "commands/frame")
			benchmarkVectorCommandsSink = commands
		})
	}
}

func BenchmarkVectorAnimationCommandsReuse(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			var workspace VectorSceneWorkspace
			b.ReportAllocs()

			var commands []render.Command
			for i := 0; b.Loop(); i++ {
				frame := i % len(workload.commands)
				commands = workspace.Append(
					anim.At(workload.elapsed(frame)),
					benchmarkVectorViewport,
					VectorSceneResources{},
				)
			}
			b.ReportMetric(workload.averageCommandCount(), "commands/frame")
			benchmarkVectorCommandsSink = commands
		})
	}
}

func BenchmarkVectorAnimationGPUPreparation(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			var builder tess.Builder
			metrics := measureVectorGPUWorkload(&builder, workload)
			b.ReportAllocs()

			var runs []tess.StreamRun
			for i := 0; b.Loop(); i++ {
				commands := workload.commands[i%len(workload.commands)]
				runs = builder.Build(
					tess.Frame{Camera: benchmarkVectorCamera, Commands: commands},
					320,
					180,
					nil,
					nil,
				)
			}
			metrics.report(b)
			benchmarkVectorRunsSink = runs
		})
	}
}

func BenchmarkVectorAnimationGPUEndToEnd(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			var builder tess.Builder
			metrics := measureVectorGPUWorkload(&builder, workload)
			b.ReportAllocs()

			var commands []render.Command
			var runs []tess.StreamRun
			for i := 0; b.Loop(); i++ {
				frame := i % len(workload.commands)
				commands = AppendVectorScene(
					nil,
					anim.At(workload.elapsed(frame)),
					benchmarkVectorViewport,
					VectorSceneResources{},
				)
				runs = builder.Build(
					tess.Frame{Camera: benchmarkVectorCamera, Commands: commands},
					320,
					180,
					nil,
					nil,
				)
			}
			metrics.report(b)
			benchmarkVectorCommandsSink = commands
			benchmarkVectorRunsSink = runs
		})
	}
}

func BenchmarkVectorAnimationGPUEndToEndReuse(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			var builder tess.Builder
			var workspace VectorSceneWorkspace
			metrics := measureVectorGPUWorkload(&builder, workload)
			b.ReportAllocs()

			var commands []render.Command
			var runs []tess.StreamRun
			for i := 0; b.Loop(); i++ {
				frame := i % len(workload.commands)
				commands = workspace.Append(
					anim.At(workload.elapsed(frame)),
					benchmarkVectorViewport,
					VectorSceneResources{},
				)
				runs = builder.Build(
					tess.Frame{Camera: benchmarkVectorCamera, Commands: commands},
					320,
					180,
					nil,
					nil,
				)
			}
			metrics.report(b)
			benchmarkVectorCommandsSink = commands
			benchmarkVectorRunsSink = runs
		})
	}
}

func BenchmarkVectorAnimationRasterEndToEnd(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			renderer := raster.Renderer{
				Width:  320,
				Height: 180,
				Camera: benchmarkVectorCamera,
				Clear:  render.RGBA8(16, 24, 39, 255),
			}
			target := renderer.NewTarget()
			b.ReportAllocs()

			var commands []render.Command
			for i := 0; b.Loop(); i++ {
				frame := i % len(workload.commands)
				commands = AppendVectorScene(
					nil,
					anim.At(workload.elapsed(frame)),
					benchmarkVectorViewport,
					VectorSceneResources{},
				)
				renderer.RenderInto(target, commands)
			}
			b.ReportMetric(workload.averageCommandCount(), "commands/frame")
			benchmarkVectorCommandsSink = commands
			benchmarkVectorPixelSink = target.Pix[len(target.Pix)/2]
		})
	}
}

func BenchmarkVectorAnimationRasterEndToEndReuse(b *testing.B) {
	for _, workload := range vectorBenchmarkWorkloads() {
		b.Run(workload.name, func(b *testing.B) {
			renderer := raster.Renderer{
				Width:  320,
				Height: 180,
				Camera: benchmarkVectorCamera,
				Clear:  render.RGBA8(16, 24, 39, 255),
			}
			target := renderer.NewTarget()
			var workspace VectorSceneWorkspace
			b.ReportAllocs()

			var commands []render.Command
			for i := 0; b.Loop(); i++ {
				frame := i % len(workload.commands)
				commands = workspace.Append(
					anim.At(workload.elapsed(frame)),
					benchmarkVectorViewport,
					VectorSceneResources{},
				)
				renderer.RenderInto(target, commands)
			}
			b.ReportMetric(workload.averageCommandCount(), "commands/frame")
			benchmarkVectorCommandsSink = commands
			benchmarkVectorPixelSink = target.Pix[len(target.Pix)/2]
		})
	}
}

func BenchmarkVectorAnimationScaleLOD(b *testing.B) {
	for _, test := range []struct {
		name     string
		viewport math2d.Rect
	}{
		{
			name: "Coarse",
			viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: 160, Y: 90},
			),
		},
		{name: "Standard", viewport: benchmarkVectorViewport},
		{
			name: "Fine",
			viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: 640, Y: 360},
			),
		},
	} {
		b.Run(test.name, func(b *testing.B) {
			var workspace VectorSceneWorkspace
			b.ReportAllocs()

			var commands []render.Command
			for b.Loop() {
				commands = workspace.Append(anim.At(3), test.viewport, VectorSceneResources{})
			}
			metrics := workspace.FrameMetrics()
			b.ReportMetric(float64(metrics.LOD), "lod")
			b.ReportMetric(float64(vectorTriangleVertices(commands)), "triangle-vertices/frame")
			benchmarkVectorCommandsSink = commands
		})
	}
}

func BenchmarkVectorAnimationCulling(b *testing.B) {
	for _, nodeCount := range []int{100, 1000} {
		b.Run(fmt.Sprintf("Offscreen%d", nodeCount), func(b *testing.B) {
			nodes := offscreenVectorNodes(nodeCount)
			viewBox := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 180})
			var builder render.Builder
			var metrics svgFrameMetrics
			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				builder.Reset(nil)
				metrics = svgFrameMetrics{}
				appendSVGNode(
					&builder,
					nodes,
					0,
					anim.At(0),
					func(uint16) render.FontID { return 0 },
					nil,
					nil,
					0.25,
					viewBox,
					true,
					math2d.Identity(),
					&metrics,
				)
				if len(builder.Commands()) != 0 {
					b.Fatal("offscreen nodes emitted commands")
				}
			}
			b.ReportMetric(float64(metrics.CulledNodes), "culled/frame")
			b.ReportMetric(float64(metrics.VisitedNodes), "nodes/frame")
		})
	}
}

func vectorTriangleVertices(commands []render.Command) int {
	total := 0
	for _, command := range commands {
		if triangles, ok := command.(render.DrawTriangles); ok {
			total += len(triangles.Vertices)
		}
	}
	return total
}

func offscreenVectorNodes(count int) []svgNode {
	nodes := make([]svgNode, count+1)
	nodes[0] = svgNode{
		children:   make([]uint16, count),
		path:       -1,
		motionPath: -1,
		opacity:    1,
		transform:  math2d.Identity(),
	}
	for index := range count {
		child := index + 1
		nodes[0].children[index] = uint16(child)
		x := 1000 + float64(index%100)*20
		y := 1000 + float64(index/100)*20
		nodes[child] = svgNode{
			kind:       1,
			path:       -1,
			motionPath: -1,
			x:          x,
			y:          y,
			width:      10,
			height:     10,
			fill:       render.RGBA8(255, 255, 255, 255),
			opacity:    1,
			transform:  math2d.Identity(),
			bounds: math2d.NewRect(
				math2d.Vec2{X: x, Y: y},
				math2d.Vec2{X: 10, Y: 10},
			),
			hasBounds: true,
		}
	}
	return nodes
}

func vectorBenchmarkWorkloads() []vectorBenchmarkWorkload {
	return []vectorBenchmarkWorkload{
		{name: "Opening", commands: vectorBenchmarkFrames(0, 1)},
		{name: "Active", commands: vectorBenchmarkFrames(1.5, 1)},
		{name: "Settled", commands: vectorBenchmarkFrames(3, 1)},
		{name: "Timeline", commands: vectorBenchmarkFrames(0, benchmarkVectorFrameCount)},
	}
}

func vectorBenchmarkFrames(start float64, count int) [][]render.Command {
	frames := make([][]render.Command, count)
	for i := range frames {
		elapsed := start + float64(i)/benchmarkVectorFrameRate
		frames[i] = AppendVectorScene(
			nil,
			anim.At(elapsed),
			benchmarkVectorViewport,
			VectorSceneResources{},
		)
	}
	return frames
}

func (w vectorBenchmarkWorkload) elapsed(frame int) float64 {
	switch w.name {
	case "Active":
		return 1.5
	case "Settled":
		return 3
	default:
		return float64(frame) / benchmarkVectorFrameRate
	}
}

func (w vectorBenchmarkWorkload) averageCommandCount() float64 {
	var total int
	for _, commands := range w.commands {
		total += len(commands)
	}
	return float64(total) / float64(len(w.commands))
}

type vectorGPUMetrics struct {
	runs  float64
	bytes float64
}

func measureVectorGPUWorkload(builder *tess.Builder, workload vectorBenchmarkWorkload) vectorGPUMetrics {
	var totalRuns, totalBytes int
	for _, commands := range workload.commands {
		runs := builder.Build(
			tess.Frame{Camera: benchmarkVectorCamera, Commands: commands},
			320,
			180,
			nil,
			nil,
		)
		solid, textured := builder.Streams()
		totalRuns += len(runs)
		totalBytes += len(solid) + len(textured)
	}
	return vectorGPUMetrics{
		runs:  float64(totalRuns) / float64(len(workload.commands)),
		bytes: float64(totalBytes) / float64(len(workload.commands)),
	}
}

func (m vectorGPUMetrics) report(b *testing.B) {
	b.ReportMetric(m.runs, "runs/frame")
	b.ReportMetric(m.bytes, "stream-B/frame")
}
