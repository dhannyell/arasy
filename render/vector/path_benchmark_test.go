package vector_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/render/vector"
)

func BenchmarkVectorSplashLikePath(b *testing.B) {
	var builder vector.Builder
	builder.MoveTo(math2d.Vec2{X: 10, Y: 50})
	builder.CubicTo(
		math2d.Vec2{X: 20, Y: 5},
		math2d.Vec2{X: 80, Y: 5},
		math2d.Vec2{X: 90, Y: 50},
	)
	builder.CubicTo(
		math2d.Vec2{X: 80, Y: 95},
		math2d.Vec2{X: 20, Y: 95},
		math2d.Vec2{X: 10, Y: 50},
	)
	builder.Close()
	path, err := builder.Build()
	if err != nil {
		b.Fatal(err)
	}
	style := vector.FillStyle{Color: render.RGBA(1, 1, 1, 1), Tolerance: 0.25}
	b.ReportAllocs()
	for range b.N {
		if _, err := path.AppendFill(nil, render.BaseCommand{}, style); err != nil {
			b.Fatal(err)
		}
	}
}
