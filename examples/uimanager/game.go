// Package uimanager is the "Matchday" football manager demo: a dashboard,
// league table, squad, and calendar screen driven entirely by deterministic,
// headless data (see data.go and sim.go) — no scene simulation, no physics.
// It exists to stress the engine's UI/runtime axis (dense data screens,
// navigation, scroll, selection, rewind of UI state) the way football and
// breakout never do.
package uimanager

import (
	"fmt"
	"math"

	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

const (
	viewW = 1920
	viewH = 1080
)

const initialSeasonSeed = 20260706
const initialSeasonYear = 2026

type Manager struct {
	runtime.StateSlot[ManagerState]
	UIState ManagerUIState
}

func (g *Manager) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    rect(0, 0, viewW, viewH),
			Viewport: rect(0, 0, viewW, viewH),
			Aspect:   render.AspectFit,
		},
		Clear:         bgDeep,
		Bounds:        rect(0, 0, viewW, viewH),
		SimulationHz:  60,
		InputBindings: matchdayBindings(),
		FPS:           60,
		HistoryLen:    600,
		UITheme:       matchdayTheme(),
	}
}

func (g *Manager) Init(ctx *runtime.UpdateContext) {
	_ = ctx
	clubs, fixtures := GenerateLeague(initialSeasonSeed)
	g.State = ManagerState{
		Screen:         ScreenDashboard,
		Seed:           initialSeasonSeed,
		SeasonYear:     initialSeasonYear,
		Round:          1,
		Clubs:          clubs,
		Fixtures:       fixtures,
		SelectedPlayer: -1,
	}
	initSquadOrder(g.State.Clubs[0].Squad)
}

func (g *Manager) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()

	in := ctx.UI.In(ui.SpaceRef)
	for i, item := range navItems {
		if in.Clicked(item.id, item.rect) {
			if g.State.Screen != Screen(i) {
				g.State.PreviousScreen = g.State.Screen
				g.State.Screen = Screen(i)
				g.UIState.SelectedNavItem = i
				g.UIState.NavPulseTick = ctx.Tick
			}
		}
	}

	if ctx.Tick-g.UIState.NavPulseTick > 30 {
		g.UIState.SelectedNavItem = -1
	}

	if in.Clicked("btn_sim_dashboard", simButtonRect()) {
		SimulateRound(&g.State)
	}
	if seasonEnded(g.State) && in.Clicked("btn_new_season_dash", simButtonRect()) {
		g.startNewSeason()
	}
	if g.State.Screen == ScreenTable {
		in.ScrollOffset("tbl_standings", standingsBodyRect(ctx.UI), float64(len(g.State.Clubs))*ctx.UI.RowHeight())
		if seasonEnded(g.State) && in.Clicked("btn_new_season_table", tableNewSeasonButtonRect()) {
			g.startNewSeason()
		}
	}
	if g.State.Screen == ScreenSquad {
		g.updateSquad(ctx)
	}
	if g.State.Screen == ScreenCalendar {
		g.updateCalendar(ctx)
	}

	g.checkPressedButtons(ctx)
}

func (g *Manager) startNewSeason() {
	seasonRNG := runtime.NewRand(g.State.Seed).Derive("next-season")
	nextSeed := seasonRNG.Uint64()
	nextYear := g.State.SeasonYear + 1
	screen := g.State.Screen
	previous := g.State.PreviousScreen
	clubs, fixtures := GenerateLeague(nextSeed)
	g.State = ManagerState{
		Screen:         screen,
		PreviousScreen: previous,
		Seed:           nextSeed,
		SeasonYear:     nextYear,
		Round:          1,
		Clubs:          clubs,
		Fixtures:       fixtures,
		SelectedPlayer: -1,
	}
	initSquadOrder(g.State.Clubs[0].Squad)
}

