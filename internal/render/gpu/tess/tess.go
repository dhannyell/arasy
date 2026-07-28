package tess

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Vertex is one interleaved vertex: clip-space position and straight-alpha
// color, matching the WGSL vertex layout (float32x2 + float32x4).
type Vertex struct {
	X, Y       float32
	R, G, B, A float32
}

// Mesh accumulates triangles for the solid-color pipeline.
type Mesh struct {
	Vertices    []Vertex
	strokeJoins [][2]math2d.Vec2
	stream      *[]byte
	collect     bool
}

func (m *Mesh) reset(stream *[]byte, collect bool) {
	m.Vertices = m.Vertices[:0]
	m.stream = stream
	m.collect = collect
}

func (m *Mesh) append(vertices ...Vertex) {
	if m.stream != nil && !m.collect {
		for _, v := range vertices {
			*m.stream = appendSolidVertex(*m.stream, v)
		}
		return
	}
	m.Vertices = append(m.Vertices, vertices...)
}

func (m *Mesh) flush() {
	if m.stream == nil {
		return
	}
	for _, v := range m.Vertices {
		*m.stream = appendSolidVertex(*m.stream, v)
	}
	m.Vertices = m.Vertices[:0]
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
	m.append(
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

// DrawRunKind identifies the WebGPU pipeline needed for one ordered run.
type DrawRunKind uint8

const (
	DrawRunSolid DrawRunKind = iota
	DrawRunTextured
	DrawRunSprites
	DrawRunText
	DrawRunParticles
	// DrawRunSpriteBatch draws one retained sprite batch. The run carries the
	// command (batch ID and animation uniforms) but no instance payload:
	// content lives in the backend's retained registry.
	DrawRunSpriteBatch
	// DrawRunTextBatch draws one retained text batch. The run carries the
	// command (batch ID) but no glyph/label payload: content lives in the
	// backend's retained text registry.
	DrawRunTextBatch
)

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
