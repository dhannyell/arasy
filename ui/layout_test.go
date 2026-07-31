package ui

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

// layoutRect is 300x120 at (100, 200): wide enough that three columns of 100
// and three rows of 40 land on round numbers the assertions can state
// literally.
func layoutRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 100, Y: 200}, math2d.Vec2{X: 300, Y: 120})
}

func layoutCorners(minX, minY, maxX, maxY float64) math2d.Rect {
	return math2d.Rect{
		Min: math2d.Vec2{X: minX, Y: minY},
		Max: math2d.Vec2{X: maxX, Y: maxY},
	}
}

// layoutNear compares rects with a tolerance, for the cases where an equal
// split is not representable in binary (three columns out of 280 pixels).
func layoutNear(a, b math2d.Rect) bool {
	return nearFloat(a.Min.X, b.Min.X) && nearFloat(a.Min.Y, b.Min.Y) &&
		nearFloat(a.Max.X, b.Max.X) && nearFloat(a.Max.Y, b.Max.Y)
}

func nearFloat(x, y float64) bool {
	const eps = 1e-9
	diff := x - y
	return diff < eps && diff > -eps
}

func TestLayoutCuts(t *testing.T) {
	r := layoutRect()

	piece, rest := CutTop(r, 40)
	if want := layoutCorners(100, 200, 400, 240); piece != want {
		t.Errorf("CutTop() piece = %+v, want %+v", piece, want)
	}
	if want := layoutCorners(100, 240, 400, 320); rest != want {
		t.Errorf("CutTop() rest = %+v, want %+v", rest, want)
	}

	piece, rest = CutBottom(r, 40)
	if want := layoutCorners(100, 280, 400, 320); piece != want {
		t.Errorf("CutBottom() piece = %+v, want %+v", piece, want)
	}
	if want := layoutCorners(100, 200, 400, 280); rest != want {
		t.Errorf("CutBottom() rest = %+v, want %+v", rest, want)
	}

	piece, rest = CutLeft(r, 50)
	if want := layoutCorners(100, 200, 150, 320); piece != want {
		t.Errorf("CutLeft() piece = %+v, want %+v", piece, want)
	}
	if want := layoutCorners(150, 200, 400, 320); rest != want {
		t.Errorf("CutLeft() rest = %+v, want %+v", rest, want)
	}

	piece, rest = CutRight(r, 50)
	if want := layoutCorners(350, 200, 400, 320); piece != want {
		t.Errorf("CutRight() piece = %+v, want %+v", piece, want)
	}
	if want := layoutCorners(100, 200, 350, 320); rest != want {
		t.Errorf("CutRight() rest = %+v, want %+v", rest, want)
	}
}

func TestLayoutCutsPreserveTheWholeRect(t *testing.T) {
	r := layoutRect()
	for _, tc := range []struct {
		name string
		cut  func(math2d.Rect, float64) (math2d.Rect, math2d.Rect)
		axis func(math2d.Rect) float64
	}{
		{"CutTop", CutTop, math2d.Rect.Height},
		{"CutBottom", CutBottom, math2d.Rect.Height},
		{"CutLeft", CutLeft, math2d.Rect.Width},
		{"CutRight", CutRight, math2d.Rect.Width},
	} {
		piece, rest := tc.cut(r, 37)
		if got, want := tc.axis(piece)+tc.axis(rest), tc.axis(r); got != want {
			t.Errorf("%s() piece+rest = %v, want %v", tc.name, got, want)
		}
	}
}

func TestLayoutCutsClampToTheRect(t *testing.T) {
	r := layoutRect()

	piece, rest := CutTop(r, 500)
	if piece != r {
		t.Errorf("CutTop(500) piece = %+v, want the whole rect %+v", piece, r)
	}
	if want := layoutCorners(100, 320, 400, 320); rest != want {
		t.Errorf("CutTop(500) rest = %+v, want %+v", rest, want)
	}

	piece, rest = CutLeft(r, -10)
	if want := layoutCorners(100, 200, 100, 320); piece != want {
		t.Errorf("CutLeft(-10) piece = %+v, want an empty slice %+v", piece, want)
	}
	if rest != r {
		t.Errorf("CutLeft(-10) rest = %+v, want the whole rect %+v", rest, r)
	}
}