func (g *Manager) checkPressedButtons(ctx *runtime.UpdateContext) {
	if !ctx.Input.JustPressed(input.ActionMoveX) {
		return
	}
	switch {
	case ctx.Input.Value(input.ActionMoveX) < 0:
		g.cycleScreen(-1, ctx.Tick)
	case ctx.Input.Value(input.ActionMoveX) > 0:
		g.cycleScreen(1, ctx.Tick)
	}
}

// cycleScreen moves the active screen by delta, wrapping around len(navItems)
// in either direction.
func (g *Manager) cycleScreen(delta int, tick uint64) {
	g.State.PreviousScreen = g.State.Screen
	g.State.Screen = Screen((int(g.State.Screen) + delta + len(navItems)) % len(navItems))
	g.UIState.SelectedNavItem = int(g.State.Screen)
	g.UIState.NavPulseTick = tick
}

func (g *Manager) Render(ctx *runtime.RenderContext) []render.Command {
	cmds := make([]render.Command, 0)
	cmds = append(cmds, rectCmd(0, rect(0, 0, viewW, viewH), bgDeep)...)
	cmds = append(cmds, drawTacticalMotif()...)
	cmds = append(cmds, g.drawNavAnimated(g.State.Screen, ctx)...)
	cmds = append(cmds, drawScreenTitle(screenStateToName(g.State.Screen), g.State.Clubs[0].Name)...)

	transitionKind := ui.TransitionSlideLeft
	if g.State.Screen < g.State.PreviousScreen {
		transitionKind = ui.TransitionSlideRight
	}

	tr := ctx.UI.ScreenTransition("manager_screen", stateScreenKey(g.State.Screen), ui.TransitionOptions{
		Kind:     transitionKind,
		Duration: 0.2,
		Ease:     ease.OutCubic,
	})

	if _, ok := tr.Outgoing(); ok {
		ctx.UI.BeginGroup(tr.OutgoingGroup())
		ctx.UI.Emit(g.drawScreenUIForScreen(ctx, g.State.PreviousScreen)...)
		ctx.UI.EndGroup()
	}

	ctx.UI.BeginGroup(tr.IncomingGroup())
	ctx.UI.Emit(g.drawScreenUI(ctx)...)
	ctx.UI.EndGroup()

	cmds = append(cmds, drawStatusRail(g.State)...)
	return cmds
}

func (g *Manager) drawScreenUI(ctx *runtime.RenderContext) []render.Command {
	return g.drawScreenUIForScreen(ctx, g.State.Screen)
}

func (g *Manager) drawScreenUIForScreen(ctx *runtime.RenderContext, screen Screen) []render.Command {
	cmds := make([]render.Command, 0)
	switch screen {
	case ScreenDashboard:
		cmds = append(cmds, g.drawDashboard(ctx)...)
	case ScreenTable:
		cmds = append(cmds, g.drawLeagueTable(ctx)...)
	case ScreenSquad:
		cmds = append(cmds, g.drawSquad(ctx)...)
	case ScreenCalendar:
		cmds = append(cmds, g.drawCalendar(ctx)...)
	}
	return cmds
}

func stateScreenKey(screen Screen) uint64 {
	return uint64(screen) + 1
}

func screenStateToName(screen Screen) string {
	switch screen {
	case ScreenDashboard:
		return "DASHBOARD"
	case ScreenTable:
		return "LEAGUE"
	case ScreenCalendar:
		return "CALENDAR"
	case ScreenSquad:
		return "SQUAD"
	default:
		return ""
	}
}

func (g *Manager) PostStep(_ *runtime.UpdateContext, _ scene.StepStats) {}

