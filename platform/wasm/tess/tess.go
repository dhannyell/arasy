package tess

import (
	"math"

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
	DrawRunParticles
)

// DrawRun contains commands that can be submitted together while preserving
// painter order between different WebGPU pipelines. Runs are split whenever
// the pipeline changes, and textured runs are also split when their texture
// changes.
type DrawRun struct {
	Kind      DrawRunKind
	Solid     Mesh
	Textured  TexMesh
	Text      TextMesh
	Particles render.DrawParticles
	AtlasKey  AtlasKey
}

// Build tessellates frame commands into ordered, heterogeneous WebGPU runs.
// render.Each owns layer sorting and command dispatch; this sink only maps
// each command to the appropriate tessellation pipeline.
func Build(frame runtime.Frame, w, h int, provider AtlasProvider, sizes TexSizes) []DrawRun {
	buildCtx := ctx{
		cam: frame.Camera,
		w:   float64(w),
		h:   float64(h),
	}
	sink := buildSink{
		ctx:     buildCtx,
		atlases: resolveFrameAtlases(frame.Commands, buildCtx, provider),
		sizes:   sizes,
	}
	render.Each(frame.Commands, &sink)
	return sink.runs
}

type buildSink struct {
	ctx     ctx
	atlases map[AtlasKey]*Atlas
	sizes   TexSizes
	runs    []DrawRun

	lastKind    DrawRunKind
	lastTexture render.TextureID
	lastAtlas   AtlasKey
	hasLast     bool
}

func (s *buildSink) run(kind DrawRunKind, texture render.TextureID, atlas AtlasKey) *DrawRun {
	if s.hasLast && s.lastKind == kind &&
		(kind != DrawRunTextured || s.lastTexture == texture) &&
		(kind != DrawRunText || s.lastAtlas == atlas) {
		return &s.runs[len(s.runs)-1]
	}
	s.runs = append(s.runs, DrawRun{Kind: kind, AtlasKey: atlas})
	s.lastKind = kind
	s.lastTexture = texture
	s.lastAtlas = atlas
	s.hasLast = true
	return &s.runs[len(s.runs)-1]
}

func (s *buildSink) VisitRect(cmd render.DrawRect) {
	AppendRect(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitCircle(cmd render.DrawCircle) {
	AppendCircle(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitPoint(cmd render.DrawPoint) {
	AppendPoint(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitLine(cmd render.DrawLine) {
	AppendLine(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitArc(cmd render.DrawArc) {
	AppendArc(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitPolygon(cmd render.DrawPolygon) {
	AppendPolygon(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, cmd)
}

func (s *buildSink) VisitParticles(cmd render.DrawParticles) {
	// Particle commands own GPU simulation and indirect-draw state, so they
	// always remain one command per run. Reset the merge cursor so equal
	// geometry kinds on either side cannot merge across this painter-order
	// boundary.
	s.runs = append(s.runs, DrawRun{Kind: DrawRunParticles, Particles: cmd})
	s.hasLast = false
}

func (s *buildSink) VisitText(cmd render.DrawText) {
	size := cmd.Size
	if size <= 0 {
		size = glyphHeight
	}
	px := int(math.Round(s.ctx.cam.WorldScalarToScreen(size)))
	if px <= 0 {
		return
	}
	atlas := s.atlases[AtlasKey{Font: cmd.Font, PX: px}]
	if atlas == nil {
		return
	}
	AppendText(&s.run(DrawRunText, 0, atlas.Key).Text, s.ctx, cmd, atlas)
}

// resolveFrameAtlases groups text by requested key before tessellation. This
// matters for on-demand Unicode: every command using one (font, px) sees the
// same atlas layout, even when the provider creates a fresh atlas per call.
func resolveFrameAtlases(commands []render.Command, c ctx, provider AtlasProvider) map[AtlasKey]*Atlas {
	if provider == nil {
		return nil
	}
	type request struct {
		key  AtlasKey
		text string
	}
	requests := make([]request, 0)
	indices := make(map[AtlasKey]int)
	for _, command := range commands {
		cmd, ok := command.(render.DrawText)
		if !ok || cmd.Text == "" || cmd.Color.A <= 0 {
			continue
		}
		size := cmd.Size
		if size <= 0 {
			size = glyphHeight
		}
		px := int(math.Round(c.cam.WorldScalarToScreen(size)))
		if px <= 0 {
			continue
		}
		key := AtlasKey{Font: cmd.Font, PX: px}
		if i, exists := indices[key]; exists {
			requests[i].text += cmd.Text
			continue
		}
		indices[key] = len(requests)
		requests = append(requests, request{key: key, text: cmd.Text})
	}

	resolved := make(map[AtlasKey]*Atlas, len(requests))
	var builtin *Atlas
	for _, request := range requests {
		atlas, err := provider.AtlasFor(request.key.Font, request.key.PX, request.text)
		if err != nil || atlas == nil {
			if builtin == nil {
				builtin, _ = provider.AtlasFor(0, glyphHeight, request.text)
				if builtin == nil {
					builtin = NewAtlas()
				}
			}
			atlas = builtin
		}
		resolved[request.key] = atlas
	}
	return resolved
}

func (s *buildSink) VisitImage(cmd render.DrawImage) {
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		if corners, ok := destCorners(s.ctx, cmd.Dest, cmd.Rotation); ok {
			appendMissingTexture(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, corners, cmd.Clip)
		}
		return
	}
	run := s.run(DrawRunTextured, cmd.Texture, AtlasKey{})
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
		appendMissingTexture(&s.run(DrawRunSolid, 0, AtlasKey{}).Solid, s.ctx, screen, cmd.Clip)
		return
	}
	run := s.run(DrawRunTextured, cmd.Texture, AtlasKey{})
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
