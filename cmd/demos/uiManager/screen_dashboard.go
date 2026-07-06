package uiManager

import (
	"fmt"

	"github.com/dhannyell/frameforge/render"
)

func (g *Manager) drawDashboard() []render.Command {
	s := g.State
	cmds := make([]render.Command, 0, 130)
	next, hasNext := NextFixture(s, 0)

	cmds = append(cmds, drawVersusCard(s, next, hasNext)...)
	cmds = append(cmds, drawPositionCard(s)...)
	cmds = append(cmds, drawFormCard(s)...)
	cmds = append(cmds, drawLastResultCard(s)...)
	cmds = append(cmds, drawTopFiveCard(s)...)
	cmds = append(cmds, drawAgendaCard(s)...)
	return cmds
}

func drawVersusCard(s ManagerState, fx Fixture, ok bool) []render.Command {
	panel := rect(190, 170, 820, 360)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("NEXT MATCH", panel.Min.X+28, panel.Min.Y-16, 280, grass)...)
	if !ok {
		cmds = append(cmds, text("SEASON ENDED", panel.Center(), textColor, 34, render.AlignCenter, render.BaselineMiddle, 12)...)
		return cmds
	}

	home := s.Clubs[fx.Home]
	away := s.Clubs[fx.Away]
	arena := home.Stadium
	homeAway := "HOME"
	if fx.Away == 0 {
		homeAway = "AWAY"
		arena = home.Stadium
	}

	split := rect(panel.Min.X+42, panel.Min.Y+68, panel.Width()-84, 150)
	offset := 70.0
	topRight := v(split.Center().X+offset, split.Min.Y)
	bottomLeft := v(split.Center().X-offset, split.Max.Y)
	cmds = append(cmds,
		poly(9, home.Accent, nil,
			v(split.Min.X, split.Min.Y),
			topRight,
			bottomLeft,
			v(split.Min.X, split.Max.Y),
		),
		poly(9, away.Accent, nil,
			topRight,
			v(split.Max.X, split.Min.Y),
			v(split.Max.X, split.Max.Y),
			bottomLeft,
		),
		circle(11, split.Center(), 54, bgDeep, textColor, 2),
	)
	cmds = append(cmds, text(home.Short, v(split.Min.X+150, split.Center().Y), textColor, 48, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text("VS", split.Center(), gold, 30, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(away.Short, v(split.Max.X-150, split.Center().Y), textColor, 48, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, badge(fmt.Sprintf("ROUND %02d", fx.Round), v(panel.Min.X+120, panel.Max.Y-90), grass, 12)...)
	cmds = append(cmds, badge(homeAway, v(panel.Min.X+285, panel.Max.Y-90), bgRaised, 12)...)
	cmds = append(cmds, text(arena, v(panel.Min.X+56, panel.Max.Y-46), muted, 16, render.AlignLeft, render.BaselineMiddle, 12)...)
	cmds = append(cmds, slantedPanel(simButtonRect(), gold, render.RGBA(0, 0, 0, 0), 0, 16, 12)...)
	cmds = append(cmds, text("SIMULATE ROUND", simButtonRect().Center(), bgDeep, 18, render.AlignCenter, render.BaselineMiddle, 13)...)
	return cmds
}

func drawPositionCard(s ManagerState) []render.Command {
	panel := rect(1050, 170, 315, 360)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("POSITION", panel.Min.X+24, panel.Min.Y-16, 180, gold)...)
	rows := Standings(s.Clubs, s.Results)
	pos, row := 1, TableRow{Club: 0}
	for i, r := range rows {
		if r.Club == 0 {
			pos, row = i+1, r
			break
		}
	}
	cmds = append(cmds, text(fmt.Sprintf("%d°", pos), v(panel.Min.X+64, panel.Min.Y+132), gold, 72, render.AlignLeft, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(fmt.Sprintf("%d PTS", row.Points), v(panel.Min.X+194, panel.Min.Y+114), textColor, 28, render.AlignLeft, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(fmt.Sprintf("GD %+d", row.GoalDiff()), v(panel.Min.X+196, panel.Min.Y+156), muted, 18, render.AlignLeft, render.BaselineMiddle, 12)...)
	cmds = append(cmds, drawSegmentBar(rect(panel.Min.X+42, panel.Max.Y-92, 230, 18), float64(maxInt(row.Points, 1))/42, grass, 10)...)
	cmds = append(cmds, text("META: TOP 2", v(panel.Min.X+42, panel.Max.Y-48), muted, 13, render.AlignLeft, render.BaselineMiddle, 12)...)
	return cmds
}

func drawFormCard(s ManagerState) []render.Command {
	panel := rect(1400, 170, 330, 360)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("PERFORMANCE", panel.Min.X+24, panel.Min.Y-16, 160, grass)...)
	letters := lastFiveLetters(s, 0)
	for i, letter := range letters {
		col := drawColor
		switch letter {
		case "W":
			col = grass
		case "L":
			col = loss
		case "D":
			col = drawColor
		}
		cmds = append(cmds, circle(10, v(panel.Min.X+62+float64(i)*54, panel.Min.Y+112), 21, col, render.Color{}, 0))
		cmds = append(cmds, text(letter, v(panel.Min.X+62+float64(i)*54, panel.Min.Y+112), bgDeep, 16, render.AlignCenter, render.BaselineMiddle, 11)...)
	}
	cmds = append(cmds, drawSegmentBar(rect(panel.Min.X+44, panel.Min.Y+174, 244, 22), formScore(letters), grass, 12)...)
	cmds = append(cmds, text("LAST 5 GAMES", v(panel.Min.X+44, panel.Min.Y+230), muted, 14, render.AlignLeft, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(Morale(s.Clubs[0]), v(panel.Min.X+44, panel.Min.Y+282), gold, 34, render.AlignLeft, render.BaselineMiddle, 12)...)
	return cmds
}

func drawLastResultCard(s ManagerState) []render.Command {
	panel := rect(190, 570, 500, 260)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("LATEST RESULT", panel.Min.X+28, panel.Min.Y-16, 270, grass)...)
	result, ok := LastResult(s, 0)
	if !ok {
		cmds = append(cmds, text("AWAITING PREMIERE", panel.Center(), muted, 24, render.AlignCenter, render.BaselineMiddle, 12)...)
		return cmds
	}
	home := s.Clubs[result.Home]
	away := s.Clubs[result.Away]
	cmds = append(cmds, text(home.Short, v(panel.Min.X+94, panel.Center().Y), home.Accent, 34, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(fmt.Sprintf("%d  x  %d", result.HomeGoals, result.AwayGoals), panel.Center(), textColor, 54, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(away.Short, v(panel.Max.X-94, panel.Center().Y), away.Accent, 34, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text(fmt.Sprintf("ROUND %02d", result.Round), v(panel.Min.X+44, panel.Max.Y-42), muted, 14, render.AlignLeft, render.BaselineMiddle, 12)...)
	return cmds
}

func drawTopFiveCard(s ManagerState) []render.Command {
	panel := rect(730, 570, 405, 260)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("TOP 5", panel.Min.X+24, panel.Min.Y-16, 130, gold)...)
	rows := Standings(s.Clubs, s.Results)
	for i := 0; i < 5 && i < len(rows); i++ {
		r := rows[i]
		y := panel.Min.Y + 58 + float64(i)*36
		fill := render.RGBA(1, 1, 1, 0)
		if r.Club == 0 {
			fill = render.RGBA(0.94, 0.73, 0.23, 0.14)
		} else if i%2 == 1 {
			fill = zebra
		}
		cmds = append(cmds, rectCmd(9, rect(panel.Min.X+26, y-17, panel.Width()-52, 32), fill)...)
		cmds = append(cmds, text(fmt.Sprintf("%d", i+1), v(panel.Min.X+50, y), muted, 15, render.AlignRight, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(s.Clubs[r.Club].Short, v(panel.Min.X+78, y), s.Clubs[r.Club].Accent, 16, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(s.Clubs[r.Club].Name, v(panel.Min.X+132, y), textColor, 14, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(fmt.Sprintf("%d", r.Points), v(panel.Max.X-40, y), gold, 16, render.AlignRight, render.BaselineMiddle, 12)...)
	}
	return cmds
}

func drawAgendaCard(s ManagerState) []render.Command {
	panel := rect(1175, 570, 555, 260)
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab("AGENDA", panel.Min.X+24, panel.Min.Y-16, 160, grass)...)
	count := 0
	for _, fx := range s.Fixtures {
		if fx.Round < s.Round || (fx.Home != 0 && fx.Away != 0) {
			continue
		}
		y := panel.Min.Y + 64 + float64(count)*54
		opp := fx.Away
		where := "HOME"
		whereColor := grass
		if fx.Away == 0 {
			opp = fx.Home
			where = "AWAY"
			whereColor = loss
		}
		cmds = append(cmds, rectCmd(9, rect(panel.Min.X+30, y-22, panel.Width()-60, 42), zebra)...)
		cmds = append(cmds, text(fmt.Sprintf("R%02d", fx.Round), v(panel.Min.X+58, y), gold, 15, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(s.Clubs[opp].Short, v(panel.Min.X+142, y), s.Clubs[opp].Accent, 20, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(where, v(panel.Max.X-64, y), whereColor, 15, render.AlignRight, render.BaselineMiddle, 12)...)
		count++
		if count == 3 {
			break
		}
	}
	if count == 0 {
		cmds = append(cmds, text("SEASON ENDED", panel.Center(), muted, 22, render.AlignCenter, render.BaselineMiddle, 12)...)
	}
	return cmds
}