func (g *Manager) drawNavAnimated(active Screen, ctx *runtime.RenderContext) []render.Command {
	cmds := make([]render.Command, 0, 20)
	cmds = append(cmds, rectCmd(3, rect(0, 0, viewW, 92), render.RGBA(0, 0, 0, 0.18))...)
	cmds = append(cmds, text("MATCHDAY", v(72, 46), textColor, 28, render.AlignLeft, render.BaselineMiddle, 12)...)
	for i, item := range navItems {
		col := bgRaised
		txt := textColor
		if Screen(i) == active {
			col = gold
			txt = bgDeep
		}

		target := 1.0
		if ctx.UI.In(ui.SpaceRef).Hovered(item.rect) {
			target = 1.08
		}

		// Pulse animation quando selecionado
		if g.UIState.SelectedNavItem == i {
			pulseTime := float64(ctx.Tick-g.UIState.NavPulseTick) / 60.0
			target = 1.0 + 0.15*math.Sin(pulseTime*6.28)
		}

		scaledRect := ctx.UI.SpringScale("nav_"+item.id, target, ui.SpringConfig{Stiffness: 200, Damping: 12}, item.rect)

		cmds = append(cmds, slantedPanel(scaledRect, col, render.RGBA(0, 0, 0, 0), 0, 12, 9)...)
		cmds = append(cmds, text(item.label, scaledRect.Center(), txt, 15, render.AlignCenter, render.BaselineMiddle, 10)...)
		if i < len(navItems)-1 {
			cmds = append(cmds, text("/", v(item.rect.Max.X+18, item.rect.Center().Y), muted, 22, render.AlignCenter, render.BaselineMiddle, 10)...)
		}
	}
	return cmds
}

func drawScreenTitle(title, club string) []render.Command {
	cmds := make([]render.Command, 0, 4)
	cmds = append(cmds,
		textRot(club, v(50, viewH/2), grass, 40, -math.Pi/2, render.AlignCenter, render.BaselineMiddle, 4),
		textRot(title, v(118, viewH/2), render.RGBA(1, 1, 1, 1), 120, -math.Pi/2, render.AlignCenter, render.BaselineMiddle, 4),
	)
	return cmds
}

func drawStatusRail(s ManagerState) []render.Command {
	cmds := make([]render.Command, 0, 30)
	items := []struct {
		label string
		value string
		col   render.Color
	}{
		{"SEASON", fmt.Sprintf("%d", s.SeasonYear), bgRaised},
		{"ROUND", fmt.Sprintf("%02d/%v", min(s.Round, len(s.Clubs)*2-2), len(s.Clubs)*2-2), gold},
		{"LEAGUE", "AURORA", bgRaised},
		{"MORALE", Morale(s.Clubs[0]), bgRaised},
		{"FUNDS", Cash(s.Clubs, s.Results, 0), bgRaised},
	}
	x := 190.0
	for _, item := range items {
		r := rect(x, 944, 300, 58)
		cmds = append(cmds, slantedPanel(r, item.col, line, 1, 12, 14)...)
		txt := textColor
		if item.col == gold {
			txt = bgDeep
		}
		cmds = append(cmds, text(item.label, v(r.Min.X+28, r.Min.Y+18), muted, 11, render.AlignLeft, render.BaselineMiddle, 15)...)
		cmds = append(cmds, text(item.value, v(r.Min.X+28, r.Min.Y+40), txt, 16, render.AlignLeft, render.BaselineMiddle, 15)...)
		x += 308
	}
	return cmds
}

func drawTacticalMotif() []render.Command {
	return []render.Command{
		poly(1, render.RGBA(0.18, 0.75, 0.44, 0.05), nil, v(1180, 92), v(1920, 92), v(1460, 1080), v(930, 1080)),
		poly(1, render.RGBA(0.94, 0.73, 0.23, 0.035), nil, v(0, 760), v(620, 310), v(780, 1080), v(0, 1080)),
		lineCmd(2, v(1260, 250), v(1660, 410), 3, render.RGBA(1, 1, 1, 0.05)),
		lineCmd(2, v(1660, 410), v(1470, 680), 3, render.RGBA(1, 1, 1, 0.05)),
		lineCmd(2, v(320, 870), v(620, 720), 3, render.RGBA(1, 1, 1, 0.05)),
		circle(2, v(1430, 285), 58, render.Color{}, render.RGBA(1, 1, 1, 0.05), 3),
		circle(2, v(1545, 575), 42, render.Color{}, render.RGBA(1, 1, 1, 0.05), 3),
	}
}

