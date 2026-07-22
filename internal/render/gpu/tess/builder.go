package tess

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Frame is the render-only input to Build: camera plus the painter-ordered
// commands of one frame. Identical in layout to runtime.Frame so hosts convert
// with a plain tess.Frame(rtFrame); the core must not import the orchestration
// layer.
type Frame struct {
	Camera   render.Camera
	Commands []render.Command
}

// StreamRun identifies one ordered draw in Builder's shared streams.
// ByteOffset is relative to the stream selected by Kind. VertexCount is a
// vertex count for mesh streams and a record count for DrawRunSprites and
// DrawRunText.
type StreamRun struct {
	Kind        DrawRunKind
	Texture     render.TextureID
	AtlasKey    AtlasKey
	Particles   render.DrawParticles
	SpriteBatch render.DrawSpriteBatch
	TextBatch   render.DrawTextBatch
	ByteOffset  int
	VertexCount int
}

// Builder tessellates frames into per-pipeline byte streams reused across
// frames. Build overwrites the previous frame's streams and returned runs.
type Builder struct {
	solid   []byte
	tex     []byte
	sprites []spriteRecord
	glyphs  []textGlyphRecord
	labels  []textLabelRecord
	runs    []StreamRun

	solidMesh Mesh
	texMesh   TexMesh

	// spriteArray selects the array-texture sprite path. It permits adjacent
	// DrawImage commands with different textures to share one draw run because
	// the texture layer is carried by each storage record.
	spriteArray bool
}

// SetSpriteArray selects whether sprite records carry array-texture layers.
// Renderers enable it only when their declared textures fit one GPU texture
// array; the default preserves the individual-texture fallback.
func (b *Builder) SetSpriteArray(enabled bool) {
	b.spriteArray = enabled
}

// Build tessellates a frame into this builder's reusable streams.
func (b *Builder) Build(frame Frame, w, h int, provider AtlasProvider, sizes TexSizes) []StreamRun {
	b.solid = b.solid[:0]
	b.tex = b.tex[:0]
	b.sprites = b.sprites[:0]
	b.glyphs = b.glyphs[:0]
	b.labels = b.labels[:0]
	b.runs = b.runs[:0]

	buildCtx := ctx{cam: frame.Camera, w: float64(w), h: float64(h)}
	// The built-in Atlas is immutable and covers every rune it can render, so
	// it needs neither per-frame glyph discovery nor an AtlasKey map. Keeping
	// this path separate avoids doing a second, rune-by-rune pass over HUD
	// labels before emitting their instance records.
	var atlases map[AtlasKey]*Atlas
	ordered := true
	var fallback *Atlas
	if atlas, ok := provider.(*Atlas); ok {
		fallback = atlas
		for i := 1; i < len(frame.Commands); i++ {
			if frame.Commands[i].Layer() < frame.Commands[i-1].Layer() {
				ordered = false
				break
			}
		}
	} else {
		atlases, ordered = resolveFrameAtlases(frame.Commands, buildCtx, provider)
	}
	sink := streamSink{
		builder:  b,
		ctx:      buildCtx,
		atlases:  atlases,
		fallback: fallback,
		sizes:    sizes,
	}
	if ordered {
		render.EachOrdered(frame.Commands, &sink)
	} else {
		render.Each(frame.Commands, &sink)
	}
	return b.runs
}

// Streams returns the mesh vertex data built by the most recent Build.
func (b *Builder) Streams() (solid, tex []byte) {
	return b.solid, b.tex
}

// SpriteStream returns the sprite storage records built by the most recent
// Build. Each record occupies SpriteStride bytes.
func (b *Builder) SpriteStream() []byte {
	return spriteBytes(b.sprites)
}

// TextGlyphStream returns the storage records built for DrawText commands.
// Each record occupies TextGlyphStride bytes.
func (b *Builder) TextGlyphStream() []byte {
	return textGlyphBytes(b.glyphs)
}

// TextLabelStream returns one shared transform/clip/tint record for every
// visible DrawText command in the most recent Build.
func (b *Builder) TextLabelStream() []byte {
	return textLabelBytes(b.labels)
}

type streamSink struct {
	builder  *Builder
	ctx      ctx
	atlases  map[AtlasKey]*Atlas
	fallback *Atlas
	sizes    TexSizes

	lastRun streamRunIdentity
	hasLast bool
}

type streamRunIdentity struct {
	kind    DrawRunKind
	texture render.TextureID
	atlas   AtlasKey
}

