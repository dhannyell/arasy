package ui

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
)

// Layout here is deliberately just arithmetic on rects: every helper is a
// pure function that takes a rect and returns rects. Nothing is remembered
// between frames and nothing is measured, which is what keeps the package's
// whole state down to the few fields the driver clones for rewind.
//
// The model is RectCut: take a slice off one edge and keep what is left.
// Content-driven sizing — a box that grows to fit its text — needs either a
// second pass or last frame's measurements, and is deliberately absent.

// CutTop slices amount off the top of r, returning the slice and the rest.
// amount is clamped to [0, r.Height()], so an oversized cut yields the whole
// rect and an empty rest rather than an inverted one.
func CutTop(r math2d.Rect, amount float64) (piece, rest math2d.Rect) {
	amount = clampCut(amount, r.Height())
	piece = math2d.NewRect(r.Min, math2d.Vec2{X: r.Width(), Y: amount})
	rest = math2d.NewRect(
		math2d.Vec2{X: r.Min.X, Y: r.Min.Y + amount},
		math2d.Vec2{X: r.Width(), Y: r.Height() - amount},
	)
	return piece, rest
}

// CutBottom slices amount off the bottom of r, returning the slice and the
// rest. amount is clamped to [0, r.Height()].
func CutBottom(r math2d.Rect, amount float64) (piece, rest math2d.Rect) {
	amount = clampCut(amount, r.Height())
	piece = math2d.NewRect(
		math2d.Vec2{X: r.Min.X, Y: r.Max.Y - amount},
		math2d.Vec2{X: r.Width(), Y: amount},
	)
	rest = math2d.NewRect(r.Min, math2d.Vec2{X: r.Width(), Y: r.Height() - amount})
	return piece, rest
}

// CutLeft slices amount off the left of r, returning the slice and the rest.
// amount is clamped to [0, r.Width()].
func CutLeft(r math2d.Rect, amount float64) (piece, rest math2d.Rect) {
	amount = clampCut(amount, r.Width())
	piece = math2d.NewRect(r.Min, math2d.Vec2{X: amount, Y: r.Height()})
	rest = math2d.NewRect(
		math2d.Vec2{X: r.Min.X + amount, Y: r.Min.Y},
		math2d.Vec2{X: r.Width() - amount, Y: r.Height()},
	)
	return piece, rest
}

// CutRight slices amount off the right of r, returning the slice and the
// rest. amount is clamped to [0, r.Width()].
func CutRight(r math2d.Rect, amount float64) (piece, rest math2d.Rect) {
	amount = clampCut(amount, r.Width())
	piece = math2d.NewRect(
		math2d.Vec2{X: r.Max.X - amount, Y: r.Min.Y},
		math2d.Vec2{X: amount, Y: r.Height()},
	)
	rest = math2d.NewRect(r.Min, math2d.Vec2{X: r.Width() - amount, Y: r.Height()})
	return piece, rest
}

// Inset shrinks r by amount on all four sides. A negative amount grows it,
// which is how focus rings are drawn just outside their widget.
func Inset(r math2d.Rect, amount float64) math2d.Rect {
	return InsetXY(r, amount, amount)
}

// InsetXY shrinks r by x horizontally and y vertically. Negative values grow
// r, like Inset.
func InsetXY(r math2d.Rect, x, y float64) math2d.Rect {
	return math2d.NewRectFromCorners(
		math2d.Vec2{X: r.Min.X + x, Y: r.Min.Y + y},
		math2d.Vec2{X: r.Max.X - x, Y: r.Max.Y - y},
	)
}

// Rows splits r into count rows of equal height separated by gap. A count of
// zero or less yields no rows.
func Rows(r math2d.Rect, count int, gap float64) []math2d.Rect {
	rows := make([]math2d.Rect, max(count, 0))
	for i := range rows {
		rows[i] = Row(r, i, count, gap)
	}
	return rows
}

// Columns splits r into count columns of equal width separated by gap. A
// count of zero or less yields no columns.
func Columns(r math2d.Rect, count int, gap float64) []math2d.Rect {
	cols := make([]math2d.Rect, max(count, 0))
	for i := range cols {
		cols[i] = Column(r, i, count, gap)
	}
	return cols
}

// Column returns the i-th of count equal columns, for callers that want one
// cell without allocating the whole row.
func Column(r math2d.Rect, i, count int, gap float64) math2d.Rect {
	if count <= 0 {
		return math2d.NewRect(r.Min, math2d.Vec2{})
	}
	width := (r.Width() - gap*float64(count-1)) / float64(count)
	x := r.Min.X + float64(i)*(width+gap)
	return math2d.NewRect(
		math2d.Vec2{X: x, Y: r.Min.Y},
		math2d.Vec2{X: math.Max(0, width), Y: r.Height()},
	)
}

// Row returns the i-th of count equal rows.
func Row(r math2d.Rect, i, count int, gap float64) math2d.Rect {
	if count <= 0 {
		return math2d.NewRect(r.Min, math2d.Vec2{})
	}
	height := (r.Height() - gap*float64(count-1)) / float64(count)
	y := r.Min.Y + float64(i)*(height+gap)
	return math2d.NewRect(
		math2d.Vec2{X: r.Min.X, Y: y},
		math2d.Vec2{X: r.Width(), Y: math.Max(0, height)},
	)
}

// Grid splits r into a cols-by-rows grid, row-major: index y*cols+x.
func Grid(r math2d.Rect, cols, rows int, gapX, gapY float64) []math2d.Rect {
	if cols <= 0 || rows <= 0 {
		return []math2d.Rect{}
	}
	cells := make([]math2d.Rect, 0, cols*rows)
	for _, line := range Rows(r, rows, gapY) {
		cells = append(cells, Columns(line, cols, gapX)...)
	}
	return cells
}

// clampCut keeps a cut inside its rect, so no helper can return an inverted
// or negative-sized piece.
func clampCut(amount, limit float64) float64 {
	return math.Max(0, math.Min(amount, limit))
}