func titleTab(label string, x, y, w float64, col render.Color) []render.Command {
	r := rect(x, y, w, 42)
	cmds := slantedPanel(r, col, render.Color{}, 0, 12, 11)
	cmds = append(cmds, text(label, r.Center(), bgDeep, 14, render.AlignCenter, render.BaselineMiddle, 12)...)
	return cmds
}

func slantedPanel(r math2d.Rect, fill, stroke render.Color, strokeWidth, cut float64, layer int) []render.Command {
	points := []math2d.Vec2{
		v(r.Min.X+cut, r.Min.Y),
		v(r.Max.X, r.Min.Y),
		v(r.Max.X-cut, r.Max.Y),
		v(r.Min.X, r.Max.Y),
	}
	return []render.Command{poly(layer, fill, &strokeSpec{color: stroke, width: strokeWidth}, points...)}
}

func lastFiveLetters(s ManagerState, club int) []string {
	letters := []string{"-", "-", "-", "-", "-"}
	idx := len(letters) - 1
	for i := len(s.Results) - 1; i >= 0 && idx >= 0; i-- {
		result := s.Results[i]
		if result.Home != club && result.Away != club {
			continue
		}
		letters[idx] = resultLetter(result, club)
		idx--
	}
	return letters
}

// resultLetter reports the outcome of result from club's perspective: "W"
// (win), "D" (draw) or "L" (loss).
func resultLetter(result Result, club int) string {
	if result.HomeGoals == result.AwayGoals {
		return "D"
	}
	clubWon := (result.Home == club && result.HomeGoals > result.AwayGoals) ||
		(result.Away == club && result.AwayGoals > result.HomeGoals)
	if clubWon {
		return "W"
	}
	return "L"
}

func formScore(letters []string) float64 {
	score := 0.0
	played := 0.0
	for _, letter := range letters {
		switch letter {
		case "W":
			score += 1
			played++
		case "D":
			score += 0.45
			played++
		case "L":
			played++
		}
	}
	if played == 0 {
		return 0.45
	}
	return score / played
}

type navItem struct {
	id, label string
	rect      math2d.Rect
}

const (
	navItemY      = 26.0
	navItemH      = 40.0
	navItemGap    = 30.0
	navItemsRight = 1845.0 // right edge the nav row is anchored to
)

// navItems lays its entries out right-to-left from navItemsRight so adding,
// removing, or renaming a tab only requires editing its width here — no
// entry needs its x position hand-computed against its neighbors.
var navItems = buildNavItems([]struct {
	id, label string
	width     float64
}{
	{"nav_dashboard", "DASHBOARD", 150},
	{"nav_table", "TABLE", 120},
	{"nav_squad", "SQUAD", 120},
	{"nav_calendar", "CALENDAR", 150},
})

func buildNavItems(entries []struct {
	id, label string
	width     float64
}) []navItem {
	totalW := -navItemGap
	for _, e := range entries {
		totalW += e.width + navItemGap
	}

	items := make([]navItem, len(entries))
	x := navItemsRight - totalW
	for i, e := range entries {
		items[i] = navItem{id: e.id, label: e.label, rect: rect(x, navItemY, e.width, navItemH)}
		x += e.width + navItemGap
	}
	return items
}

func simButtonRect() math2d.Rect { return rect(720, 425, 238, 56) }

func drawSeasonEndedButton(idPrefix string, button math2d.Rect, ctx *runtime.RenderContext) []render.Command {
	btnTarget := 1.0
	if ctx.UI.In(ui.SpaceRef).Hovered(button) {
		btnTarget = 1.08
	}
	scaled := ctx.UI.SpringScale(idPrefix+"_btn_new_season", btnTarget, ui.SpringConfig{Stiffness: 200, Damping: 12}, button)
	cmds := slantedPanel(scaled, gold, render.RGBA(0, 0, 0, 0), 0, 16, 12)
	cmds = append(cmds, text("START NEW SEASON", scaled.Center(), bgDeep, 17, render.AlignCenter, render.BaselineMiddle, 13)...)
	return cmds
}

