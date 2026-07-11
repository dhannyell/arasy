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

// DrawRunKind identifies the WebGPU pipeline needed for one ordered run.
type DrawRunKind uint8

const (
	DrawRunSolid DrawRunKind = iota
	DrawRunTextured
	DrawRunText
)

// DrawRun contains commands that can be submitted together while preserving
// painter order between different WebGPU pipelines. Runs are split whenever
// the pipeline changes, and textured runs are also split when their texture
// changes.
type DrawRun struct {
	Kind     DrawRunKind
	Solid    Mesh
	Textured TexMesh
	Text     TextMesh
}

// Build tessellates frame commands into ordered, heterogeneous WebGPU runs.
// render.Each owns layer sorting and command dispatch; this sink only maps
// each command to the appropriate tessellation pipeline.
func Build(frame runtime.Frame, w, h int, atlas *Atlas, sizes TexSizes) []DrawRun {
	sink := buildSink{
		ctx: ctx{
			cam: frame.Camera,
			w:   float64(w),
			h:   float64(h),
		},
		atlas: atlas,
		sizes: sizes,
	}
	render.Each(frame.Commands, &sink)
	return sink.runs
}

type buildSink struct {
	ctx   ctx
	atlas *Atlas
	sizes TexSizes
	runs  []DrawRun

	lastKind    DrawRunKind
	lastTexture render.TextureID
	hasLast     bool
}

func (s *buildSink) run(kind DrawRunKind, texture render.TextureID) *DrawRun {
	if s.hasLast && s.lastKind == kind && (kind != DrawRunTextured || s.lastTexture == texture) {
		return &s.runs[len(s.runs)-1]
	}
	s.runs = append(s.runs, DrawRun{Kind: kind})
	s.lastKind = kind
	s.lastTexture = texture
	s.hasLast = true
	return &s.runs[len(s.runs)-1]
}

func (s *buildSink) VisitRect(cmd render.DrawRect) {
	AppendRect(&s.run(DrawRunSolid, 0).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitCircle(cmd render.DrawCircle) {
	AppendCircle(&s.run(DrawRunSolid, 0).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitPoint(cmd render.DrawPoint) {
	AppendPoint(&s.run(DrawRunSolid, 0).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitLine(cmd render.DrawLine) {
	AppendLine(&s.run(DrawRunSolid, 0).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitArc(cmd render.DrawArc) {
	AppendArc(&s.run(DrawRunSolid, 0).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitPolygon(cmd render.DrawPolygon) {
	AppendPolygon(&s.run(DrawRunSolid, 0).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitText(cmd render.DrawText) {
	if s.atlas == nil {
		return
	}
	AppendText(&s.run(DrawRunText, 0).Text, s.ctx, cmd, s.atlas)
}

func (s *buildSink) VisitImage(cmd render.DrawImage) {
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		if corners, ok := destCorners(s.ctx, cmd.Dest, cmd.Rotation); ok {
			appendMissingTexture(&s.run(DrawRunSolid, 0).Solid, s.ctx, corners, cmd.Clip)
		}
		return
	}
	run := s.run(DrawRunTextured, cmd.Texture)
	run.Textured.Texture = cmd.Texture
	AppendImage(&run.Textured, s.ctx, cmd, texW, texH)
}

func (s *buildSink) VisitImageQuad(cmd render.DrawImageQuad) {
	screen, _, valid := imageQuadProjection(s.ctx, cmd.Corners)
	if !valid {
		return
	}
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		appendMissingTexture(&s.run(DrawRunSolid, 0).Solid, s.ctx, screen, cmd.Clip)
		return
	}
	run := s.run(DrawRunTextured, cmd.Texture)
	run.Textured.Texture = cmd.Texture
	AppendImageQuad(&run.Textured, s.ctx, cmd, texW, texH)
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
