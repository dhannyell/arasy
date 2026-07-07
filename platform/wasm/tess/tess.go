package tess

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// Vertex is one interleaved vertex: clip-space position and straight-alpha
// color, matching the WGSL vertex layout (float32x2 + float32x4).
type Vertex struct {
	X, Y       float32
	R, G, B, A float32
}

// Mesh accumulates triangles for the solid-color pipeline.
type Mesh struct {
	Vertices []Vertex
}

type ctx struct {
	cam  render.Camera
	w, h float64
}

func vertex(pos [2]float32, col render.Color) Vertex {
	return Vertex{
		X: pos[0],
		Y: pos[1],
		R: float32(col.R),
		G: float32(col.G),
		B: float32(col.B),
		A: float32(col.A),
	}
}

func (c ctx) toClip(world math2d.Vec2) (x, y float32) {
	s := c.cam.WorldToScreen(world)
	return float32(2*s.X/c.w - 1), float32(1 - 2*s.Y/c.h)
}

func pushTri(m *Mesh, a, b, c [2]float32, col render.Color) {
	m.Vertices = append(m.Vertices,
		vertex(a, col),
		vertex(b, col),
		vertex(c, col),
	)
}

func pushQuad(m *Mesh, a, b, c, d [2]float32, col render.Color) {
	pushTri(m, a, b, c, col)
	pushTri(m, a, c, d, col)
}

// Build tessellates frame commands into solid-color and text meshes.
//
// Solid commands are appended to the solid mesh in input order. Text commands
// are tessellated into the text mesh when atlas is non-nil; text is therefore
// drawn after all solids by the WebGPU v1 renderer.
func Build(frame runtime.Frame, w, h int, atlas *Atlas) (Mesh, TextMesh) {
	c := ctx{
		cam: frame.Camera,
		w:   float64(w),
		h:   float64(h),
	}

	var solid Mesh
	var text TextMesh

	for _, cmd := range frame.Commands {
		switch cmd := cmd.(type) {
		case render.DrawRect:
			AppendRect(&solid, c, cmd)
		case render.DrawCircle:
			AppendCircle(&solid, c, cmd)
		case render.DrawPoint:
			AppendPoint(&solid, c, cmd)
		case render.DrawLine:
			AppendLine(&solid, c, cmd)
		case render.DrawArc:
			AppendArc(&solid, c, cmd)
		case render.DrawPolygon:
			AppendPolygon(&solid, c, cmd)
		case render.DrawText:
			if atlas != nil {
				AppendText(&text, c, cmd, atlas)
			}
		}
	}

	return solid, text
}
