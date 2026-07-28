package tess

import (
	"math"
	"unicode/utf8"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// AppendText tessellates a DrawText command into textured triangles.
//
// Text quads use the built-in atlas UVs while position, alignment, baseline,
// camera scaling, rotation, and clipping follow the raster backend's DrawText
// semantics.
func AppendText(m *TextMesh, c ctx, cmd render.DrawText, atlas *Atlas) {
	if atlas == nil || cmd.Text == "" || cmd.Color.A <= 0 {
		return
	}

	start := len(m.Vertices)

	nativeHeight := atlas.Height
	if nativeHeight <= 0 {
		return
	}
	nativeAscent := atlas.Ascent
	scale := 1.0
	builtin := atlas.Key.Font == 0
	var measured math2d.Vec2
	if builtin {
		size := cmd.Size
		if size <= 0 {
			size = nativeHeight
		}
		measured = render.MeasureTextSpaced(cmd.Text, size, cmd.LetterSpacing)
		scale = c.cam.WorldScalarToScreen(measured.Y) / nativeHeight
	} else if atlas.Key.PX > 0 {
		scale = requestedTextPixelSize(c, cmd.Size) / float64(atlas.Key.PX)
	}

	glyphs := m.glyphScratch[:0]
	destWidth := 0.0
	for _, r := range cmd.Text {
		glyph, ok := atlas.Glyphs[r]
		if !ok {
			// The zero Glyph fallback keeps the rune's slot (advance 0, no
			// quad), matching the previous behavior when '?' is also missing.
			glyph = atlas.Glyphs['?']
		}
		glyphs = append(glyphs, glyph)
		destWidth += glyph.Advance * scale
	}
	glyphCount := utf8.RuneCountInString(cmd.Text)
	spacing := c.cam.WorldScalarToScreen(cmd.LetterSpacing)
	if !builtin && glyphCount > 1 {
		destWidth += float64(glyphCount-1) * spacing
	}
	m.glyphScratch = glyphs
	destHeight := nativeHeight * scale
	if builtin {
		destWidth = c.cam.WorldScalarToScreen(measured.X)
		destHeight = c.cam.WorldScalarToScreen(measured.Y)
	}
	if destWidth <= 0 || destHeight <= 0 {
		return
	}

	clipMin, clipMax := textClip(c, cmd)
	transform := textScreenTransform(c.cam, cmd)
	topY := textTopY(cmd.Baseline, destHeight, nativeAscent, nativeHeight)
	layout := textLabelLayout{
		position: transform.position,
		originX:  textOriginX(cmd.Align, destWidth),
		topY:     topY, baselineY: topY + nativeAscent*scale,
		scale: scale, clipMin: clipMin, clipMax: clipMax,
		a: transform.a, b: transform.b, cc: transform.c, d: transform.d,
	}
	baseline := layout.position.Y + layout.baselineY
	penX := layout.position.X + layout.originX
	for index, glyph := range glyphs {
		if glyph.Width <= 0 || glyph.Height <= 0 {
			penX += glyph.Advance * scale
			if index+1 < len(glyphs) {
				penX += spacing
			}
			continue
		}

		x0 := penX + glyph.OffsetX*scale
		y0 := baseline + glyph.OffsetY*scale
		x1 := x0 + glyph.Width*scale
		y1 := y0 + glyph.Height*scale
		advance := glyph.Advance * scale
		if builtin {
			advance = destWidth / float64(len(glyphs))
			x0 = penX
			y0 = layout.position.Y + layout.topY
			x1 = x0 + advance
			y1 = y0 + destHeight
		}

		a := math2d.Vec2{X: x0, Y: y0}
		b := math2d.Vec2{X: x1, Y: y0}
		cc := math2d.Vec2{X: x1, Y: y1}
		d := math2d.Vec2{X: x0, Y: y1}

		a = layout.rotate(a)
		b = layout.rotate(b)
		cc = layout.rotate(cc)
		d = layout.rotate(d)

		pushTextQuad(m, c, a, b, cc, d, glyph.UV, cmd.Color)
		penX += advance
		if index+1 < len(glyphs) {
			penX += spacing
		}
	}

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := screenClipRect(c, layout.clipMin, layout.clipMax)
		clipped := clipTextTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
}

// appendTextGlyphRecords retains the established Go-side text layout while
// emitting one storage record per visible glyph. The vertex shader expands
// the glyph quad and applies the label rotation around screenPos.
func appendTextGlyphRecords(dst *[]textGlyphRecord, labels *[]textLabelRecord, c ctx, cmd render.DrawText, atlas *Atlas) int {
	if atlas == nil || cmd.Text == "" || cmd.Color.A <= 0 {
		return 0
	}

	nativeHeight := atlas.Height
	if nativeHeight <= 0 {
		return 0
	}
	nativeAscent := atlas.Ascent
	scale := 1.0
	builtin := atlas.Key.Font == 0
	start := len(*dst)
	labelIndex := uint32(len(*labels))
	destWidth := 0.0
	destHeight := nativeHeight
	if builtin {
		size := cmd.Size
		if size <= 0 {
			size = nativeHeight
		}
		measured := render.MeasureTextSpaced(cmd.Text, size, cmd.LetterSpacing)
		glyphCount := utf8.RuneCountInString(cmd.Text)
		if glyphCount == 0 {
			return 0
		}
		destWidth = c.cam.WorldScalarToScreen(measured.X)
		destHeight = c.cam.WorldScalarToScreen(measured.Y)
		scale = destHeight / nativeHeight
		advance := destWidth / float64(glyphCount)
		spacing := c.cam.WorldScalarToScreen(cmd.LetterSpacing)
		advance -= spacing * float64(glyphCount-1) / float64(glyphCount)
		penX := 0.0
		index := 0
		for _, r := range cmd.Text {
			glyph := atlas.glyph(r)
			if glyph.Width <= 0 || glyph.Height <= 0 {
				penX += advance
				continue
			}
			*dst = append(*dst, textGlyphRecord{
				penX: float32(penX), metric: glyph.index, label: labelIndex,
			})
			penX += advance
			if index+1 < glyphCount {
				penX += spacing
			}
			index++
		}
	} else {
		if atlas.Key.PX > 0 {
			scale = requestedTextPixelSize(c, cmd.Size) / float64(atlas.Key.PX)
		}
		spacing := c.cam.WorldScalarToScreen(cmd.LetterSpacing)
		ascii := true
		for i := 0; i < len(cmd.Text); i++ {
			if cmd.Text[i] < atlasFirstRune || cmd.Text[i] > atlasLastRune {
				ascii = false
				break
			}
			glyph := atlas.asciiGlyphs[cmd.Text[i]]
			if glyph.Width > 0 && glyph.Height > 0 {
				*dst = append(*dst, textGlyphRecord{
					penX: float32(destWidth), metric: glyph.index, label: labelIndex,
				})
			}
			destWidth += glyph.Advance * scale
			if i+1 < len(cmd.Text) {
				destWidth += spacing
			}
		}
		if !ascii {
			*dst = (*dst)[:start]
			destWidth = 0
			runeCount := utf8.RuneCountInString(cmd.Text)
			index := 0
			for _, r := range cmd.Text {
				glyph := atlas.glyph(r)
				if glyph.Width > 0 && glyph.Height > 0 {
					*dst = append(*dst, textGlyphRecord{
						penX: float32(destWidth), metric: glyph.index, label: labelIndex,
					})
				}
				destWidth += glyph.Advance * scale
				if index+1 < runeCount {
					destWidth += spacing
				}
				index++
			}
		}
	}
	if destWidth <= 0 || destHeight <= 0 || len(*dst) == start {
		*dst = (*dst)[:start]
		return 0
	}

	clipMin, clipMax := textClip(c, cmd)
	transform := textScreenTransform(c.cam, cmd)
	topY := textTopY(cmd.Baseline, destHeight, nativeAscent, nativeHeight)
	appendTextLabel(
		labels, transform, textOriginX(cmd.Align, destWidth),
		topY+nativeAscent*scale, scale, clipMin, clipMax, cmd.Color,
	)
	return len(*dst) - start
}

func requestedTextPixelSize(c ctx, size float64) float64 {
	if size <= 0 {
		size = glyphHeight
	}
	return c.cam.WorldScalarToScreen(size)
}

type textLabelLayout struct {
	position         math2d.Vec2
	originX          float64
	topY, baselineY  float64
	scale            float64
	clipMin, clipMax math2d.Vec2
	a, b, cc, d      float64
}

func textOriginX(align render.TextAlign, width float64) float64 {
	switch align {
	case render.AlignCenter:
		return -width / 2
	case render.AlignRight:
		return -width
	default:
		return 0
	}
}

func textTopY(baseline render.TextBaseline, height, nativeAscent, nativeHeight float64) float64 {
	switch baseline {
	case render.BaselineMiddle:
		return -height / 2
	case render.BaselineBottom:
		return -height
	case render.BaselineTop:
		return 0
	default:
		return -(nativeAscent / nativeHeight) * height
	}
}

func textRotation(angle float64) (sin, cos float64) {
	sin, cos = 0.0, 1.0
	if angle != 0 {
		sin, cos = math.Sincos(angle)
	}
	return sin, cos
}

func (l textLabelLayout) rotate(point math2d.Vec2) math2d.Vec2 {
	dx := point.X - l.position.X
	dy := point.Y - l.position.Y
	return math2d.Vec2{
		X: l.position.X + dx*l.a + dy*l.cc,
		Y: l.position.Y + dx*l.b + dy*l.d,
	}
}

type textTransform struct {
	position   math2d.Vec2
	a, b, c, d float64
}

func textScreenTransform(camera render.Camera, cmd render.DrawText) textTransform {
	sin, cos := textRotation(cmd.Rotation)
	mapPoint := func(offset math2d.Vec2) math2d.Vec2 {
		rotated := math2d.Vec2{
			X: offset.X*cos - offset.Y*sin,
			Y: offset.X*sin + offset.Y*cos,
		}
		world := camera.ScreenToWorld(camera.WorldToScreen(cmd.Position).Add(rotated))
		return camera.WorldToScreen(cmd.Transform.Apply(world))
	}
	origin := mapPoint(math2d.Vec2{})
	x := mapPoint(math2d.Vec2{X: 1}).Sub(origin)
	y := mapPoint(math2d.Vec2{Y: 1}).Sub(origin)
	return textTransform{position: origin, a: x.X, b: x.Y, c: y.X, d: y.Y}
}

func appendTextLabel(labels *[]textLabelRecord, transform textTransform, originX, baselineY, scale float64, clipMin, clipMax math2d.Vec2, color render.Color) uint32 {
	index := uint32(len(*labels))
	*labels = append(*labels, textLabelRecord{
		positionX: float32(transform.position.X), positionY: float32(transform.position.Y),
		transformA: float32(transform.a), transformB: float32(transform.b),
		transformC: float32(transform.c), transformD: float32(transform.d),
		originX: float32(originX), baselineY: float32(baselineY), scale: float32(scale),
		clipMinX: float32(clipMin.X), clipMinY: float32(clipMin.Y),
		clipMaxX: float32(clipMax.X), clipMaxY: float32(clipMax.Y),
		tint: packTextTint(color),
	})
	return index
}

func screenClipRect(c ctx, min, max math2d.Vec2) [4]float32 {
	a := screenToClip(c, min)
	b := screenToClip(c, max)
	return [4]float32{min32(a[0], b[0]), max32(a[0], b[0]), min32(a[1], b[1]), max32(a[1], b[1])}
}

func textClip(c ctx, cmd render.DrawText) (math2d.Vec2, math2d.Vec2) {
	if cmd.Clip.Width() <= 0 || cmd.Clip.Height() <= 0 {
		return math2d.Vec2{}, math2d.Vec2{X: c.w, Y: c.h}
	}
	a := c.cam.WorldToScreen(cmd.Clip.Min)
	b := c.cam.WorldToScreen(cmd.Clip.Max)
	return math2d.Vec2{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y)},
		math2d.Vec2{X: math.Max(a.X, b.X), Y: math.Max(a.Y, b.Y)}
}

