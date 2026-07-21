package math2d

import (
	"math"
	"testing"
)

func TestAffineIdentity(t *testing.T) {
	id := Identity()
	for _, p := range []Vec2{
		{},
		{X: 1, Y: 2},
		{X: -3, Y: 4.5},
		{X: 100, Y: -200},
	} {
		got := id.Apply(p)
		if got != p {
			t.Fatalf("Identity.Apply(%v) = %v, want %v", p, got, p)
		}
	}
}

func TestAffineZeroValueIsIdentity(t *testing.T) {
	var zero Affine
	if !zero.IsIdentity() {
		t.Fatalf("zero value should be identity")
	}

	p := Vec2{X: 42, Y: -17}
	if got := zero.Apply(p); got != p {
		t.Fatalf("zero.Apply(%v) = %v, want %v", p, got, p)
	}
}

func TestAffineTranslate(t *testing.T) {
	tv := Vec2{X: 5, Y: -3}
	m := Translate(tv)

	tests := []struct {
		in, want Vec2
	}{
		{Vec2{}, tv},
		{Vec2{X: 1, Y: 2}, Vec2{X: 6, Y: -1}},
		{Vec2{X: -5, Y: 3}, Vec2{X: 0, Y: 0}},
	}
	for _, tc := range tests {
		if got := m.Apply(tc.in); got != tc.want {
			t.Fatalf("Translate%v.Apply(%v) = %v, want %v", tv, tc.in, got, tc.want)
		}
	}
}

func TestAffineRotate(t *testing.T) {
	m := Rotate(math.Pi / 2)

	// 90° CCW: (1,0) -> (0,1), (0,1) -> (-1,0)
	if got := m.Apply(Vec2{X: 1, Y: 0}); !vec2Approx(got, Vec2{X: 0, Y: 1}) {
		t.Fatalf("Rotate(pi/2).Apply((1,0)) = %v, want (0,1)", got)
	}
	if got := m.Apply(Vec2{X: 0, Y: 1}); !vec2Approx(got, Vec2{X: -1, Y: 0}) {
		t.Fatalf("Rotate(pi/2).Apply((0,1)) = %v, want (-1,0)", got)
	}
}

