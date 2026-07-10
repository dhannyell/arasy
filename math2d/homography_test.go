package math2d_test

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

// homographyTestEpsilon is the tight tolerance used for round-trip and
// corner-reproduction checks, where the underlying arithmetic is expected to
// be exact (no accumulated rounding beyond a couple of divisions).
const homographyTestEpsilon = 1e-9

func unitSquareCorners() [4]math2d.Vec2 {
	return [4]math2d.Vec2{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 1, Y: 1},
		{X: 0, Y: 1},
	}
}

func TestHomographyFromQuadUnitSquareIsIdentity(t *testing.T) {
	corners := unitSquareCorners()

	h, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		t.Fatalf("HomographyFromQuad(unit square) ok = false, want true")
	}
	if !h.IsAffine() {
		t.Fatalf("IsAffine() = false, want true for the unit square")
	}

	for _, c := range corners {
		got := h.Apply(c)
		assertVec2NearEps(t, got, c, homographyTestEpsilon)
	}
}

func TestHomographyFromQuadParallelogramIsAffine(t *testing.T) {
	// A sheared parallelogram: opposite sides remain parallel, so the map
	// stays affine (no perspective row needed).
	corners := [4]math2d.Vec2{
		{X: 0, Y: 0},
		{X: 2, Y: 0},
		{X: 3, Y: 1},
		{X: 1, Y: 1},
	}

	h, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		t.Fatalf("HomographyFromQuad(parallelogram) ok = false, want true")
	}
	if !h.IsAffine() {
		t.Fatalf("IsAffine() = false, want true for a parallelogram")
	}

	// Hand-derived affine map for this parallelogram: x' = 2x + y, y' = y.
	scenarios := []struct {
		name string
		p    math2d.Vec2
		want math2d.Vec2
	}{
		{"top-left", math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 0, Y: 0}},
		{"top-right", math2d.Vec2{X: 1, Y: 0}, math2d.Vec2{X: 2, Y: 0}},
		{"bottom-right", math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 3, Y: 1}},
		{"bottom-left", math2d.Vec2{X: 0, Y: 1}, math2d.Vec2{X: 1, Y: 1}},
		{"center", math2d.Vec2{X: 0.5, Y: 0.5}, math2d.Vec2{X: 1.5, Y: 0.5}},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			got := h.Apply(s.p)
			assertVec2NearEps(t, got, s.want, homographyTestEpsilon)
		})
	}
}

func TestHomographyFromQuadTrapezoidHasPerspective(t *testing.T) {
	// A trapezoid where the top and bottom edges are parallel but of
	// different length: the diagonals no longer share a midpoint, so the
	// map needs a genuine perspective row.
	corners := [4]math2d.Vec2{
		{X: 0, Y: 0},
		{X: 4, Y: 0},
		{X: 3, Y: 2},
		{X: 1, Y: 2},
	}

	h, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		t.Fatalf("HomographyFromQuad(trapezoid) ok = false, want true")
	}
	if h.IsAffine() {
		t.Fatalf("IsAffine() = true, want false for a genuine perspective trapezoid")
	}

	unitSquare := unitSquareCorners()
	for i, u := range unitSquare {
		got := h.Apply(u)
		assertVec2NearEps(t, got, corners[i], homographyTestEpsilon)
	}

	// A perspective map bends the diagonals of the unit square, so its
	// center must not land on the centroid of the destination corners; only
	// an affine map would preserve that relationship.
	center := h.Apply(math2d.Vec2{X: 0.5, Y: 0.5})
	centroid := math2d.Vec2{X: 2, Y: 1} // average of the four corners above
	if near(center.X, centroid.X) && near(center.Y, centroid.Y) {
		t.Fatalf("Apply(center) = %v landed on the corner centroid %v, want a perspective-skewed point", center, centroid)
	}
}

func TestHomographyInverseRoundTrip(t *testing.T) {
	corners := [4]math2d.Vec2{
		{X: 0, Y: 0},
		{X: 4, Y: 0},
		{X: 3, Y: 2},
		{X: 1, Y: 2},
	}

	h, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		t.Fatalf("HomographyFromQuad ok = false, want true")
	}
	inv, ok := h.Inverse()
	if !ok {
		t.Fatalf("Inverse() ok = false, want true")
	}

	points := []math2d.Vec2{
		{X: 0, Y: 0},
		{X: 1, Y: 1},
		{X: 0.2, Y: 0.3},
		{X: 0.7, Y: 0.4},
		{X: 0.9, Y: 0.85},
	}

	for _, p := range points {
		mapped := h.Apply(p)
		back := inv.Apply(mapped)
		assertVec2NearEps(t, back, p, homographyTestEpsilon)
	}
}

func TestHomographyFromQuadDegenerateCorners(t *testing.T) {
	scenarios := []struct {
		name    string
		corners [4]math2d.Vec2
	}{
		{
			name: "three corners collinear",
			corners: [4]math2d.Vec2{
				{X: 0, Y: 1},
				{X: 1, Y: 0},
				{X: 2, Y: 0},
				{X: 3, Y: 0},
			},
		},
		{
			name: "all four corners equal",
			corners: [4]math2d.Vec2{
				{X: 1, Y: 1},
				{X: 1, Y: 1},
				{X: 1, Y: 1},
				{X: 1, Y: 1},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			_, ok := math2d.HomographyFromQuad(s.corners)
			if ok {
				t.Fatalf("HomographyFromQuad(%v) ok = true, want false", s.corners)
			}
		})
	}
}

func TestIsConvexQuad(t *testing.T) {
	tests := []struct {
		name string
		quad [4]math2d.Vec2
		want bool
	}{
		{
			name: "convex trapezoid",
			quad: [4]math2d.Vec2{{X: 1, Y: 0}, {X: 3, Y: 0}, {X: 4, Y: 2}, {X: 0, Y: 2}},
			want: true,
		},
		{
			name: "self intersecting bowtie",
			quad: [4]math2d.Vec2{{X: 0, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}, {X: 2, Y: 0}},
			want: false,
		},
		{
			name: "collinear corner",
			quad: [4]math2d.Vec2{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 0}, {X: 0, Y: 2}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := math2d.IsConvexQuad(tt.quad); got != tt.want {
				t.Fatalf("IsConvexQuad() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHomographyApplyWPositiveWeightForConvexQuad(t *testing.T) {
	corners := [4]math2d.Vec2{
		{X: 0, Y: 0},
		{X: 4, Y: 0},
		{X: 3, Y: 2},
		{X: 1, Y: 2},
	}

	h, ok := math2d.HomographyFromQuad(corners)
	if !ok {
		t.Fatalf("HomographyFromQuad ok = false, want true")
	}

	for _, u := range unitSquareCorners() {
		_, w := h.ApplyW(u)
		if w <= 0 {
			t.Fatalf("ApplyW(%v) w = %v, want > 0", u, w)
		}
	}
}

func assertVec2NearEps(t *testing.T, got, want math2d.Vec2, epsilon float64) {
	t.Helper()

	if math.Abs(got.X-want.X) > epsilon || math.Abs(got.Y-want.Y) > epsilon {
		t.Fatalf("got %v, want %v (epsilon %v)", got, want, epsilon)
	}
}
