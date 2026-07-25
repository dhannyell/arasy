package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestListHitUsesScrollOffsetForClickedRow(t *testing.T) {
	rowH := 20.0
	state := &State{scroll: map[ID]float64{HashID("players"): 40}}
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 200, Y: 100})
	cam := testInteractCamera()
	downUp := []input.PointerEvent{
		{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 20, Y: 52}},
		{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 20, Y: 52}},
	}
	u := NewUpdate(state, pointerFrame(downUp...), cam, DefaultTheme(), cam.Viewport)

	// (52-10+40)/20 = 4.1 → row 4
	row := u.listHitIn("players", rect, rowH, 20, SpaceRef)
	if row != 4 {
		t.Fatalf("clicked row = %v, want 4", row)
	}
}

func TestListHitReturnsMinusOneOutsideRect(t *testing.T) {
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 200, Y: 100})
	cam := testInteractCamera()
	downUp := []input.PointerEvent{
		{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 5, Y: 5}},
		{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 5, Y: 5}},
	}
	u := NewUpdate(&State{}, pointerFrame(downUp...), cam, DefaultTheme(), cam.Viewport)

	if row := u.listHitIn("players", rect, 20, 20, SpaceRef); row != -1 {
		t.Fatalf("clicked row = %v, want -1 (click outside rect)", row)
	}
}

func TestListInCullsRowsOutsideVisibleWindow(t *testing.T) {
	rowH := 20.0
	rowCount := 50
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	rect := math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 100, Y: 60})

	drawn := map[int]bool{}
	u.listIn("rows", rect, rowH, rowCount, -1, func(row int, rowRect math2d.Rect) {
		drawn[row] = true
	}, SpaceRef)

	if len(drawn) == 0 {
		t.Fatal("expected at least one row to be drawn")
	}
	if len(drawn) >= rowCount {
		t.Fatalf("drew %d of %d rows, want most rows culled outside the visible window", len(drawn), rowCount)
	}
	if !drawn[0] {
		t.Fatal("expected row 0 to be visible at zero scroll offset")
	}
	if drawn[rowCount-1] {
		t.Fatal("expected the last row to be culled at zero scroll offset")
	}
}

func TestListInMarksSelectedRow(t *testing.T) {
	rowH := 20.0
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	rect := math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 100, Y: 60})

	u.listIn("rows", rect, rowH, 3, 1, func(row int, rowRect math2d.Rect) {}, SpaceRef)

	found := false
	for _, command := range u.Commands() {
		if r, ok := command.(render.DrawRect); ok && r.Fill == u.theme.Row.Selected {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the selected row to be drawn with the theme's Selected fill")
	}
}

// TestListInClipsRowContent guards against rows that are only partially
// inside the visible scroll window bleeding their content past the list's
// bounds — the bug reported after squad/calendar shipped with an unclipped
// List: the last (partially cut) row's text/badges drew past the panel edge.
func TestListInClipsRowContent(t *testing.T) {
	rowH := 20.0
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	rect := math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 100, Y: 50}) // 2.5 rows visible

	u.listIn("rows", rect, rowH, 10, -1, func(row int, rowRect math2d.Rect) {
		u.Emit(render.DrawText{Position: rowRect.Center(), Text: "row"})
	}, SpaceRef)

	// Every row-emitted command must carry the list's clip rect so a
	// partially-visible row is cut at the bounds instead of drawn in full.
	found := false
	for _, command := range u.Commands() {
		dt, ok := command.(render.DrawText)
		if !ok || dt.Text != "row" {
			continue
		}
		found = true
		if dt.Clip != rect {
			t.Fatalf("row text Clip = %v, want %v (the list's own bounds)", dt.Clip, rect)
		}
	}
	if !found {
		t.Fatal("expected at least one row's text command")
	}
}
