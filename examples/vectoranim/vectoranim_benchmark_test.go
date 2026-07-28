package vectoranim

import (
	_ "embed"
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
