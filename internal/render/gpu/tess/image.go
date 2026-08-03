package tess

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// TexVertex feeds the textured pipeline: clip-space position pre-multiplied
// by w (hardware perspective-correct interpolation divides it back), tint
// and texture UV.
type TexVertex struct {
	XW, YW, W  float32
	R, G, B, A float32
	U, V       float32
}

// TexMesh accumulates triangles for a single texture.
type TexMesh struct {
	Vertices []TexVertex
	Texture  render.TextureID
	stream   *[]byte
	collect  bool
}

func (m *TexMesh) reset(stream *[]byte, collect bool) {
	m.Vertices = m.Vertices[:0]
	m.stream = stream
	m.collect = collect
}

func (m *TexMesh) append(vertices ...TexVertex) {
	if m.stream != nil && !m.collect {
		for _, v := range vertices {
			*m.stream = appendTexVertex(*m.stream, v)
		}
		return
	}
	m.Vertices = append(m.Vertices, vertices...)
}

func (m *TexMesh) flush() {
	if m.stream == nil {
		return
	}
	for _, v := range m.Vertices {
		*m.stream = appendTexVertex(*m.stream, v)
	}
	m.Vertices = m.Vertices[:0]
}

// TexSizes maps TextureID to pixel dimensions, mirroring pixel.Registry:
// index 0 is always the zero value (TextureID 0 means "missing texture").
// Src rectangles are expressed in texture pixels and need these dimensions
// to normalize into UV space.
type TexSizes [][2]int

// Lookup returns the pixel dimensions for id, or ok=false when id is unknown
// (zero, out of range, or a stored zero-size entry).
func (s TexSizes) Lookup(id render.TextureID) (w, h int, ok bool) {
	if id == 0 || int(id) >= len(s) {
		return 0, 0, false
	}
	size := s[id]
	if size[0] <= 0 || size[1] <= 0 {
		return 0, 0, false
	}
	return size[0], size[1], true
}

// AppendImage converts a render.DrawImage into two textured triangles with
// W=1 (no perspective). Rotation turns the destination rect around its own
// screen-space center, matching the raster backend.
func AppendImage(m *TexMesh, c ctx, cmd render.DrawImage, texW, texH int) {
	corners, ok := imageScreenCorners(c, cmd)
	if !ok {
		return
	}

	uv, ok := srcUV(cmd.Src, texW, texH)
	if !ok {
		return
	}
	tint := texTint(cmd.Tint)

	start := len(m.Vertices)
	pushTexQuad(m, c, corners, [4]float64{1, 1, 1, 1}, uv, tint)

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTexTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// appendSpriteRecord converts a DrawImage into the 64-byte storage-buffer
// record consumed by the instanced sprite pipeline. Camera projection and
// quad expansion deliberately stay on the GPU; only the command-local
// rotation, texture region, tint, and screen-space clip are prepared here.
func appendSpriteRecord(dst *[]spriteRecord, c ctx, cmd render.DrawImage, texW, texH int, textureLayer uint32) bool {
	// The overwhelmingly common game path uses an upright destination rect,
	// the whole texture, no clip, and the default white tint. Keep it direct:
	// rotation still travels as an angle for the shader, while no temporary
	// UV/clip vectors or color conversion are needed on the wasm hot loop.
	if cmd.Dest.Min.X < cmd.Dest.Max.X && cmd.Dest.Min.Y < cmd.Dest.Max.Y &&
		cmd.Src == (math2d.Rect{}) && cmd.Clip == (math2d.Rect{}) && cmd.Tint == (render.Color{}) {
		*dst = append(*dst, spriteRecord{
			centerX:    float32((cmd.Dest.Min.X + cmd.Dest.Max.X) * 0.5),
			centerY:    float32((cmd.Dest.Min.Y + cmd.Dest.Max.Y) * 0.5),
			halfWidth:  float32((cmd.Dest.Max.X - cmd.Dest.Min.X) * 0.5),
			halfHeight: float32((cmd.Dest.Max.Y - cmd.Dest.Min.Y) * 0.5),
			rotation:   float32(cmd.Rotation),
			uvMaxX:     1, uvMaxY: 1,
			clipMaxX: float32(c.w), clipMaxY: float32(c.h),
			tint: 0xffffffff, textureLayer: textureLayer,
		})
		return true
	}

	minX, maxX := cmd.Dest.Min.X, cmd.Dest.Max.X
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := cmd.Dest.Min.Y, cmd.Dest.Max.Y
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	if maxX <= minX || maxY <= minY {
		return false
	}
	uvMin := [2]float32{}
	uvMax := [2]float32{1, 1}
	if cmd.Src != (math2d.Rect{}) {
		uv, ok := srcUV(cmd.Src, texW, texH)
		if !ok {
			return false
		}
		uvMin, uvMax = uv[0], uv[2]
	}

	clipMin := math2d.Vec2{}
	clipMax := math2d.Vec2{X: c.w, Y: c.h}
	if cmd.Clip != (math2d.Rect{}) {
		if cmd.Clip.Width() <= 0 || cmd.Clip.Height() <= 0 {
			return false
		}
		a := c.cam.WorldToScreen(cmd.Clip.Min)
		b := c.cam.WorldToScreen(cmd.Clip.Max)
		clipMin = math2d.Vec2{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y)}
		clipMax = math2d.Vec2{X: math.Max(a.X, b.X), Y: math.Max(a.Y, b.Y)}
	}

	tint := packSpriteTint(cmd.Tint)
	*dst = appendSprite(
		*dst,
		float32((minX+maxX)*0.5), float32((minY+maxY)*0.5),
		float32((maxX-minX)*0.5), float32((maxY-minY)*0.5),
		float32(cmd.Rotation),
		uvMin, uvMax,
		[2]float32{float32(clipMin.X), float32(clipMin.Y)},
		[2]float32{float32(clipMax.X), float32(clipMax.Y)},
		tint,
		textureLayer,
	)
	return true
}

