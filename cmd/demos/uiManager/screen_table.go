package uiManager

import (
	"fmt"

	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
)

func (g *Manager) drawLeagueTable(ctx *runtime.RenderContext) []render.Command {
	s := g.State
	cmds := make([]render.Command, 0, 130)
	cmds = append(cmds, drawStandingsPanel(s)...)
	cmds = append(cmds, drawStandingsTable(s, ctx)...)
	// cmds = append(cmds, drawRoundCard(s)...)
	// cmds = append(cmds, drawNextMatchCard(s)...)
	// cmds = append(cmds, drawLegendCard()...)
	return cmds
}

func drawStandingsPanel(s ManagerState) []render.Command {
	panel := rect(190, 170, 820, 690)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("LEAGUE TABLE", panel.Min.X+24, panel.Min.Y-16, 230, grass)...)
	return cmds
}

func drawStandingsTable(s ManagerState, ctx *runtime.RenderContext) []render.Command {
	panel := rect(190, 170, 820, 690)
	cols := []ui.TableColumn{
		{Title: "#", Width: 48, Align: render.AlignRight},
		{Title: "CLUB", Width: 0, Align: render.AlignLeft}, // flex
		{Title: "PTS", Width: 78, Align: render.AlignRight},
		{Title: "G", Width: 60, Align: render.AlignRight},
		{Title: "W", Width: 60, Align: render.AlignRight},
		{Title: "D", Width: 60, Align: render.AlignRight},
		{Title: "L", Width: 60, Align: render.AlignRight},
		{Title: "GD", Width: 78, Align: render.AlignRight},
	}

	rows := Standings(s.Clubs, s.Results)

	const userClub = 0
	selected := -1
	for i, r := range rows {
		if r.Club == userClub {
			selected = i
			break
		}
	}

	tableRect := rect(panel.Min.X+20, panel.Min.Y+52, panel.Width()-40, panel.Height()-72)
	tableBodyRect := rect(tableRect.Min.X, tableRect.Min.Y+36, tableRect.Width(), tableRect.Height()-36)

	style := func(row, col int) ui.TableCellStyle {
		r := rows[row]
		switch col {
		case 2: // PTS
			if r.Club == userClub {
				return ui.TableCellStyle{Color: gold}
			}
		case 4: // W
			if r.Wins > 0 {
				return ui.TableCellStyle{Color: grass}
			}
		case 6: // L
			if r.Loss > 0 {
				return ui.TableCellStyle{Color: loss}
			}
		}
		return ui.TableCellStyle{}
	}

	cell := func(row, col int) string {
		if col == 0 {
			return fmt.Sprintf("%d", row+1)
		}
		return rows[row].Cell(col, s.Clubs)
	}

	_ = ctx.UI.TableStyledIn("tbl_standings", tableRect, cols, len(rows), selected, cell, style, ui.SpaceRef)

	cmds := make([]render.Command, 0, len(rows))
	cmds = append(cmds, lineCmd(20, tableBodyRect.Min, v(tableBodyRect.Max.X, tableBodyRect.Min.Y), 2, render.RGBA(1, 0, 0, 0.5)))
	cmds = append(cmds, lineCmd(20, v(tableBodyRect.Max.X, tableBodyRect.Min.Y), tableBodyRect.Max, 2, render.RGBA(1, 0, 0, 0.5)))
	cmds = append(cmds, lineCmd(20, tableBodyRect.Max, v(tableBodyRect.Min.X, tableBodyRect.Max.Y), 2, render.RGBA(1, 0, 0, 0.5)))
	cmds = append(cmds, lineCmd(20, v(tableBodyRect.Min.X, tableBodyRect.Max.Y), tableBodyRect.Min, 2, render.RGBA(1, 0, 0, 0.5)))
	const barW = 3.0
	headerH := 36.0
	rowH := 32.0
	barX := tableRect.Min.X
	for i := range rows {
		y := tableRect.Min.Y + headerH + float64(i)*rowH
		var col render.Color
		switch i {
		case 0, 1:
			col = grass
		case len(rows) - 2, len(rows) - 1:
			col = loss
		default:
			continue
		}
		cmds = append(cmds, rectCmd(15, rect(barX, y+2, barW, rowH-4), col)...)
	}

	return cmds
}

func drawRound(s ManagerState) []render.Command {
	panel := rect(190, 170, 820, 360)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("ROUND", panel.Min.X+24, panel.Min.Y-16, 180, gold)...)

	return cmds
}
