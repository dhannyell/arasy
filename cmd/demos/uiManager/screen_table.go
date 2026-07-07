package uiManager

import (
	"fmt"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
)

func (g *Manager) drawLeagueTable(ctx *runtime.RenderContext) []render.Command {
	s := g.State
	cmds := make([]render.Command, 0, 130)
	cmds = append(cmds, drawStandingsPanel(s)...)
	cmds = append(cmds, drawStandingsTable(s, ctx)...)
	return cmds
}

// standingsPanel + standingsTableRect are the single source of geometry for
// the league table screen, shared by Update (standingsBodyRect in game.go)
// and both Render helpers below — duplicating these rects per function let
// them silently drift out of sync with each other.
func standingsPanel() math2d.Rect { return rect(190, 170, 820, 690) }
func standingsTableRect() math2d.Rect {
	p := standingsPanel()
	return rect(p.Min.X+20, p.Min.Y+52, p.Width()-40, p.Height()-72)
}

func drawStandingsPanel(s ManagerState) []render.Command {
	panel := standingsPanel()
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("LEAGUE TABLE", panel.Min.X+24, panel.Min.Y-16, 230, grass)...)
	return cmds
}

func drawStandingsTable(s ManagerState, ctx *runtime.RenderContext) []render.Command {
	cols := []ui.TableColumn{
		{Title: "#", Width: 32, Align: render.AlignRight},
		{Title: "", Width: 54, Align: render.AlignLeft},
		{Title: "CLUB", Width: 0, Align: render.AlignLeft}, // flex
		{Title: "PTS", Width: 64, Align: render.AlignRight},
		{Title: "G", Width: 48, Align: render.AlignRight},
		{Title: "W", Width: 48, Align: render.AlignRight},
		{Title: "D", Width: 48, Align: render.AlignRight},
		{Title: "L", Width: 48, Align: render.AlignRight},
		{Title: "GD", Width: 64, Align: render.AlignRight},
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

	tableRect := standingsTableRect()
	bodyRect := standingsBodyRect(ctx.UI)
	rowH := ctx.UI.RowHeight()
	contentH := float64(len(rows)) * rowH

	// Get the current scroll offset and clip rect (same ID used in Update).
	offset, clip := ctx.UI.ScrollAreaIn("tbl_standings", bodyRect, contentH, ui.SpaceRef)

	style := func(row, col int) ui.TableCellStyle {
		r := rows[row]
		switch col {
		case 3: // PTS
			if r.Club == userClub {
				return ui.TableCellStyle{Color: gold}
			}
		case 5: // W
			if r.Wins > 0 {
				return ui.TableCellStyle{Color: grass}
			}
		case 7: // L
			if r.Loss > 0 {
				return ui.TableCellStyle{Color: loss}
			}
		}
		return ui.TableCellStyle{}
	}

	cell := func(row, col int) string {
		r := rows[row]
		switch col {
		case 0:
			return fmt.Sprintf("%d", row+1)
		case 1:
			return ""
		case 2:
			return s.Clubs[r.Club].Name
		case 3:
			return fmt.Sprintf("%d", r.Points)
		case 4:
			return fmt.Sprintf("%d", r.Played)
		case 5:
			return fmt.Sprintf("%d", r.Wins)
		case 6:
			return fmt.Sprintf("%d", r.Draws)
		case 7:
			return fmt.Sprintf("%d", r.Loss)
		case 8:
			gd := r.GoalDiff()
			if gd > 0 {
				return fmt.Sprintf("+%d", gd)
			}
			return fmt.Sprintf("%d", gd)
		default:
			return ""
		}
	}

	_ = ctx.UI.TableStyledIn("tbl_standings", tableRect, cols, len(rows), selected, cell, style, ui.SpaceRef)

	const badgeW = 46.0
	const barW = 3.0

	cmds := make([]render.Command, 0, len(rows)*2)
	for i, r := range rows {
		y := bodyRect.Min.Y + float64(i)*rowH - offset

		// Culling: skip rows outside the visible clip area.
		if y+rowH < clip.Min.Y || y > clip.Max.Y {
			continue
		}

		// ── position bar (promotion/relegation) ──
		if i < 2 || i >= len(rows)-2 {
			col := grass
			if i >= len(rows)-2 {
				col = loss
			}
			barR := rect(tableRect.Min.X+1, y+2, barW, rowH-4)
			cmds = append(cmds, rectCmd(15, barR, col)...)
		}

		// ── club abbreviation badge ──
		club := s.Clubs[r.Club]
		bx := tableRect.Min.X + 32
		badgeR := rect(bx+4, y+4, badgeW, rowH-8)
		cmds = append(cmds, slantedPanel(badgeR, club.Accent, render.Color{}, 0, 4, 13)...)
		cmds = append(cmds, text(club.Short, badgeR.Center(), bgDeep, 11, render.AlignCenter, render.BaselineMiddle, 13)...)
	}

	return cmds
}
