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

// missingTexturePlaceholder mirrors platform/raster/renderer.go's
// missingTextureColor: an opaque magenta fill drawn in place of a
// render.DrawImage/DrawImageQuad whose texture id could not be resolved.
var missingTexturePlaceholder = render.RGBA8(255, 0, 255, 255)

// Build tessellates frame commands into solid-color, textured, and text
// meshes.
//
// Draw order is solids first, then textured runs, then text, matching the
// WebGPU v1 renderer's pass order.
//
// Solid commands (including the magenta placeholder substituted for a
// command whose texture could not be resolved via sizes) are appended to the
// solid mesh in input order. Textured commands are grouped into contiguous
// runs by texture: a new TexMesh starts whenever the texture differs from
// the previous textured command, so consecutive draws on the same texture
// share one draw call while the returned slice still reflects overall
// drawing order. Text commands are tessellated into the text mesh when atlas
// is non-nil.
func Build(frame runtime.Frame, w, h int, atlas *Atlas, sizes TexSizes) (Mesh, []TexMesh, TextMesh) {
	c := ctx{
		cam: frame.Camera,
		w:   float64(w),
		h:   float64(h),
	}

	var solid Mesh
	var textured []TexMesh
	var text TextMesh

	// runIndex is the index into textured that the current run of same-
	// texture draws appends to, or -1 when no run is open yet. This is kept
	// as an index rather than a *TexMesh: appending to textured can move its
	// backing array, which would invalidate a pointer taken before the
	// append.
	runIndex := -1

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
		case render.DrawImage:
			texW, texH, ok := sizes.Lookup(cmd.Texture)
			if !ok {
				if corners, ok := destCorners(c, cmd.Dest, cmd.Rotation); ok {
					appendMissingTexture(&solid, c, corners, cmd.Clip)
				}
				continue
			}
			runIndex = appendTexRun(&textured, runIndex, cmd.Texture)
			AppendImage(&textured[runIndex], c, cmd, texW, texH)
		case render.DrawImageQuad:
			screen, _, valid := imageQuadProjection(c, cmd.Corners)
			if !valid {
				continue
			}
			texW, texH, ok := sizes.Lookup(cmd.Texture)
			if !ok {
				appendMissingTexture(&solid, c, screen, cmd.Clip)
				continue
			}
			runIndex = appendTexRun(&textured, runIndex, cmd.Texture)
			AppendImageQuad(&textured[runIndex], c, cmd, texW, texH)
		}
	}

	return solid, textured, text
}

// appendTexRun returns the index within runs that a command drawing texture
// id should append its vertices to. It reuses the currently open run
// (current) when its texture matches id, otherwise it starts a new run.
func appendTexRun(runs *[]TexMesh, current int, id render.TextureID) int {
	if current >= 0 && (*runs)[current].Texture == id {
		return current
	}
	*runs = append(*runs, TexMesh{Texture: id})
	return len(*runs) - 1
}

// appendMissingTexture emits a solid magenta quad from screen-space corners
// (TL, TR, BR, BL), in place of a DrawImage or DrawImageQuad command whose
// texture id could not be resolved through TexSizes.
func appendMissingTexture(m *Mesh, c ctx, corners [4]math2d.Vec2, clip math2d.Rect) {
	start := len(m.Vertices)
	pushScreenQuad(m, c,
		corners[0].X, corners[0].Y,
		corners[1].X, corners[1].Y,
		corners[2].X, corners[2].Y,
		corners[3].X, corners[3].Y,
		missingTexturePlaceholder,
	)

	if clip.Width() > 0 && clip.Height() > 0 {
		clipped := clipTriangles(m.Vertices[start:], clipRect(c, clip))
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}
