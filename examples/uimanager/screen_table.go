package uimanager

import (
	"fmt"
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

func (g *Manager) drawLeagueTable(ctx *runtime.RenderContext) []render.Command {
	s := g.State
	cmds := make([]render.Command, 0, 160)
	cmds = append(cmds, drawStandingsPanel(s)...)
	cmds = append(cmds, drawStandingsTable(s, ctx)...)
	cmds = append(cmds, drawRoundCard(s)...)
	cmds = append(cmds, drawNextMatchCard(s, ctx)...)
	cmds = append(cmds, drawLegendCard()...)
	return cmds
}

// standingsPanel + standingsTableRect are the single source of geometry for
// the league table screen, shared by Update (standingsBodyRect in game.go)
// and both Render helpers below — duplicating these rects per function let
// them silently drift out of sync with each other.
func standingsPanel() math2d.Rect { return rect(190, 170, 740, 690) }
func standingsTableRect() math2d.Rect {
	p := standingsPanel()
	return rect(p.Min.X+20, p.Min.Y+52, p.Width()-40, p.Height()-72)
}

// Right column: three stacked cards (plans/implementacao/021 §3.3 — RODADA,
// PRÓXIMO JOGO, legenda TOPO/RISCO), sized to fill the same y=170..860 band
// the left-column panels use elsewhere in this demo.
func tableRoundPanel() math2d.Rect     { return rect(970, 170, 770, 200) }
func tableNextMatchPanel() math2d.Rect { return rect(970, 390, 770, 300) }
func tableLegendPanel() math2d.Rect    { return rect(970, 710, 770, 150) }
func tableNewSeasonButtonRect() math2d.Rect {
	p := tableNextMatchPanel()
	return rect(p.Min.X+180, p.Max.Y-88, p.Width()-360, 56)
}

func drawRoundCard(s ManagerState) []render.Command {
	panel := tableRoundPanel()
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("ROUND", panel.Min.X+24, panel.Min.Y-16, 150, grass)...)
	round := min(s.Round, totalRounds(len(s.Clubs)))
	cmds = append(cmds, text(fmt.Sprintf("%d", round), panel.Center().Add(v(0, 14)), textColor, 90, render.AlignCenter, render.BaselineMiddle, 12)...)
	return cmds
}

func drawNextMatchCard(s ManagerState, ctx *runtime.RenderContext) []render.Command {
	panel := tableNextMatchPanel()
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("NEXT MATCH", panel.Min.X+24, panel.Min.Y-16, 190, grass)...)

	fx, ok := NextFixture(s, 0)
	if !ok {
		cmds = append(cmds, text("SEASON ENDED", panel.Center().Add(v(0, -34)), muted, 20, render.AlignCenter, render.BaselineMiddle, 12)...)
		cmds = append(cmds, drawSeasonEndedButton("table",
			callToAction{id: "btn_new_season_table", rect: tableNewSeasonButtonRect()}, ctx)...)
		return cmds
	}

	home, away := s.Clubs[fx.Home], s.Clubs[fx.Away]
	vsY := panel.Min.Y + 90
	cmds = append(cmds, text(home.Short, v(panel.Center().X-90, vsY), home.Accent, 36, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text("VS", v(panel.Center().X, vsY), gold, 18, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(away.Short, v(panel.Center().X+90, vsY), away.Accent, 36, render.AlignCenter, render.BaselineMiddle, 12)...)

	homeAway := "HOME"
	if fx.Away == 0 {
		homeAway = "AWAY"
	}
	cmds = append(cmds, text(homeAway, v(panel.Center().X, vsY+56), muted, 15, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, drawStadiumGlyph(v(panel.Center().X, vsY+108), 20, grass)...)
	return cmds
}

func drawLegendCard() []render.Command {
	panel := tableLegendPanel()
	cmds := slantedPanel(panel, bgPanel, line, 1, 16, 8)

	const swatchW, swatchH = 36.0, 14.0
	rows := []struct {
		label string
		col   render.Color
		y     float64
	}{
		{"TOP", grass, panel.Min.Y + 46},
		{"RISK", loss, panel.Min.Y + 100},
	}
	for _, r := range rows {
		cmds = append(cmds, rectCmd(9, rect(panel.Min.X+30, r.y-swatchH/2, swatchW, swatchH), r.col)...)
		cmds = append(cmds, text(r.label, v(panel.Min.X+80, r.y), textColor, 16, render.AlignLeft, render.BaselineMiddle, 9)...)
	}
	return cmds
}

// drawStadiumGlyph is a minimal flat pictogram (bowl curve + rim line) per
// plans/implementacao/021 §3.6: keep icons legible in a handful of draw
// commands, fall back to text rather than chase pixel-perfect fidelity.
func drawStadiumGlyph(center math2d.Vec2, size float64, col render.Color) []render.Command {
	return []render.Command{
		render.DrawArc{
			BaseCommand: render.BaseCommand{LayerValue: 13},
			Center:      center,
			Radius:      size,
			StartAngle:  math.Pi * 0.15,
			EndAngle:    math.Pi * 0.85,
			Width:       3,
			Color:       col,
		},
		lineCmd(13, v(center.X-size*0.8, center.Y-size*0.1), v(center.X+size*0.8, center.Y-size*0.1), 2, col),
	}
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
	offset, clip := ctx.UI.In(ui.SpaceRef).ScrollArea("tbl_standings", bodyRect, contentH)

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

	_ = ctx.UI.In(ui.SpaceRef).TableStyled("tbl_standings", tableRect, cols, len(rows), selected, cell, style)

	const badgeW = 46.0
	const barW = 3.0

	cmds := make([]render.Command, 0, len(rows)*2)
	for i, r := range rows {
		y := bodyRect.Min.Y + float64(i)*rowH - offset

		// Culling: skip rows outside the visible clip area.
		if y+rowH < clip.Min.Y || y > clip.Max.Y {
			continue
		}

		// This overlay draws straight into cmds instead of going through
		// ui.List, so a row only partially inside the visible window (the
		// same edge case ui.List clips internally) isn't clipped by
		// anything upstream — clip it here so it doesn't bleed past clip.
		start := len(cmds)

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

		render.ClipCommands(cmds[start:], clip)
	}

	return cmds
}