func TestLayoutInset(t *testing.T) {
	r := layoutRect()

	if got, want := Inset(r, 10), layoutCorners(110, 210, 390, 310); got != want {
		t.Errorf("Inset(10) = %+v, want %+v", got, want)
	}
	// A negative inset grows the rect: that is how the focus ring is drawn
	// just outside its widget.
	if got, want := Inset(r, -10), layoutCorners(90, 190, 410, 330); got != want {
		t.Errorf("Inset(-10) = %+v, want %+v", got, want)
	}
	if got, want := InsetXY(r, 20, 5), layoutCorners(120, 205, 380, 315); got != want {
		t.Errorf("InsetXY(20, 5) = %+v, want %+v", got, want)
	}
}

func TestLayoutColumns(t *testing.T) {
	r := layoutRect()

	cols := Columns(r, 3, 0)
	if len(cols) != 3 {
		t.Fatalf("len(Columns(3, 0)) = %d, want 3", len(cols))
	}
	for i, want := range []math2d.Rect{
		layoutCorners(100, 200, 200, 320),
		layoutCorners(200, 200, 300, 320),
		layoutCorners(300, 200, 400, 320),
	} {
		if cols[i] != want {
			t.Errorf("Columns(3, 0)[%d] = %+v, want %+v", i, cols[i], want)
		}
	}

	// (300 - 2*10) / 3 = 93.333..., so the gapped split is compared with a
	// tolerance; what must hold exactly is that the last column ends on Max.X.
	gapped := Columns(r, 3, 10)
	const width = (300.0 - 20) / 3
	for i, want := range []math2d.Rect{
		layoutCorners(100, 200, 100+width, 320),
		layoutCorners(100+width+10, 200, 100+2*width+10, 320),
		layoutCorners(100+2*width+20, 200, 100+3*width+20, 320),
	} {
		if !layoutNear(gapped[i], want) {
			t.Errorf("Columns(3, 10)[%d] = %+v, want %+v", i, gapped[i], want)
		}
	}
	if got, want := gapped[2].Max.X, r.Max.X; !nearFloat(got, want) {
		t.Errorf("Columns(3, 10) last column ends at %v, want %v", got, want)
	}
}

func TestLayoutRows(t *testing.T) {
	r := layoutRect()

	rows := Rows(r, 3, 0)
	if len(rows) != 3 {
		t.Fatalf("len(Rows(3, 0)) = %d, want 3", len(rows))
	}
	for i, want := range []math2d.Rect{
		layoutCorners(100, 200, 400, 240),
		layoutCorners(100, 240, 400, 280),
		layoutCorners(100, 280, 400, 320),
	} {
		if rows[i] != want {
			t.Errorf("Rows(3, 0)[%d] = %+v, want %+v", i, rows[i], want)
		}
	}

	gapped := Rows(r, 3, 15)
	const height = (120.0 - 30) / 3
	for i, want := range []math2d.Rect{
		layoutCorners(100, 200, 400, 200+height),
		layoutCorners(100, 200+height+15, 400, 200+2*height+15),
		layoutCorners(100, 200+2*height+30, 400, 200+3*height+30),
	} {
		if !layoutNear(gapped[i], want) {
			t.Errorf("Rows(3, 15)[%d] = %+v, want %+v", i, gapped[i], want)
		}
	}
}

// The indexed helpers exist for callers on a hot path; this locks them to the
// slice-returning ones so the two formulas cannot drift apart.
func TestLayoutIndexedHelpersMatchTheSlices(t *testing.T) {
	r := layoutRect()
	for _, count := range []int{1, 2, 3, 5, 7} {
		for _, gap := range []float64{0, 10, 13.5} {
			cols := Columns(r, count, gap)
			rows := Rows(r, count, gap)
			for i := range count {
				if got := Column(r, i, count, gap); got != cols[i] {
					t.Errorf("Column(%d, %d, %v) = %+v, want Columns()[%d] = %+v", i, count, gap, got, i, cols[i])
				}
				if got := Row(r, i, count, gap); got != rows[i] {
					t.Errorf("Row(%d, %d, %v) = %+v, want Rows()[%d] = %+v", i, count, gap, got, i, rows[i])
				}
			}
		}
	}
}

