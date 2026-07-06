package ui

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestResolveTableWidthsDistributesFlexColumns(t *testing.T) {
	widths := resolveTableWidths(300, []TableColumn{
		{Width: 80},
		{},
		{},
	})

	want := []float64{80, 110, 110}
	for i := range want {
		if widths[i] != want[i] {
			t.Fatalf("widths[%d] = %v, want %v", i, widths[i], want[i])
		}
	}
}

func TestTableHitUsesScrollOffsetForClickedRow(t *testing.T) {
	state := &State{scroll: map[ID]float64{HashID("players"): 64}}
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 200, Y: 100})
	cam := testInteractCamera()
	theme := DefaultTheme()
	downUp := []input.PointerEvent{
		{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 20, Y: 52}},
		{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 20, Y: 52}},
	}
	u := NewUpdate(state, pointerFrame(downUp...), cam, theme, cam.Viewport)

	row := u.TableHitIn("players", rect, []TableColumn{{Title: "Name"}}, 20, -1, SpaceRef)
	if row != 2 {
		t.Fatalf("clicked row = %v, want 2", row)
	}
}

func TestTableInClipsBodyRows(t *testing.T) {
	state := &State{}
	cam := testCamera()
	store := AnimStore{}
	u := New(state, math2d.Vec2{X: 20, Y: 70}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 80, Y: 90})

	u.TableIn("players", rect, []TableColumn{{Title: "Name"}}, 3, 1, func(row, col int) string {
		return "P"
	}, SpaceRef)

	body := tableBodyRect(rect, u.theme.Header.Height)
	clip := body
	foundBodyClip := false
	for _, command := range u.Commands() {
		switch c := command.(type) {
		case render.DrawText:
			if c.Text == "P" && c.Clip == clip {
				foundBodyClip = true
			}
		case render.DrawRect:
			if c.Fill == u.theme.Row.Selected && c.Clip == clip {
				foundBodyClip = true
			}
		}
	}
	if !foundBodyClip {
		t.Fatal("expected body row commands to carry the table body clip")
	}
}

func TestCardAndBadgeEmitCommands(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)

	u.Card(math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 50, Y: 30}), render.RGBA(1, 0, 0, 1))
	u.Badge("GK", math2d.Vec2{X: 50, Y: 50}, render.Color{})

	if len(u.Commands()) != 4 {
		t.Fatalf("commands = %d, want card fill/accent and badge rect/text", len(u.Commands()))
	}
}
