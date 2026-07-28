package tess

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestAppendTrianglesUsesSolidStreamAndTransform(t *testing.T) {
	context := testCtx()
	command := render.DrawTriangles{
		BaseCommand: render.BaseCommand{
			Transform: math2d.Translate(math2d.Vec2{X: 10}),
		},
		Vertices: []math2d.Vec2{{}, {X: 10}, {Y: 10}},
		Indices:  []uint32{0, 1, 2},
		Color:    render.RGBA(0.25, 0.5, 0.75, 1),
	}
	var mesh Mesh
	AppendTriangles(&mesh, context, command)
	if len(mesh.Vertices) != 3 {
		t.Fatalf("vertices = %d, want 3", len(mesh.Vertices))
	}
	want := screenToClip(context, context.cam.WorldToScreen(math2d.Vec2{X: 10}))
	if mesh.Vertices[0].X != want[0] || mesh.Vertices[0].Y != want[1] {
		t.Fatalf("first vertex = (%v,%v), want %v", mesh.Vertices[0].X, mesh.Vertices[0].Y, want)
	}
}

func TestAppendTrianglesSkipsInvalidGeometry(t *testing.T) {
	var mesh Mesh
	AppendTriangles(&mesh, testCtx(), render.DrawTriangles{
		Vertices: []math2d.Vec2{{}},
		Indices:  []uint32{0, 1, 2},
		Color:    render.RGBA(1, 1, 1, 1),
	})
	if len(mesh.Vertices) != 0 {
		t.Fatalf("vertices = %d, want 0", len(mesh.Vertices))
	}
}