func TestLayoutGrid(t *testing.T) {
	r := layoutRect()

	cells := Grid(r, 3, 2, 0, 0)
	if len(cells) != 6 {
		t.Fatalf("len(Grid(3, 2)) = %d, want 6", len(cells))
	}
	// Row-major, index y*cols+x: 3 is the second row's first column and 4 is
	// its second.
	if want := layoutCorners(100, 260, 200, 320); cells[3] != want {
		t.Errorf("Grid(3, 2)[3] = %+v, want %+v", cells[3], want)
	}
	if want := layoutCorners(200, 260, 300, 320); cells[4] != want {
		t.Errorf("Grid(3, 2)[4] = %+v, want %+v", cells[4], want)
	}
	if want := layoutCorners(100, 200, 200, 260); cells[0] != want {
		t.Errorf("Grid(3, 2)[0] = %+v, want %+v", cells[0], want)
	}
	if want := layoutCorners(300, 260, 400, 320); cells[5] != want {
		t.Errorf("Grid(3, 2)[5] = %+v, want %+v", cells[5], want)
	}

	gapped := Grid(r, 3, 2, 10, 20)
	if got, want := gapped[0], Column(Row(r, 0, 2, 20), 0, 3, 10); got != want {
		t.Errorf("Grid(3, 2, 10, 20)[0] = %+v, want %+v", got, want)
	}
}

// The tab header band is laid out with Columns and a zero gap. This locks
// that split to the indexed formula the widget still exposes, so the shared
// helper cannot drift away from the geometry the goldens were rendered with.
func TestLayoutColumnsMatchTheTabHeaderFormula(t *testing.T) {
	rect := math2d.NewRect(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 300, Y: 200})
	const barH = 40
	for _, count := range []int{1, 2, 3, 4, 7} {
		headers := tabHeaderRects(rect, count, barH)
		if len(headers) != count {
			t.Fatalf("len(tabHeaderRects(%d)) = %d, want %d", count, len(headers), count)
		}
		for i := range count {
			if want := tabHeaderRect(rect, i, count, barH); headers[i] != want {
				t.Errorf("tabHeaderRects(%d)[%d] = %+v, want %+v", count, i, headers[i], want)
			}
		}
	}
}

func TestLayoutDegenerateCounts(t *testing.T) {
	r := layoutRect()

	for _, count := range []int{0, -1} {
		if got := Columns(r, count, 10); len(got) != 0 {
			t.Errorf("len(Columns(%d)) = %d, want 0", count, len(got))
		}
		if got := Rows(r, count, 10); len(got) != 0 {
			t.Errorf("len(Rows(%d)) = %d, want 0", count, len(got))
		}
		if got := Grid(r, count, 2, 0, 0); len(got) != 0 {
			t.Errorf("len(Grid(cols=%d)) = %d, want 0", count, len(got))
		}
		if got := Grid(r, 2, count, 0, 0); len(got) != 0 {
			t.Errorf("len(Grid(rows=%d)) = %d, want 0", count, len(got))
		}

		empty := math2d.NewRect(r.Min, math2d.Vec2{})
		if got := Column(r, 0, count, 10); got != empty {
			t.Errorf("Column(count=%d) = %+v, want %+v", count, got, empty)
		}
		if got := Row(r, 0, count, 10); got != empty {
			t.Errorf("Row(count=%d) = %+v, want %+v", count, got, empty)
		}
	}
}

func TestLayoutNeverProducesNegativeSizes(t *testing.T) {
	r := layoutRect()

	// A gap wider than the rect itself would give every cell a negative size
	// without the clamp.
	for i, col := range Columns(r, 3, 400) {
		if col.Width() < 0 {
			t.Errorf("Columns(3, 400)[%d].Width() = %v, want >= 0", i, col.Width())
		}
	}
	for i, row := range Rows(r, 3, 400) {
		if row.Height() < 0 {
			t.Errorf("Rows(3, 400)[%d].Height() = %v, want >= 0", i, row.Height())
		}
	}
	for i, cell := range Grid(r, 3, 2, 400, 400) {
		if cell.Width() < 0 || cell.Height() < 0 {
			t.Errorf("Grid(3, 2, 400, 400)[%d] size = %+v, want no negative side", i, cell.Size())
		}
	}
	if _, rest := CutTop(r, 500); rest.Height() < 0 {
		t.Errorf("CutTop(500) rest.Height() = %v, want >= 0", rest.Height())
	}
}

// The helpers are pure: they take a rect by value and return new ones. This
// documents that intent and fails if someone later switches to a pointer
// receiver.
func TestLayoutLeavesTheInputRectAlone(t *testing.T) {
	r := layoutRect()
	before := r

	CutTop(r, 40)
	CutBottom(r, 40)
	CutLeft(r, 40)
	CutRight(r, 40)
	Inset(r, 10)
	InsetXY(r, 10, 20)
	Columns(r, 3, 10)
	Rows(r, 3, 10)
	Column(r, 1, 3, 10)
	Row(r, 1, 3, 10)
	Grid(r, 3, 2, 10, 10)

	if r != before {
		t.Errorf("input rect = %+v after the layout helpers, want %+v", r, before)
	}
}
