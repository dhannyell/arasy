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