type frameAtlasRequest struct {
	key    AtlasKey
	cached *Atlas
	runes  []rune
	ascii  [128]bool
	other  map[rune]struct{}
}

func (r *frameAtlasRequest) add(text string) {
	if r.cached != nil && printableASCII(text) {
		return
	}
	for _, char := range text {
		if r.cached != nil && r.cached.covers(char) {
			continue
		}
		if char < 128 {
			if r.ascii[char] {
				continue
			}
			r.ascii[char] = true
		} else {
			if r.other == nil {
				r.other = make(map[rune]struct{})
			}
			if _, exists := r.other[char]; exists {
				continue
			}
			r.other[char] = struct{}{}
		}
		r.runes = append(r.runes, char)
	}
}

func resolveFrameAtlases(commands []render.Command, c ctx, provider AtlasProvider) (map[AtlasKey]*Atlas, bool) {
	ordered := true
	if provider == nil {
		return nil, false
	}
	var requests []frameAtlasRequest
	indices := make(map[AtlasKey]int)
	cache, hasCache := provider.(CachedAtlasProvider)
	for commandIndex, command := range commands {
		if commandIndex > 0 && command.Layer() < commands[commandIndex-1].Layer() {
			ordered = false
		}
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
		requestIndex, exists := indices[key]
		if !exists {
			requestIndex = len(requests)
			indices[key] = requestIndex
			request := frameAtlasRequest{key: key}
			if hasCache {
				request.cached, _ = cache.CachedAtlas(key.Font, key.PX)
			}
			requests = append(requests, request)
		}
		requests[requestIndex].add(cmd.Text)
	}
	if len(requests) == 0 {
		return nil, ordered
	}

	resolved := make(map[AtlasKey]*Atlas, len(requests))
	var builtin *Atlas
	for requestIndex := range requests {
		request := &requests[requestIndex]
		if request.cached != nil && len(request.runes) == 0 {
			resolved[request.key] = request.cached
			continue
		}
		text := string(request.runes)
		atlas, err := provider.AtlasFor(request.key.Font, request.key.PX, text)
		if err != nil || atlas == nil {
			if builtin == nil {
				builtin, _ = provider.AtlasFor(0, glyphHeight, text)
				if builtin == nil {
					builtin = NewAtlas()
				}
			}
			atlas = builtin
		}
		resolved[request.key] = atlas
	}
	return resolved, ordered
}

// appendRun is the only place that decides whether adjacent stream content
// can share a draw. Its narrow parameters avoid copying StreamRun's particle
// payload for every ordinary mesh, sprite, or text command.
func (s *streamSink) appendRun(kind DrawRunKind, texture render.TextureID, atlas AtlasKey, byteOffset, vertexCount int) {
	identity := streamRunIdentity{kind: kind, texture: texture, atlas: atlas}
	if s.hasLast && s.lastRun == identity {
		s.builder.runs[len(s.builder.runs)-1].VertexCount += vertexCount
		return
	}
	s.startRun(identity, texture, atlas, byteOffset, vertexCount)
}

// startRun keeps the allocation-heavy cold path out of appendRun's merge
// path, which executes once per command in large sprite and text streams.
func (s *streamSink) startRun(identity streamRunIdentity, texture render.TextureID, atlas AtlasKey, byteOffset, vertexCount int) {
	s.builder.runs = append(s.builder.runs, StreamRun{
		Kind: identity.kind, Texture: texture, AtlasKey: atlas,
		ByteOffset: byteOffset, VertexCount: vertexCount,
	})
	s.lastRun = identity
	s.hasLast = true
}

func (s *streamSink) solidBegin(clip math2d.Rect) int {
	s.builder.solidMesh.reset(&s.builder.solid, clip.Width() > 0 && clip.Height() > 0)
	return len(s.builder.solid)
}

func (s *streamSink) solidEnd(start int) {
	s.builder.solidMesh.flush()
	count := (len(s.builder.solid) - start) / SolidVertexStride
	if count > 0 {
		s.appendRun(DrawRunSolid, 0, AtlasKey{}, start, count)
	}
}

func (s *streamSink) texBegin(clip math2d.Rect) int {
	s.builder.texMesh.reset(&s.builder.tex, clip.Width() > 0 && clip.Height() > 0)
	return len(s.builder.tex)
}

func (s *streamSink) texEnd(texture render.TextureID, start int) {
	s.builder.texMesh.flush()
	count := (len(s.builder.tex) - start) / TexVertexStride
	if count > 0 {
		s.appendRun(DrawRunTextured, texture, AtlasKey{}, start, count)
	}
}

