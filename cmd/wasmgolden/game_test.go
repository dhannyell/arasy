package wasmgolden

import (
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestCurvedStrokeJoinGeometryReachesGPUStream(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
		Aspect:   render.AspectStretch,
	}
	var builder tess.Builder
	runs := builder.Build(tess.Frame{
		Camera: camera, Commands: []render.Command{curvedStrokeCommand},
	}, 320, 240, nil, nil)
	if len(runs) != 1 || runs[0].Kind != tess.DrawRunSolid {
		t.Fatalf("GPU runs = %+v, want one solid run", runs)
	}
	wantVertices := len(curvedStrokeCommand.Indices)
	if runs[0].VertexCount != wantVertices {
		t.Fatalf("GPU vertex count = %d, want %d", runs[0].VertexCount, wantVertices)
	}
	if wantVertices < 12 {
		t.Fatalf("curved stroke emitted only %d vertices; join fans are missing", wantVertices)
	}
}
