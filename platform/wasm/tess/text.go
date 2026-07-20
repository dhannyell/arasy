package tess

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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
	if builtin {
		size := cmd.Size
		if size <= 0 {
			size = nativeHeight
		}
		measured := render.MeasureText(cmd.Text, size)
		scale = c.cam.WorldScalarToScreen(measured.Y) / nativeHeight
	}

	runes := []rune(cmd.Text)
	glyphs := make([]Glyph, len(runes))
	destWidth := 0.0
	for i, r := range runes {
		glyph, ok := atlas.Glyphs[r]
		if !ok {
			glyph, ok = atlas.Glyphs['?']
			if !ok {
				continue
			}
		}
		glyphs[i] = glyph
		destWidth += glyph.Advance * scale
	}
	destHeight := nativeHeight * scale
	if builtin {
		size := cmd.Size
		if size <= 0 {
			size = nativeHeight
		}
		measured := render.MeasureText(cmd.Text, size)
		destWidth = c.cam.WorldScalarToScreen(measured.X)
		destHeight = c.cam.WorldScalarToScreen(measured.Y)
	}
	if destWidth <= 0 || destHeight <= 0 {
		return
	}

	screenPos := c.cam.WorldToScreen(cmd.Position)

	offsetX := 0.0
	switch cmd.Align {
	case render.AlignCenter:
		offsetX = -destWidth / 2
	case render.AlignRight:
		offsetX = -destWidth
	}

	offsetY := 0.0
	switch cmd.Baseline {
	case render.BaselineTop:
		offsetY = 0
	case render.BaselineMiddle:
		offsetY = -destHeight / 2
	case render.BaselineBottom:
		offsetY = -destHeight
	default:
		offsetY = -(nativeAscent / nativeHeight) * destHeight
	}

	baseline := screenPos.Y + offsetY + nativeAscent*scale
	penX := screenPos.X + offsetX
	for _, glyph := range glyphs {
		if glyph.Width <= 0 || glyph.Height <= 0 {
			penX += glyph.Advance * scale
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
			y0 = screenPos.Y + offsetY
			x1 = x0 + advance
			y1 = y0 + destHeight
		}

		a := math2d.Vec2{X: x0, Y: y0}
		b := math2d.Vec2{X: x1, Y: y0}
		cc := math2d.Vec2{X: x1, Y: y1}
		d := math2d.Vec2{X: x0, Y: y1}

		if cmd.Rotation != 0 {
			a = rotateScreenPoint(a, screenPos, cmd.Rotation)
			b = rotateScreenPoint(b, screenPos, cmd.Rotation)
			cc = rotateScreenPoint(cc, screenPos, cmd.Rotation)
			d = rotateScreenPoint(d, screenPos, cmd.Rotation)
		}

		pushTextQuad(m, c, a, b, cc, d, glyph.UV, cmd.Color)
		penX += advance
	}

	if cmd.Clip.Width() > 0 && cmd.Clip.Height() > 0 {
		clip := clipRect(c, cmd.Clip)
		clipped := clipTextTriangles(m.Vertices[start:], clip)
		m.Vertices = append(m.Vertices[:start], clipped...)
	}
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

func rotateScreenPoint(p, anchor math2d.Vec2, angle float64) math2d.Vec2 {
	sin, cos := math.Sin(angle), math.Cos(angle)
	dx := p.X - anchor.X
	dy := p.Y - anchor.Y

	return math2d.Vec2{
		X: anchor.X + dx*cos - dy*sin,
		Y: anchor.Y + dx*sin + dy*cos,
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