func (s *streamSink) VisitRect(cmd render.DrawRect) {
	start := s.solidBegin(cmd.Clip)
	AppendRect(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitCircle(cmd render.DrawCircle) {
	start := s.solidBegin(cmd.Clip)
	AppendCircle(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitEllipse(cmd render.DrawEllipse) {
	start := s.solidBegin(cmd.Clip)
	AppendEllipse(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitPoint(cmd render.DrawPoint) {
	start := s.solidBegin(cmd.Clip)
	AppendPoint(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitLine(cmd render.DrawLine) {
	start := s.solidBegin(cmd.Clip)
	AppendLine(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitArc(cmd render.DrawArc) {
	start := s.solidBegin(cmd.Clip)
	AppendArc(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitPolygon(cmd render.DrawPolygon) {
	start := s.solidBegin(cmd.Clip)
	AppendPolygon(&s.builder.solidMesh, s.ctx, cmd)
	s.solidEnd(start)
}

func (s *streamSink) VisitParticles(cmd render.DrawParticles) {
	s.builder.runs = append(s.builder.runs, StreamRun{Kind: DrawRunParticles, Particles: cmd})
	s.hasLast = false
}

func (s *streamSink) VisitSpriteBatch(cmd render.DrawSpriteBatch) {
	// A retained batch owns GPU state and breaks painter-order merging, like
	// a particle command: one command per run, no instance payload.
	s.builder.runs = append(s.builder.runs, StreamRun{Kind: DrawRunSpriteBatch, SpriteBatch: cmd})
	s.hasLast = false
}

func (s *streamSink) VisitTextBatch(cmd render.DrawTextBatch) {
	// A retained text batch owns GPU state and breaks painter-order merging,
	// like a particle/sprite-batch command: one command per run, no payload.
	s.builder.runs = append(s.builder.runs, StreamRun{Kind: DrawRunTextBatch, TextBatch: cmd})
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
	atlas := s.fallback
	if atlas == nil {
		atlas = s.atlases[AtlasKey{Font: cmd.Font, PX: px}]
	}
	if atlas == nil {
		return
	}
	start := len(s.builder.glyphs) * TextGlyphStride
	count := appendTextGlyphRecords(&s.builder.glyphs, &s.builder.labels, s.ctx, cmd, atlas)
	if count == 0 {
		return
	}
	s.appendRun(DrawRunText, 0, atlas.Key, start, count)
}

func (s *streamSink) VisitImage(cmd render.DrawImage) {
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		if corners, ok := imageScreenCorners(s.ctx, cmd); ok {
			start := s.solidBegin(cmd.Clip)
			appendMissingTexture(&s.builder.solidMesh, s.ctx, corners, cmd.Clip)
			s.solidEnd(start)
		}
		return
	}
	if !cmd.Transform.IsIdentity() {
		// A transformed sprite leaves the instanced fast path (whose record
		// carries no per-sprite matrix) and draws as an explicit textured quad,
		// so rotation/shear/scale are honored like the raster backend.
		start := s.texBegin(cmd.Clip)
		AppendImage(&s.builder.texMesh, s.ctx, cmd, texW, texH)
		s.texEnd(cmd.Texture, start)
		return
	}
	start := len(s.builder.sprites) * SpriteStride
	if !appendSpriteRecord(&s.builder.sprites, s.ctx, cmd, texW, texH, uint32(cmd.Texture-1)) {
		return
	}
	runTexture := cmd.Texture
	if s.builder.spriteArray {
		runTexture = 0
	}
	s.appendRun(DrawRunSprites, runTexture, AtlasKey{}, start, 1)
}

func (s *streamSink) VisitImageQuad(cmd render.DrawImageQuad) {
	screen, _, valid := imageQuadProjection(s.ctx, transformedCorners(cmd.Transform, cmd.Corners))
	if !valid {
		return
	}
	texW, texH, ok := s.sizes.Lookup(cmd.Texture)
	if !ok {
		start := s.solidBegin(cmd.Clip)
		appendMissingTexture(&s.builder.solidMesh, s.ctx, screen, cmd.Clip)
		s.solidEnd(start)
		return
	}
	start := s.texBegin(cmd.Clip)
	AppendImageQuad(&s.builder.texMesh, s.ctx, cmd, texW, texH)
	s.texEnd(cmd.Texture, start)
}