func TestAffineScale(t *testing.T) {
	m := Scale(Vec2{X: 2, Y: 3})

	tests := []struct {
		in, want Vec2
	}{
		{Vec2{X: 1, Y: 1}, Vec2{X: 2, Y: 3}},
		{Vec2{X: -1, Y: 2}, Vec2{X: -2, Y: 6}},
	}
	for _, tc := range tests {
		if got := m.Apply(tc.in); got != tc.want {
			t.Fatalf("Scale(2,3).Apply(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestAffineShear(t *testing.T) {
	// Shear(2, 0): x' = x + 2*y, y' = y
	m := Shear(2, 0)
	if got := m.Apply(Vec2{X: 1, Y: 2}); got != (Vec2{X: 5, Y: 2}) {
		t.Fatalf("Shear(2,0).Apply((1,2)) = %v, want (5,2)", got)
	}

	// Shear(0, 3): y' = y + 3*x, x' = x
	m2 := Shear(0, 3)
	if got := m2.Apply(Vec2{X: 2, Y: 1}); got != (Vec2{X: 2, Y: 7}) {
		t.Fatalf("Shear(0,3).Apply((2,1)) = %v, want (2,7)", got)
	}
}

func TestAffineMul(t *testing.T) {
	// rotate∘translate ≠ translate∘rotate
	rot := Rotate(math.Pi / 2)
	trans := Translate(Vec2{X: 10, Y: 0})
	p := Vec2{X: 1, Y: 0}

	rt := rot.Mul(trans) // trans first, then rot
	tr := trans.Mul(rot) // rot first, then trans

	rtGot := rt.Apply(p)
	// trans(1,0) = (11,0); rot(11,0) = (0,11)
	if !vec2Approx(rtGot, Vec2{X: 0, Y: 11}) {
		t.Fatalf("rotate∘translate.Apply((1,0)) = %v, want (0,11)", rtGot)
	}

	trGot := tr.Apply(p)
	// rot(1,0) = (0,1); trans(0,1) = (10,1)
	if !vec2Approx(trGot, Vec2{X: 10, Y: 1}) {
		t.Fatalf("translate∘rotate.Apply((1,0)) = %v, want (10,1)", trGot)
	}

	// Prove different
	if rtGot == trGot {
		t.Fatal("rotate∘translate and translate∘rotate should differ")
	}

	// Composition contract: m.Mul(n).Apply(p) == m.Apply(n.Apply(p))
	contractGot := rot.Apply(trans.Apply(p))
	if !vec2Approx(rtGot, contractGot) {
		t.Fatalf("Mul contract: rot.Mul(trans).Apply(p) = %v, rot.Apply(trans.Apply(p)) = %v", rtGot, contractGot)
	}
}

func TestAffineAbout(t *testing.T) {
	// Rotate 90° around (10, 10): (11, 10) -> (10, 11)
	pivot := Vec2{X: 10, Y: 10}
	m := Rotate(math.Pi / 2).About(pivot)
	p := Vec2{X: 11, Y: 10}
	got := m.Apply(p)
	want := Vec2{X: 10, Y: 11}
	if !vec2Approx(got, want) {
		t.Fatalf("Rotate.About((10,10)).Apply((11,10)) = %v, want (10,11)", got)
	}

	// Pivot stays fixed
	pivotGot := m.Apply(pivot)
	if !vec2Approx(pivotGot, pivot) {
		t.Fatalf("About: pivot %v -> %v, should stay fixed", pivot, pivotGot)
	}
}

func TestAffineInverse(t *testing.T) {
	tests := []Affine{
		Identity(),
		Translate(Vec2{X: 5, Y: -3}),
		Rotate(0.7),
		Scale(Vec2{X: 2, Y: 0.5}),
		Shear(0.5, 0),
		Scale(Vec2{X: 2, Y: 3}).Mul(Rotate(1.2)).Mul(Translate(Vec2{X: 7, Y: -4})),
	}

	points := []Vec2{
		{X: 0, Y: 0},
		{X: 1, Y: 2},
		{X: -3.5, Y: 7.2},
		{X: 42, Y: -17},
	}

	for _, m := range tests {
		inv, ok := m.Inverse()
		if !ok {
			t.Fatalf("Inverse of %+v should succeed", m)
		}
		for _, p := range points {
			transformed := m.Apply(p)
			roundTripped := inv.Apply(transformed)
			if !vec2Approx(roundTripped, p) {
				t.Fatalf("inverse round-trip: m=%+v p=%v -> %v -> %v", m, p, transformed, roundTripped)
			}
		}
	}
}

func TestAffineInverseSingular(t *testing.T) {
	// Zero scale on both axes -> singular
	singular := Scale(Vec2{X: 0, Y: 0})
	_, ok := singular.Inverse()
	if ok {
		t.Fatal("zero scale should be singular")
	}
}

func TestAffineIsIdentity(t *testing.T) {
	zero := Affine{}
	if !zero.IsIdentity() {
		t.Fatal("zero value should be identity")
	}
	ident := Affine{A: 1, D: 1}
	if !ident.IsIdentity() {
		t.Fatal("{1,0,0,1,0,0} should be identity")
	}
	near1 := Affine{A: 1, D: 1, E: 0.001}
	if near1.IsIdentity() {
		t.Fatal("near-identity should not be identity")
	}
	near2 := Affine{A: 0.9999, D: 1}
	if near2.IsIdentity() {
		t.Fatal("near-identity should not be identity")
	}
}

func TestAffineApplyDoesNotAllocate(t *testing.T) {
	m := Rotate(math.Pi / 3)
	p := Vec2{X: 1, Y: 2}
	var sink Vec2
	allocs := testing.AllocsPerRun(100, func() {
		sink = m.Apply(p)
	})
	_ = sink
	if allocs != 0 {
		t.Fatalf("Apply allocated %v times per run, want 0", allocs)
	}
}

func TestAffineMulAssociative(t *testing.T) {
	a := Rotate(0.7)
	b := Scale(Vec2{X: 2, Y: 0.5})
	c := Translate(Vec2{X: 7, Y: -4})

	left := a.Mul(b).Mul(c)
	right := a.Mul(b.Mul(c))

	for _, p := range []Vec2{{}, {X: 1, Y: 2}, {X: -3.5, Y: 7.2}} {
		if l, r := left.Apply(p), right.Apply(p); !vec2Approx(l, r) {
			t.Fatalf("Mul not associative at %v: (a*b)*c = %v, a*(b*c) = %v", p, l, r)
		}
	}
}

func TestAffineMeanScale(t *testing.T) {
	tests := []struct {
		name string
		m    Affine
		want float64
	}{
		{"zero value", Affine{}, 1},
		{"identity", Identity(), 1},
		{"uniform scale 2", Scale(Vec2{X: 2, Y: 2}), 2},
		{"non-uniform 2x0.5", Scale(Vec2{X: 2, Y: 0.5}), 1},
		{"rotation preserves area", Rotate(1.2), 1},
		// A non-zero singular matrix (flattens Y): det = 0 but it is not the
		// zero value, so the identity short-circuit does not apply.
		{"singular", Scale(Vec2{X: 2, Y: 0}), 0},
	}
	for _, tc := range tests {
		if got := tc.m.MeanScale(); math.Abs(got-tc.want) > 1e-12 {
			t.Fatalf("%s: MeanScale() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAffineCombinedWithAbout(t *testing.T) {
	// Shear + Scale + Translate about pivot: the shadow case from the plan
	base := Vec2{X: 5, Y: 5}
	m := Shear(0.3, 0).About(base)
	// pivot stays
	if got := m.Apply(base); !vec2Approx(got, base) {
		t.Fatalf("shear about pivot: %v -> %v", base, got)
	}
	// point offset from pivot is sheared
	offset := Vec2{X: 5, Y: 7}
	got := m.Apply(offset) // (5,7): x' = x + 0.3*(y-5) = 5 + 0.3*2 = 5.6; y' = y = 7
	want := Vec2{X: 5.6, Y: 7}
	if !vec2Approx(got, want) {
		t.Fatalf("shear about (5,5): %v -> %v, want %v", offset, got, want)
	}
}

func vec2Approx(a, b Vec2) bool {
	return math.Abs(a.X-b.X) < 1e-10 && math.Abs(a.Y-b.Y) < 1e-10
}
