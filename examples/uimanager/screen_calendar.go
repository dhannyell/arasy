package uimanager

import (
	"fmt"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

const calendarRowH = 44.0

func calendarRoundsPanel() math2d.Rect    { return rect(190, 170, 820, 700) }
func calendarRoundsBodyRect() math2d.Rect { return panelBodyRect(calendarRoundsPanel()) }

func calendarNextPanel() math2d.Rect   { return rect(1050, 170, 680, 300) }
func calendarDetailPanel() math2d.Rect { return rect(1030, 500, 680, 370) }

func calendarSimButtonRect() math2d.Rect {
	p := calendarDetailPanel()
	return rect(p.Min.X+30, p.Max.Y-90, p.Width()-60, 56)
}

// totalRounds is the number of rounds in a single round-robin double
// fixture list: every club plays every other club home and away.
func totalRounds(clubCount int) int { return clubCount*2 - 2 }

// fixtureForClubInRound returns club's fixture in the given round. Every
// round-robin round has exactly one fixture per club.
func fixtureForClubInRound(fixtures []Fixture, round, club int) (Fixture, bool) {
	for _, fx := range fixtures {
		if fx.Round == round && (fx.Home == club || fx.Away == club) {
			return fx, true
		}
	}
	return Fixture{}, false
}

// resultFor returns the recorded result for fx, if it has been played.
func resultFor(results []Result, fx Fixture) (Result, bool) {
	for _, r := range results {
		if r.Round == fx.Round && r.Home == fx.Home && r.Away == fx.Away {
			return r, true
		}
	}
	return Result{}, false
}

func (g *Manager) updateCalendar(ctx *runtime.UpdateContext) {
	rounds := totalRounds(len(g.State.Clubs))
	// See the identical comment in updateSquad: Update applies wheel input,
	// Render's List call only re-reads/clamps the resulting offset.
	in := ctx.UI.In(ui.SpaceRef)
	in.ScrollOffset("lst_calendar", calendarRoundsBodyRect(), calendarRowH*float64(rounds))

	if in.Activated("btn_sim_cal", calendarSimButtonRect()) {
		SimulateRound(&g.State)
	}
	if seasonEnded(g.State) && in.Activated("btn_new_season_cal", calendarSimButtonRect()) {
		g.startNewSeason()
	}
}

func (g *Manager) drawCalendar(ctx *runtime.RenderContext) []render.Command {
	s := g.State
	cmds := make([]render.Command, 0, 160)
	cmds = append(cmds, drawCalendarRounds(s, ctx)...)
	cmds = append(cmds, drawCalendarNext(s, ctx)...)
	return cmds
}

func drawCalendarRounds(s ManagerState, ctx *runtime.RenderContext) []render.Command {
	panel := calendarRoundsPanel()
	rounds := totalRounds(len(s.Clubs))
	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab(fmt.Sprintf("SEASON 2026 · ROUNDS %d", rounds), panel.Min.X+24, panel.Min.Y-16, 320, grass)...)

	ctx.UI.In(ui.SpaceRef).List("lst_calendar", calendarRoundsBodyRect(), calendarRowH, rounds, -1, func(i int, rowRect math2d.Rect) {
		round := i + 1
		fx, ok := fixtureForClubInRound(s.Fixtures, round, 0)
		if !ok {
			return
		}

		accent, statusLabel := muted, "FUTURE"
		switch {
		case round < s.Round:
			accent, statusLabel = grass, "PLAYED"
		case round == s.Round:
			accent, statusLabel = gold, "TODAY"
		}
		ctx.UI.Emit(rectCmd(11, rect(rowRect.Min.X, rowRect.Min.Y+2, 3, rowRect.Height()-4), accent)...)

		home, away := s.Clubs[fx.Home], s.Clubs[fx.Away]
		ctx.UI.Emit(text(fmt.Sprintf("R%02d", round), v(rowRect.Min.X+30, rowRect.Center().Y), muted, 14, render.AlignLeft, render.BaselineMiddle, 12)...)

		matchX := rowRect.Min.X + 96
		if result, played := resultFor(s.Results, fx); played {
			letterCol := drawColor
			switch resultLetter(result, 0) {
			case "W":
				letterCol = grass
			case "L":
				letterCol = loss
			}
			ctx.UI.Emit(text(home.Short, v(matchX, rowRect.Center().Y), home.Accent, 16, render.AlignLeft, render.BaselineMiddle, 12)...)
			ctx.UI.Emit(text(fmt.Sprintf("%d x %d", result.HomeGoals, result.AwayGoals), v(matchX+90, rowRect.Center().Y), letterCol, 17, render.AlignCenter, render.BaselineMiddle, 12)...)
			ctx.UI.Emit(text(away.Short, v(matchX+180, rowRect.Center().Y), away.Accent, 16, render.AlignLeft, render.BaselineMiddle, 12)...)
		} else {
			ctx.UI.Emit(text(home.Short, v(matchX, rowRect.Center().Y), home.Accent, 16, render.AlignLeft, render.BaselineMiddle, 12)...)
			ctx.UI.Emit(text("vs", v(matchX+90, rowRect.Center().Y), muted, 14, render.AlignCenter, render.BaselineMiddle, 12)...)
			ctx.UI.Emit(text(away.Short, v(matchX+180, rowRect.Center().Y), away.Accent, 16, render.AlignLeft, render.BaselineMiddle, 12)...)
		}

		ctx.UI.Emit(badge(statusLabel, v(rowRect.Max.X-70, rowRect.Center().Y), bgRaised, 12)...)
	})

	return cmds
}

func drawCalendarNext(s ManagerState, ctx *runtime.RenderContext) []render.Command {
	next, hasNext := NextFixture(s, 0)
	cmds := drawVersusPanel("cal", calendarNextPanel(), s, next, hasNext, ctx,
		callToAction{id: "btn_sim_cal", rect: calendarSimButtonRect()},
		callToAction{id: "btn_new_season_cal", rect: calendarSimButtonRect()})

	detail := calendarDetailPanel()
	cmds = append(cmds, slantedPanel(detail, bgPanel, line, 1, 20, 8)...)
	cmds = append(cmds, titleTab("MATCH DETAILS", detail.Min.X+24, detail.Min.Y-16, 230, gold)...)

	if !hasNext {
		cmds = append(cmds, text("SEASON ENDED", detail.Center().Add(v(0, -60)), muted, 22, render.AlignCenter, render.BaselineMiddle, 12)...)
		return cmds
	}

	date, hour, stadium, weather := MatchInfo(&s, next)
	rows := []struct{ label, value string }{
		{"DATE", date},
		{"TIME", hour},
		{"STADIUM", stadium},
		{"WEATHER", weather},
	}
	for i, row := range rows {
		y := detail.Min.Y + 56 + float64(i)*48
		cmds = append(cmds, text(row.label, v(detail.Min.X+32, y), muted, 13, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(row.value, v(detail.Max.X-32, y), textColor, 18, render.AlignRight, render.BaselineMiddle, 12)...)
	}

	return cmds
}
