package tess

// clipTriangles clips a triangle list against an axis-aligned rectangle in
// clip space.
//
// verts is interpreted as a sequence of independent triangles: every 3
// vertices form one triangle. clip is ordered as {xMin, xMax, yMin, yMax}.
//
// A triangle that is fully inside the clip rectangle is kept unchanged. A
// triangle that is fully outside is discarded. A triangle that intersects the
// rectangle may become a polygon with more than 3 vertices; that polygon is
// triangulated back into a fan before being returned.
//
// The returned vertices are still a triangle list and can be uploaded directly
// to the solid-color WebGPU vertex buffer.
func clipTriangles(verts []Vertex, clip [4]float32) []Vertex {
	if len(verts) == 0 {
		return nil
	}

	out := make([]Vertex, 0, len(verts))

	for i := 0; i+2 < len(verts); i += 3 {
		poly := []Vertex{verts[i], verts[i+1], verts[i+2]}

		poly = clipPolygon(poly, func(v Vertex) bool { return v.X >= clip[0] }, func(a, b Vertex) Vertex {
			return intersectX(a, b, clip[0])
		})
		poly = clipPolygon(poly, func(v Vertex) bool { return v.X <= clip[1] }, func(a, b Vertex) Vertex {
			return intersectX(a, b, clip[1])
		})
		poly = clipPolygon(poly, func(v Vertex) bool { return v.Y >= clip[2] }, func(a, b Vertex) Vertex {
			return intersectY(a, b, clip[2])
		})
		poly = clipPolygon(poly, func(v Vertex) bool { return v.Y <= clip[3] }, func(a, b Vertex) Vertex {
			return intersectY(a, b, clip[3])
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

// clipPolygon clips one polygon against a single half-plane.
//
// The inside function decides whether a vertex is inside the current clipping
// plane. The intersect function creates a new vertex where an edge crosses that
// plane.
//
// This is one step of the Sutherland-Hodgman polygon clipping algorithm. To
// clip against a rectangle, call it once for each rectangle side.
func clipPolygon(
	poly []Vertex,
	inside func(Vertex) bool,
	intersect func(Vertex, Vertex) Vertex,
) []Vertex {
	if len(poly) == 0 {
		return nil
	}

	out := make([]Vertex, 0, len(poly)+1)

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

// clipTexTriangles clips a textured triangle list against an axis-aligned
// rectangle in clip space, generalizing clipTriangles/clipPolygon to
// TexVertex.
//
// It must only be called when W is constant across the primitive: with
// W==1 (AppendImage) or a constant non-unit w (an affine AppendImageQuad),
// linear interpolation of every attribute is exact. A projective quad
// (varying w per corner) must never reach this function -- linearly
// interpolating XW/YW/W there would not reconstruct the true clip-space
// position, and interpolating U/V directly would skip the perspective
// divide, distorting the texture. See AppendImageQuad for the guard.
func clipTexTriangles(verts []TexVertex, clip [4]float32) []TexVertex {
	if len(verts) == 0 {
		return nil
	}

	out := make([]TexVertex, 0, len(verts))

	for i := 0; i+2 < len(verts); i += 3 {
		poly := []TexVertex{verts[i], verts[i+1], verts[i+2]}

		poly = clipTexPolygon(poly, func(v TexVertex) bool { return texClipX(v) >= clip[0] }, func(a, b TexVertex) TexVertex {
			return intersectTexX(a, b, clip[0])
		})
		poly = clipTexPolygon(poly, func(v TexVertex) bool { return texClipX(v) <= clip[1] }, func(a, b TexVertex) TexVertex {
			return intersectTexX(a, b, clip[1])
		})
		poly = clipTexPolygon(poly, func(v TexVertex) bool { return texClipY(v) >= clip[2] }, func(a, b TexVertex) TexVertex {
			return intersectTexY(a, b, clip[2])
		})
		poly = clipTexPolygon(poly, func(v TexVertex) bool { return texClipY(v) <= clip[3] }, func(a, b TexVertex) TexVertex {
			return intersectTexY(a, b, clip[3])
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

// clipTexPolygon clips one polygon against a single half-plane. See
// clipPolygon; this is the same Sutherland-Hodgman step generalized to
// TexVertex.
func clipTexPolygon(
	poly []TexVertex,
	inside func(TexVertex) bool,
	intersect func(TexVertex, TexVertex) TexVertex,
) []TexVertex {
	if len(poly) == 0 {
		return nil
	}

	out := make([]TexVertex, 0, len(poly)+1)

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

// texClipX and texClipY recover the true clip-space coordinates of a
// TexVertex: XW/W and YW/W (with W==1 this is XW/YW directly). The clip
// rectangle bounds are expressed in real clip space, so the inside/outside
// test and the interpolation parameter below must use these, not the raw
// pre-multiplied XW/YW.
func texClipX(v TexVertex) float32 { return v.XW / v.W }
func texClipY(v TexVertex) float32 { return v.YW / v.W }

// intersectTexX returns the interpolated vertex where the edge from a to b
// crosses the vertical clip-space line x.
//
// If the edge is vertical in clip space, there is no stable interpolation
// parameter to compute, so the function returns a. In normal clipping flow
// this degenerate case only happens when the edge is already on the
// clipping plane.
func intersectTexX(a, b TexVertex, x float32) TexVertex {
	dx := texClipX(b) - texClipX(a)
	if dx == 0 {
		return a
	}

	t := (x - texClipX(a)) / dx
	return lerpTexVertex(a, b, t)
}

// intersectTexY returns the interpolated vertex where the edge from a to b
// crosses the horizontal clip-space line y. See intersectTexX.
func intersectTexY(a, b TexVertex, y float32) TexVertex {
	dy := texClipY(b) - texClipY(a)
	if dy == 0 {
		return a
	}

	t := (y - texClipY(a)) / dy
	return lerpTexVertex(a, b, t)
}

// lerpTexVertex linearly interpolates all TexVertex attributes between a and
// b, including the raw XW/YW/W fields.
//
// This is exact only because clipTexTriangles is restricted to constant-w
// primitives: a and b then share the same W, so lerping XW directly is
// equivalent to lerping the true clip-space X (XW/W) and re-multiplying by
// that shared W. Under this constraint linear interpolation is exact, so
// there is no perspective-correct fixup to apply here.
func lerpTexVertex(a, b TexVertex, t float32) TexVertex {
	return TexVertex{
		XW: a.XW + (b.XW-a.XW)*t,
		YW: a.YW + (b.YW-a.YW)*t,
		W:  a.W + (b.W-a.W)*t,
		R:  a.R + (b.R-a.R)*t,
		G:  a.G + (b.G-a.G)*t,
		B:  a.B + (b.B-a.B)*t,
		A:  a.A + (b.A-a.A)*t,
		U:  a.U + (b.U-a.U)*t,
		V:  a.V + (b.V-a.V)*t,
	}
}

// intersectX returns the interpolated vertex where the edge from a to b crosses
// the vertical line x.
//
// If the edge is vertical, there is no stable interpolation parameter to
// compute, so the function returns a. In normal clipping flow this degenerate
// case only happens when the edge is already on the clipping plane.
func intersectX(a, b Vertex, x float32) Vertex {
	dx := b.X - a.X
	if dx == 0 {
		return a
	}

	t := (x - a.X) / dx
	return lerpVertex(a, b, t)
}

// intersectY returns the interpolated vertex where the edge from a to b crosses
// the horizontal line y.
//
// If the edge is horizontal, there is no stable interpolation parameter to
// compute, so the function returns a. In normal clipping flow this degenerate
// case only happens when the edge is already on the clipping plane.
func intersectY(a, b Vertex, y float32) Vertex {
	dy := b.Y - a.Y
	if dy == 0 {
		return a
	}

	t := (y - a.Y) / dy
	return lerpVertex(a, b, t)
}

// lerpVertex linearly interpolates all vertex attributes between a and b.
//
// t is expected to be in the range [0, 1] for clipping intersections. Position
// and color are interpolated together so newly-created clipping vertices remain
// visually continuous with the original triangle.
//
// For the solid-color mesh the color is usually constant, but keeping full
// interpolation here makes the clipping logic reusable for future vertex types
// that carry attributes such as texture coordinates.
func lerpVertex(a, b Vertex, t float32) Vertex {
	return Vertex{
		X: a.X + (b.X-a.X)*t,
		Y: a.Y + (b.Y-a.Y)*t,
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}
