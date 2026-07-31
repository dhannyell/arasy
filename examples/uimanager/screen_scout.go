package uimanager

import (
	"fmt"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

// Layout of the scout screen: one wide panel with a filter row on top and the
// market table filling the rest.
func scoutPanel() math2d.Rect { return rect(190, 170, 1550, 690) }
func scoutSearchRect() math2d.Rect {
	p := scoutPanel()
	return rect(p.Min.X+20, p.Min.Y+24, 460, 40)
}
func scoutFilterRect() math2d.Rect {
	search := scoutSearchRect()
	return rect(search.Max.X+20, search.Min.Y, 260, 40)
}
func scoutU23Rect() math2d.Rect {
	filter := scoutFilterRect()
	return rect(filter.Max.X+24, filter.Min.Y+6, 200, 28)
}
func scoutTableRect() math2d.Rect {
	p := scoutPanel()
	search := scoutSearchRect()
	return rect(p.Min.X+20, search.Max.Y+20, p.Width()-40, p.Max.Y-search.Max.Y-40)
}

func scoutColumns() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: "NAME", Width: 420},
		{Title: "CLUB", Width: 160},
		{Title: "POS", Width: 120},
		{Title: "AGE", Width: 140, Align: render.AlignRight},
		{Title: "OVR", Align: render.AlignRight},
	}
}

func (g *Manager) updateScout(ctx *runtime.UpdateContext) {
	in := ctx.UI.In(ui.SpaceRef)
	s := &g.State

	if result := in.TextInput("scout_search", scoutSearchRect(), s.ScoutQuery, ui.TextInputOptions{
		Placeholder: "SEARCH PLAYER",
		MaxRunes:    24,
	}); result.Changed {
		s.ScoutQuery = result.Text
		s.ScoutSelected = -1
	}

	if next, changed := in.DropdownInput("scout_filter", scoutFilterRect(), len(scoutFilters), s.ScoutFilter); changed {
		s.ScoutFilter = next
		s.ScoutSelected = -1
	}

	if next, changed := in.CheckboxInput("scout_u23", scoutU23Rect(), s.ScoutOnlyU23); changed {
		s.ScoutOnlyU23 = next
		s.ScoutSelected = -1
	}

	// Sorting is the game's: the widget reports which header was clicked, and
	// what that means for these rows is decided here.
	if col := in.TableHeaderHit("tbl_scout", scoutTableRect(), scoutColumns()); col >= 0 {
		s.ScoutSortBy = toggleScoutSort(s.ScoutSortBy, col)
	}

	rows := g.scoutVisibleRows()
	in.ScrollOffset("tbl_scout", ctx.UI.TableBodyRect(scoutTableRect()), float64(len(rows))*ctx.UI.RowHeight())
	if next, changed := in.TableInputValue("tbl_scout", scoutTableRect(), scoutColumns(), len(rows), s.ScoutSelected); changed {
		s.ScoutSelected = next
	}
}

// scoutVisibleRows is the filtered, ordered view of the market. Both Update and
// Render derive it from the same state instead of caching a slice, so the two
// phases cannot disagree about which row is row 3.
func (g *Manager) scoutVisibleRows() []int {
	return scoutRows(g.State.ScoutPool, g.State.ScoutQuery, g.State.ScoutFilter, g.State.ScoutOnlyU23, g.State.ScoutSortBy)
}

func (g *Manager) drawScout(ctx *runtime.RenderContext) []render.Command {
	s := g.State
	panel := scoutPanel()
	rows := g.scoutVisibleRows()

	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab(fmt.Sprintf("MARKET (%d)", len(rows)), panel.Min.X+24, panel.Min.Y-16, 200, grass)...)

	in := ctx.UI.In(ui.SpaceRef)
	in.TextInput("scout_search", scoutSearchRect(), s.ScoutQuery, ui.TextInputOptions{
		Placeholder: "SEARCH PLAYER",
		MaxRunes:    24,
	})
	in.Checkbox("scout_u23", scoutU23Rect(), s.ScoutOnlyU23, "U23 ONLY")

	in.TableSorted("tbl_scout", scoutTableRect(), scoutColumns(), len(rows), s.ScoutSelected,
		ui.TableSort{Col: s.ScoutSortBy.Col, Desc: s.ScoutSortBy.Desc},
		func(row, col int) string { return scoutCell(s.ScoutPool, rows, row, col) })

	// The dropdown is declared last so its open list draws over the table —
	// the overlay layer block puts it there regardless, and declaring it here
	// keeps the reading order the same as the drawing order.
	in.Dropdown("scout_filter", scoutFilterRect(), scoutFilters, s.ScoutFilter)

	return cmds
}

func scoutCell(pool []ScoutEntry, rows []int, row, col int) string {
	if row < 0 || row >= len(rows) {
		return ""
	}
	entry := pool[rows[row]]
	switch col {
	case scoutColClub:
		return entry.Club
	case scoutColPos:
		return posAbbr(entry.Player.Pos)
	case scoutColAge:
		return fmt.Sprintf("%d", entry.Player.Age)
	case scoutColRating:
		return fmt.Sprintf("%d", entry.Rating)
	default:
		return entry.Player.Name
	}
}
