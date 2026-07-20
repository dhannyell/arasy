package tess

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// StreamRun identifies one ordered draw in Builder's shared vertex streams.
// ByteOffset is relative to the stream selected by Kind.
type StreamRun struct {
	Kind        DrawRunKind
	Texture     render.TextureID
	AtlasKey    AtlasKey
	Particles   render.DrawParticles
	ByteOffset  int
	VertexCount int
}

// Builder tessellates frames into per-pipeline byte streams reused across
// frames. Build overwrites the previous frame's streams and returned runs.
type Builder struct {
	solid []byte
	tex   []byte
	text  []byte
	runs  []StreamRun

	solidMesh Mesh
	texMesh   TexMesh
	textMesh  TextMesh
}

// Build tessellates a frame into this builder's reusable streams.
func (b *Builder) Build(frame runtime.Frame, w, h int, provider AtlasProvider, sizes TexSizes) []StreamRun {
	b.solid = b.solid[:0]
	b.tex = b.tex[:0]
	b.text = b.text[:0]
	b.runs = b.runs[:0]

	buildCtx := ctx{cam: frame.Camera, w: float64(w), h: float64(h)}
	sink := streamSink{
		builder: b,
		ctx:     buildCtx,
		atlases: resolveFrameAtlases(frame.Commands, buildCtx, provider),
		sizes:   sizes,
	}
	render.Each(frame.Commands, &sink)
	return b.runs
}

// Streams returns the vertex data built by the most recent Build.
func (b *Builder) Streams() (solid, tex, text []byte) {
	return b.solid, b.tex, b.text
}

type streamSink struct {
	builder *Builder
	ctx     ctx
	atlases map[AtlasKey]*Atlas
	sizes   TexSizes

	lastKind    DrawRunKind
	lastTexture render.TextureID
	lastAtlas   AtlasKey
	hasLast     bool
}

func (s *streamSink) run(kind DrawRunKind, texture render.TextureID, atlas AtlasKey) *StreamRun {
	if s.hasLast && s.lastKind == kind &&
		(kind != DrawRunTextured || s.lastTexture == texture) &&
		(kind != DrawRunText || s.lastAtlas == atlas) {
		return &s.builder.runs[len(s.builder.runs)-1]
	}

	byteOffset := 0
	switch kind {
	case DrawRunSolid:
		byteOffset = len(s.builder.solid)
	case DrawRunTextured:
		byteOffset = len(s.builder.tex)
	case DrawRunText:
		byteOffset = len(s.builder.text)
	}
	s.builder.runs = append(s.builder.runs, StreamRun{Kind: kind, Texture: texture, AtlasKey: atlas, ByteOffset: byteOffset})
	s.lastKind = kind
	s.lastTexture = texture
	s.lastAtlas = atlas
	s.hasLast = true
	return &s.builder.runs[len(s.builder.runs)-1]
}

func (s *streamSink) solidBegin(clip math2d.Rect) int {
	s.builder.solidMesh.reset(&s.builder.solid, clip.Width() > 0 && clip.Height() > 0)
	return len(s.builder.solid)
}

func (s *streamSink) solidEnd(run *StreamRun, start int) {
	s.builder.solidMesh.flush()
	run.VertexCount += (len(s.builder.solid) - start) / SolidVertexStride
}

func (s *streamSink) texBegin(clip math2d.Rect) int {
	s.builder.texMesh.reset(&s.builder.tex, clip.Width() > 0 && clip.Height() > 0)
	return len(s.builder.tex)
}

func (s *streamSink) texEnd(run *StreamRun, start int) {
	s.builder.texMesh.flush()
	run.VertexCount += (len(s.builder.tex) - start) / TexVertexStride
}

func (s *streamSink) textBegin(clip math2d.Rect) int {
	s.builder.textMesh.reset(&s.builder.text, clip.Width() > 0 && clip.Height() > 0)
	return len(s.builder.text)
}

func (s *streamSink) textEnd(run *StreamRun, start int) {
	s.builder.textMesh.flush()
	run.VertexCount += (len(s.builder.text) - start) / TextVertexStride
}

func (s *streamSink) VisitRect(cmd render.DrawRect) {
	run := s.run(DrawRunSolid, 0, AtlasKey{})
	start := s.solidBegin(cmd.Clip)
	AppendRect(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(run, start)
}

func (s *streamSink) VisitCircle(cmd render.DrawCircle) {
	run := s.run(DrawRunSolid, 0, AtlasKey{})
	start := s.solidBegin(cmd.Clip)
	AppendCircle(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(run, start)
}

func (s *streamSink) VisitPoint(cmd render.DrawPoint) {
	run := s.run(DrawRunSolid, 0, AtlasKey{})
	start := s.solidBegin(cmd.Clip)
	AppendPoint(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(run, start)
}

func (s *streamSink) VisitLine(cmd render.DrawLine) {
	run := s.run(DrawRunSolid, 0, AtlasKey{})
	start := s.solidBegin(cmd.Clip)
	AppendLine(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(run, start)
}

func (s *streamSink) VisitArc(cmd render.DrawArc) {
	run := s.run(DrawRunSolid, 0, AtlasKey{})
	start := s.solidBegin(cmd.Clip)
	AppendArc(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(run, start)
}

func (s *streamSink) VisitPolygon(cmd render.DrawPolygon) {
	run := s.run(DrawRunSolid, 0, AtlasKey{})
	start := s.solidBegin(cmd.Clip)
	AppendPolygon(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(run, start)
}

func (s *streamSink) VisitParticles(cmd render.DrawParticles) {
	s.builder.runs = append(s.builder.runs, StreamRun{Kind: DrawRunParticles, Particles: cmd})
	s.hasLast = false
}

func (s *streamSink) VisitText(cmd render.DrawText) {
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
	run := s.run(DrawRunText, 0, atlas.Key)
	start := s.textBegin(cmd.Clip)
	AppendText(&s.builder.textMesh, s.ctx, cmd, atlas)
	s.textEnd(run, start)
}

func (s *streamSink) VisitImage(cmd render.DrawImage) {
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		if corners, ok := destCorners(s.ctx, cmd.Dest, cmd.Rotation); ok {
			run := s.run(DrawRunSolid, 0, AtlasKey{})
			start := s.solidBegin(cmd.Clip)
			appendMissingTexture(&s.builder.solidMesh, s.ctx, corners, cmd.Clip)
			s.solidEnd(run, start)
		}
		return
	}
	run := s.run(DrawRunTextured, cmd.Texture, AtlasKey{})
	start := s.texBegin(cmd.Clip)
	AppendImage(&s.builder.texMesh, s.ctx, cmd, texW, texH)
	s.texEnd(run, start)
}

func (s *streamSink) VisitImageQuad(cmd render.DrawImageQuad) {
	screen, _, valid := imageQuadProjection(s.ctx, cmd.Corners)
	if !valid {
		return
	}
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		run := s.run(DrawRunSolid, 0, AtlasKey{})
		start := s.solidBegin(cmd.Clip)
		appendMissingTexture(&s.builder.solidMesh, s.ctx, screen, cmd.Clip)
		s.solidEnd(run, start)
		return
	}
	run := s.run(DrawRunTextured, cmd.Texture, AtlasKey{})
	start := s.texBegin(cmd.Clip)
	AppendImageQuad(&s.builder.texMesh, s.ctx, cmd, texW, texH)
	s.texEnd(run, start)
}