func seasonEnded(s ManagerState) bool {
	return len(s.Clubs) > 0 && s.Round > totalRounds(len(s.Clubs))
}

func rectCmd(layer int, r math2d.Rect, fill render.Color) []render.Command {
	return []render.Command{render.DrawRect{BaseCommand: render.BaseCommand{LayerValue: layer}, Rect: r, Fill: fill}}
}

func poly(layer int, fill render.Color, stroke *strokeSpec, points ...math2d.Vec2) render.Command {
	cmd := render.DrawPolygon{BaseCommand: render.BaseCommand{LayerValue: layer}, Points: points, Fill: fill}
	if stroke != nil {
		cmd.Stroke = stroke.color
		cmd.StrokeWidth = stroke.width
	}
	return cmd
}

func circle(layer int, center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64) render.Command {
	return render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: layer}, Center: center, Radius: radius, Fill: fill, Stroke: stroke, StrokeWidth: strokeWidth}
}

func lineCmd(layer int, from, to math2d.Vec2, width float64, color render.Color) render.Command {
	return render.DrawLine{BaseCommand: render.BaseCommand{LayerValue: layer}, From: from, To: to, Width: width, Color: color}
}

func text(value string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline, layer int) []render.Command {
	return []render.Command{textCommand(value, pos, color, size, 0, align, baseline, layer)}
}

func textRot(value string, pos math2d.Vec2, color render.Color, size, rotation float64, align render.TextAlign, baseline render.TextBaseline, layer int) render.Command {
	return textCommand(value, pos, color, size, rotation, align, baseline, layer)
}

func textCommand(value string, pos math2d.Vec2, color render.Color, size, rotation float64, align render.TextAlign, baseline render.TextBaseline, layer int) render.DrawText {
	return render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Position:    pos,
		Text:        value,
		Color:       color,
		Size:        size,
		Align:       align,
		Baseline:    baseline,
		Rotation:    rotation,
	}
}

func badge(label string, center math2d.Vec2, fill render.Color, layer int) []render.Command {
	w := 18*float64(len(label)) + 28
	r := rect(center.X-w/2, center.Y-18, w, 36)
	cmds := slantedPanel(r, fill, render.Color{}, 0, 8, layer)
	cmds = append(cmds, text(label, center, bgDeep, 13, render.AlignCenter, render.BaselineMiddle, layer+1)...)
	return cmds
}

func drawSegmentBar(r math2d.Rect, value float64, col render.Color, layer int) []render.Command {
	value = math.Max(0, math.Min(1, value))
	segments := 10
	gap := 6.0
	cmds := rectCmd(layer, r, render.RGBA(1, 1, 1, 0.06))
	w := (r.Width() - gap*float64(segments-1)) / float64(segments)
	active := int(math.Round(value * float64(segments)))
	for i := range segments {
		fill := render.RGBA(1, 1, 1, 0.10)
		if i < active {
			fill = col
		}
		cmds = append(cmds, rectCmd(layer+1, rect(r.Min.X+float64(i)*(w+gap), r.Min.Y, w, r.Height()), fill)...)
	}
	return cmds
}

// tableGeometry is satisfied by both ui.Interactor (Update) and ui.Drawer
// (Render) — it lets screen code ask the theme for row/header sizing instead
// of duplicating those constants locally, where they can silently drift out
// of sync with matchdayTheme.
type tableGeometry interface {
	RowHeight() float64
	HeaderHeight() float64
	TableBodyRect(rect math2d.Rect) math2d.Rect
}

func standingsBodyRect(g tableGeometry) math2d.Rect {
	return g.TableBodyRect(standingsTableRect())
}

func rect(x, y, w, h float64) math2d.Rect {
	return math2d.NewRect(v(x, y), v(w, h))
}

func v(x, y float64) math2d.Vec2 { return math2d.Vec2{X: x, Y: y} }