func packSpriteTint(tint render.Color) uint32 {
	if tint == (render.Color{}) {
		return 0xffffffff
	}
	return uint32(spriteColorByte(tint.R*tint.A)) |
		uint32(spriteColorByte(tint.G*tint.A))<<8 |
		uint32(spriteColorByte(tint.B*tint.A))<<16 |
		uint32(spriteColorByte(tint.A))<<24
}

func spriteColorByte(value float64) byte {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 255
	}
	return byte(value*255 + 0.5)
}

// AppendImageQuad converts a render.DrawImageQuad into two textured
// triangles using a projective homography that maps the unit square onto the
// screen-space quad corners. W varies per corner when the quad has genuine
// perspective, so the vertex position is pre-multiplied by w and hardware
// perspective-correct interpolation divides it back in the pipeline.
func AppendImageQuad(m *TexMesh, c ctx, cmd render.DrawImageQuad, texW, texH int) {
	_, homography, ok := imageQuadProjection(c, transformedCorners(cmd.Transform, cmd.Corners))
	if !ok {
		return
	}

	unit := [4]math2d.Vec2{
		{X: 0, Y: 0}, {X: 1, Y: 0},
		{X: 1, Y: 1}, {X: 0, Y: 1},
	}

	var projected [4]math2d.Vec2
	var w [4]float64
	for i, p := range unit {
		xy, ww := homography.ApplyW(p)
		if ww == 0 {
			return
		}
		projected[i] = math2d.Vec2{X: xy.X / ww, Y: xy.Y / ww}
		w[i] = ww
	}

	uv, ok := srcUV(cmd.Src, texW, texH)
	if !ok {
		return
	}
	tint := texTint(cmd.Tint)

	start := len(m.Vertices)
	pushTexQuad(m, c, projected, w, uv, tint)

	// Sutherland-Hodgman clipping interpolates vertex attributes linearly in
	// screen space. That is exact only when w is constant across the
	// primitive (the affine case). A genuinely projective quad has UV that
	// varies non-linearly in screen space, so clipping it here with linear
	// interpolation would visibly distort the texture near the clip
	// boundary. Clip only when the homography is affine; otherwise the quad
	// is drawn unclipped.
	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 && homography.IsAffine() {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTexTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// imageQuadProjection converts a world-space quad to screen space and
// validates the command contract shared by all render backends. A
// self-intersecting or collinear quad is skipped even when a mathematical
// homography could be formed for it.
func imageQuadProjection(c ctx, corners [4]math2d.Vec2) ([4]math2d.Vec2, math2d.Homography, bool) {
	var screen [4]math2d.Vec2
	for i, corner := range corners {
		screen[i] = c.cam.WorldToScreen(corner)
	}
	if !math2d.IsConvexQuad(screen) {
		return screen, math2d.Homography{}, false
	}
	h, ok := math2d.HomographyFromQuad(screen)
	return screen, h, ok
}

// imageScreenCorners returns a DrawImage's four screen-space corners, honoring
// a non-identity BaseCommand.Transform. The identity path stays on destCorners
// so the common (and instanced) sprite geometry is byte-identical.
func imageScreenCorners(c ctx, cmd render.DrawImage) ([4]math2d.Vec2, bool) {
	if !c.perVertex(cmd.Transform) {
		return destCorners(c, cmd.Dest, cmd.Rotation)
	}
	return transformedImageCorners(c, cmd.Dest, cmd.Rotation, cmd.Transform)
}

// transformedImageCorners projects the destination rect's four world-space
// corners through the command transform and the camera. Rotation, when set, is
// applied afterward as a screen-space spin around the transformed center,
// matching destCorners' rotation convention.
func transformedImageCorners(c ctx, dest math2d.Rect, rotation float64, transform math2d.Affine) (corners [4]math2d.Vec2, ok bool) {
	world := [4]math2d.Vec2{
		{X: dest.Min.X, Y: dest.Min.Y},
		{X: dest.Max.X, Y: dest.Min.Y},
		{X: dest.Max.X, Y: dest.Max.Y},
		{X: dest.Min.X, Y: dest.Max.Y},
	}
	for i, w := range world {
		corners[i] = c.cam.WorldToScreen(transform.Apply(w))
	}

	// Shoelace area: zero means the transform collapsed the rect to a line or
	// point, which has nothing to draw.
	area := 0.0
	for i := range corners {
		j := (i + 1) % len(corners)
		area += corners[i].X*corners[j].Y - corners[j].X*corners[i].Y
	}
	if area == 0 {
		return corners, false
	}

	if rotation != 0 {
		center := math2d.Vec2{
			X: (corners[0].X + corners[1].X + corners[2].X + corners[3].X) / 4,
			Y: (corners[0].Y + corners[1].Y + corners[2].Y + corners[3].Y) / 4,
		}
		sin, cos := math.Sincos(rotation)
		for i, p := range corners {
			dx := p.X - center.X
			dy := p.Y - center.Y
			corners[i] = math2d.Vec2{X: center.X + dx*cos - dy*sin, Y: center.Y + dx*sin + dy*cos}
		}
	}
	return corners, true
}

// transformedCorners applies a command transform to a DrawImageQuad's four
// world-space corners before projection. The identity case returns the corners
// unchanged (and un-copied semantics are irrelevant: the array is passed by
// value), keeping existing quads byte-identical.
func transformedCorners(transform math2d.Affine, corners [4]math2d.Vec2) [4]math2d.Vec2 {
	if transform.IsIdentity() {
		return corners
	}
	for i := range corners {
		corners[i] = transform.Apply(corners[i])
	}
	return corners
}

// destCorners projects a DrawImage-style destination rect from world space
// into four screen-space corners (TL, TR, BR, BL), rotating them around the
// rect's own screen-space center when rotation is non-zero. ok is false when
// the projected rect has zero or negative area.
func destCorners(c ctx, dest math2d.Rect, rotation float64) (corners [4]math2d.Vec2, ok bool) {
	rMin := c.cam.WorldToScreen(dest.Min)
	rMax := c.cam.WorldToScreen(dest.Max)

	x0 := math.Min(rMin.X, rMax.X)
	y0 := math.Min(rMin.Y, rMax.Y)
	x1 := math.Max(rMin.X, rMax.X)
	y1 := math.Max(rMin.Y, rMax.Y)
	if x1-x0 <= 0 || y1-y0 <= 0 {
		return corners, false
	}

	corners = [4]math2d.Vec2{
		{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1},
	}
	if rotation == 0 {
		return corners, true
	}

	center := math2d.Vec2{X: (x0 + x1) / 2, Y: (y0 + y1) / 2}
	sin, cos := math.Sincos(rotation)
	for i, p := range corners {
		dx := p.X - center.X
		dy := p.Y - center.Y
		corners[i] = math2d.Vec2{
			X: center.X + dx*cos - dy*sin,
			Y: center.Y + dx*sin + dy*cos,
		}
	}
	return corners, true
}

// srcUV normalizes a pixel-space sub-rectangle into texture UV. The zero
// rect (or non-positive texture dimensions) selects the whole texture.
//
// Non-zero Src is clamped to the texture bounds before normalizing, mirroring
// the raster backend's tex.Bounds() intersection (platform/raster/renderer.go
// drawImage/drawImageQuad): without this, a Src that overshoots the texture
// produces UV outside [0,1], and clamp-to-edge sampling stretches the border
// instead of shrinking the sampled sub-rect like raster does. ok is false
// when the clamped rect has zero or negative area, matching raster's
// intersect-then-skip behavior for a Src wholly outside the texture.
func srcUV(src math2d.Rect, texW, texH int) (uv [4][2]float32, ok bool) {
	if src == (math2d.Rect{}) || texW <= 0 || texH <= 0 {
		return [4][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}, true
	}

	w, h := float64(texW), float64(texH)
	minX := math.Max(0, math.Min(w, src.Min.X))
	minY := math.Max(0, math.Min(h, src.Min.Y))
	maxX := math.Max(0, math.Min(w, src.Max.X))
	maxY := math.Max(0, math.Min(h, src.Max.Y))
	if maxX <= minX || maxY <= minY {
		return [4][2]float32{}, false
	}

	u0 := float32(minX / w)
	v0 := float32(minY / h)
	u1 := float32(maxX / w)
	v1 := float32(maxY / h)
	return [4][2]float32{{u0, v0}, {u1, v0}, {u1, v1}, {u0, v1}}, true
}

// texTint mirrors the raster backend's tintFrom: the zero Color means no
// tint, and a non-zero tint is premultiplied by its own alpha to match the
// premultiplied pixels produced by asset/pixel.
func texTint(c render.Color) [4]float32 {
	if c == (render.Color{}) {
		return [4]float32{1, 1, 1, 1}
	}
	return [4]float32{
		float32(c.R * c.A),
		float32(c.G * c.A),
		float32(c.B * c.A),
		float32(c.A),
	}
}

// pushTexQuad emits two triangles (0,1,2) and (0,2,3) for a screen-space
// quad, converting each corner to clip space and pre-multiplying by its own
// w (1 for AppendImage, the homography's per-corner w for AppendImageQuad).
func pushTexQuad(m *TexMesh, c ctx, screen [4]math2d.Vec2, w [4]float64, uv [4][2]float32, tint [4]float32) {
	var v [4]TexVertex
	for i := range screen {
		clip := screenToClip(c, screen[i])
		v[i] = TexVertex{
			XW: float32(float64(clip[0]) * w[i]),
			YW: float32(float64(clip[1]) * w[i]),
			W:  float32(w[i]),
			R:  tint[0], G: tint[1], B: tint[2], A: tint[3],
			U: uv[i][0], V: uv[i][1],
		}
	}
	m.append(v[0], v[1], v[2], v[0], v[2], v[3])
}