func packTextTint(color render.Color) uint32 {
	return uint32(spriteColorByte(color.R)) |
		uint32(spriteColorByte(color.G))<<8 |
		uint32(spriteColorByte(color.B))<<16 |
		uint32(spriteColorByte(color.A))<<24
}

func pushTextQuad(
	m *TextMesh,
	c ctx,
	a, b, cc, d math2d.Vec2,
	uv [4]float32,
	col render.Color,
) {
	v0 := textVertex(screenToClip(c, a), uv[0], uv[1], col)
	v1 := textVertex(screenToClip(c, b), uv[2], uv[1], col)
	v2 := textVertex(screenToClip(c, cc), uv[2], uv[3], col)
	v3 := textVertex(screenToClip(c, d), uv[0], uv[3], col)

	m.append(v0, v1, v2, v0, v2, v3)
}

func textVertex(pos [2]float32, u, v float32, col render.Color) TextVertex {
	return TextVertex{
		X: pos[0],
		Y: pos[1],
		R: float32(col.R),
		G: float32(col.G),
		B: float32(col.B),
		A: float32(col.A),
		U: u,
		V: v,
	}
}

func clipTextTriangles(verts []TextVertex, clip [4]float32) []TextVertex {
	if len(verts) == 0 {
		return nil
	}

	out := make([]TextVertex, 0, len(verts))

	for i := 0; i+2 < len(verts); i += 3 {
		poly := []TextVertex{verts[i], verts[i+1], verts[i+2]}

		poly = clipTextPolygon(poly, func(v TextVertex) bool { return v.X >= clip[0] }, func(a, b TextVertex) TextVertex {
			return intersectTextX(a, b, clip[0])
		})
		poly = clipTextPolygon(poly, func(v TextVertex) bool { return v.X <= clip[1] }, func(a, b TextVertex) TextVertex {
			return intersectTextX(a, b, clip[1])
		})
		poly = clipTextPolygon(poly, func(v TextVertex) bool { return v.Y >= clip[2] }, func(a, b TextVertex) TextVertex {
			return intersectTextY(a, b, clip[2])
		})
		poly = clipTextPolygon(poly, func(v TextVertex) bool { return v.Y <= clip[3] }, func(a, b TextVertex) TextVertex {
			return intersectTextY(a, b, clip[3])
		})

		if len(poly) < 3 {
			continue
		}

		for j := 1; j+1 < len(poly); j++ {
			out = append(out, poly[0], poly[j], poly[j+1])
		}
	}

	return out
}

