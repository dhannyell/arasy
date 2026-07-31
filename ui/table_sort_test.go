package ui

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func sortTableRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 300, Y: 200})
}

func sortTableColumns() []TableColumn {
	return []TableColumn{{Title: "Name"}, {Title: "Age"}, {Title: "Rating"}}
}

func TestTableHeaderHitReportsClickedColumn(t *testing.T) {
	rect := sortTableRect()
	headerY := rect.Min.Y + DefaultTheme().Header.Height/2
	// The three flexible columns split the width evenly.
	pos := math2d.Vec2{X: rect.Min.X + rect.Width()/2, Y: headerY}

	col := clickAt(&State{}, pos).tableHeaderHitIn("players", rect, sortTableColumns(), SpaceRef)
	if col != 1 {
		t.Fatalf("TableHeaderHit() = %d, want 1", col)
	}
}

func TestTableHeaderHitIgnoresBodyClicks(t *testing.T) {
	rect := sortTableRect()
	pos := math2d.Vec2{X: rect.Center().X, Y: rect.Max.Y - 10}

	if col := clickAt(&State{}, pos).tableHeaderHitIn("players", rect, sortTableColumns(), SpaceRef); col != -1 {
		t.Fatalf("TableHeaderHit() = %d, want -1 for a click in the body", col)
	}
}

// Two tables on the same screen must not share header hit targets.
func TestTableHeaderHitIsScopedToTheWidget(t *testing.T) {
	rect := sortTableRect()
	state := &State{}
	pos := math2d.Vec2{X: rect.Min.X + 10, Y: rect.Min.Y + DefaultTheme().Header.Height/2}

	if col := clickAt(state, pos).tableHeaderHitIn("players", rect, sortTableColumns(), SpaceRef); col != 0 {
		t.Fatalf("TableHeaderHit() = %d, want 0", col)
	}
	if col := clickAt(state, pos).tableHeaderHitIn("staff", rect, sortTableColumns(), SpaceRef); col != 0 {
		t.Fatalf("second table TableHeaderHit() = %d, want its own hit", col)
	}
}

func drawSortedTable(sort TableSort) []render.Command {
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	u.tableStyledIn("players", sortTableRect(), sortTableColumns(), 3, -1,
		func(row, col int) string { return "x" }, nil, sort, SpaceRef)
	return u.Commands()
}

func TestTableDrawsSortIndicatorOnlyForTheSortedColumn(t *testing.T) {
	arrows := func(cmds []render.Command) []render.DrawPolygon {
		var found []render.DrawPolygon
		for _, cmd := range cmds {
			if poly, ok := cmd.(render.DrawPolygon); ok {
				found = append(found, poly)
			}
		}
		return found
	}

	if got := arrows(drawSortedTable(NoTableSort)); len(got) != 0 {
		t.Fatalf("unsorted table drew %d arrows, want 0", len(got))
	}

	asc := arrows(drawSortedTable(TableSort{Col: 2}))
	desc := arrows(drawSortedTable(TableSort{Col: 2, Desc: true}))
	if len(asc) != 1 || len(desc) != 1 {
		t.Fatalf("sorted table drew %d/%d arrows, want one each", len(asc), len(desc))
	}
	if asc[0].Points[0].Y == desc[0].Points[0].Y {
		t.Fatal("ascending and descending drew the same arrow")
	}
}