func clipTextPolygon(
	poly []TextVertex,
	inside func(TextVertex) bool,
	intersect func(TextVertex, TextVertex) TextVertex,
) []TextVertex {
	if len(poly) == 0 {
		return nil
	}

	out := make([]TextVertex, 0, len(poly)+1)

	for i := range poly {
		a := poly[i]
		b := poly[(i+1)%len(poly)]

		aIn := inside(a)
		bIn := inside(b)

		if aIn {
			out = append(out, a)
		}
		if aIn != bIn {
			out = append(out, intersect(a, b))
		}
	}

	return out
}

func intersectTextX(a, b TextVertex, x float32) TextVertex {
	dx := b.X - a.X
	if dx == 0 {
		return a
	}

	t := (x - a.X) / dx
	return lerpTextVertex(a, b, t)
}

func intersectTextY(a, b TextVertex, y float32) TextVertex {
	dy := b.Y - a.Y
	if dy == 0 {
		return a
	}

	t := (y - a.Y) / dy
	return lerpTextVertex(a, b, t)
}

func lerpTextVertex(a, b TextVertex, t float32) TextVertex {
	return TextVertex{
		X: a.X + (b.X-a.X)*t,
		Y: a.Y + (b.Y-a.Y)*t,
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
		U: a.U + (b.U-a.U)*t,
		V: a.V + (b.V-a.V)*t,
	}
}
