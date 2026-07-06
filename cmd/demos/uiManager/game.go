package uiManager

import (
	"fmt"
	"math"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

const (
	viewW = 1920
	viewH = 1080
)

type Manager struct {
	runtime.StateSlot[ManagerState]
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
		InputBindings: uiManagerBindings(),
		FPS:           60,
		HistoryLen:    600,
		UITheme:       matchdayTheme(),
	}
}

func (g *Manager) Init(ctx *runtime.UpdateContext) {
	_ = ctx
	clubs, fixtures := GenerateLeague(20260706)
	g.State = ManagerState{
		Screen:         ScreenDashboard,
		Seed:           20260706,
		Round:          1,
		Clubs:          clubs,
		Fixtures:       fixtures,
		SelectedPlayer: -1,
	}
}

func (g *Manager) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
	for i, item := range navItems {
		if ctx.UI.ClickedIn(item.id, item.rect, ui.SpaceRef) {
			g.State.PreviousScreen = g.State.Screen
			g.State.Screen = Screen(i)
		}
	}
	if ctx.UI.ClickedIn("btn_sim_dashboard", simButtonRect(), ui.SpaceRef) {
		SimulateRound(&g.State)
	}
	if g.State.Screen == ScreenTable {
		ctx.UI.ScrollOffsetIn("tbl_standings", standingsBodyRect(), float64(len(g.State.Clubs))*standingsRowH, ui.SpaceRef)
	}

	g.checkPressedButtons(ctx)
}

func (g *Manager) checkPressedButtons(ctx *runtime.UpdateContext) {
	if ctx.Input.JustPressed(input.ActionMoveX) && ctx.Input.Value(input.ActionMoveX) < 0 {
		g.State.PreviousScreen = g.State.Screen
		g.State.Screen = Screen(math2d.Clamp(float64(g.State.Screen-1), 0, 3))
	}
	if ctx.Input.JustPressed(input.ActionMoveX) && ctx.Input.Value(input.ActionMoveX) > 0 {
		g.State.PreviousScreen = g.State.Screen

		g.State.Screen = Screen(math2d.Clamp(float64(g.State.Screen+1), 0, 3))
	}
}

func (g *Manager) Render(ctx *runtime.RenderContext) []render.Command {
	cmds := make([]render.Command, 0)
	cmds = append(cmds, rectCmd(0, rect(0, 0, viewW, viewH), bgDeep)...)
	cmds = append(cmds, drawTacticalMotif()...)
	cmds = append(cmds, drawNav(g.State.Screen)...)
	cmds = append(cmds, drawScreenTitle(screenStateToName(g.State.Screen), g.State.Clubs[0].Name)...)

	transitionKind := ui.TransitionSlideLeft
	if g.State.Screen < g.State.PreviousScreen {
		transitionKind = ui.TransitionSlideRight
	}

	tr := ctx.UI.ScreenTransition("manager_screen", stateScreenKey(g.State.Screen), ui.TransitionOptions{
		Kind:     transitionKind,
		Duration: 0.1,
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
		cmds = append(cmds, g.drawDashboard()...)
	case ScreenTable:
		cmds = append(cmds, g.drawLeagueTable(ctx)...)
	default:
		cmds = append(cmds, drawPlaceholder(screen)...)
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

func drawPlaceholder(screen Screen) []render.Command {
	names := map[Screen]string{ScreenTable: "TABELA", ScreenSquad: "ELENCO", ScreenCalendar: "CALENDARIO"}
	panel := rect(360, 280, 1200, 470)
	cmds := slantedPanel(panel, bgPanel, line, 1, 24, 8)
	cmds = append(cmds, text(names[screen], panel.Center().Add(v(0, -22)), gold, 44, render.AlignCenter, render.BaselineMiddle, 12)...)
	cmds = append(cmds, text("placeholder visual - plano 021 continua depois", panel.Center().Add(v(0, 42)), muted, 18, render.AlignCenter, render.BaselineMiddle, 12)...)
	return cmds
}

func drawNav(active Screen) []render.Command {
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
		cmds = append(cmds, slantedPanel(item.rect, col, render.RGBA(0, 0, 0, 0), 0, 12, 9)...)
		cmds = append(cmds, text(item.label, item.rect.Center(), txt, 15, render.AlignCenter, render.BaselineMiddle, 10)...)
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
		{"SEASON", "2026", bgRaised},
		{"ROUND", fmt.Sprintf("%02d/%v", minInt(s.Round, len(s.Clubs)*2-2), len(s.Clubs)*2-2), gold},
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

func drawSegmentBar(r math2d.Rect, value float64, col render.Color, layer int) []render.Command {
	value = math.Max(0, math.Min(1, value))
	cmds := rectCmd(layer, r, render.RGBA(1, 1, 1, 0.08))
	segments := 10
	gap := 5.0
	w := (r.Width() - gap*float64(segments-1)) / float64(segments)
	active := int(math.Round(value * float64(segments)))
	for i := range segments {
		fill := render.RGBA(1, 1, 1, 0.12)
		if i < active {
			fill = col
		}
		cmds = append(cmds, rectCmd(layer+1, rect(r.Min.X+float64(i)*(w+gap), r.Min.Y, w, r.Height()), fill)...)
	}
	return cmds
}

func lastFiveLetters(s ManagerState, club int) []string {
	letters := []string{"-", "-", "-", "-", "-"}
	idx := len(letters) - 1
	for i := len(s.Results) - 1; i >= 0 && idx >= 0; i-- {
		result := s.Results[i]
		if result.Home != club && result.Away != club {
			continue
		}
		letter := "D"
		if result.HomeGoals != result.AwayGoals {
			clubWon := (result.Home == club && result.HomeGoals > result.AwayGoals) ||
				(result.Away == club && result.AwayGoals > result.HomeGoals)
			if clubWon {
				letter = "W"
			} else {
				letter = "L"
			}
		}
		letters[idx] = letter
		idx--
	}
	return letters
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

var navItems = []navItem{
	{"nav_dashboard", "DASHBOARD", rect(1215, 26, 150, 40)},
	{"nav_table", "TABLE", rect(1395, 26, 120, 40)},
	{"nav_squad", "SQUAD", rect(1545, 26, 120, 40)},
	{"nav_calendar", "FIXTURES", rect(1695, 26, 150, 40)},
}

func simButtonRect() math2d.Rect { return rect(720, 425, 238, 56) }

func matchdayTheme() ui.Theme {
	t := ui.DefaultTheme()
	t.Button.Normal.Background = gold
	t.Button.Normal.TextColor = bgDeep
	t.Text.Color = textColor
	t.Text.Muted = muted
	t.Header.Background = bgPanel
	t.Header.TextColor = textColor
	t.Header.Border = line
	t.Row.TextColor = textColor
	t.Row.Normal = render.Color{}
	t.Row.Zebra = zebra
	t.Row.Hovered = render.RGBA(1, 1, 1, 0.06)
	t.Row.Selected = render.RGBA(0.941, 0.725, 0.231, 0.16)
	return t
}

type strokeSpec struct {
	color render.Color
	width float64
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

// standingsRowH and standingsHeaderH must match the theme Row.Height and Header.Height
// (both left at defaults by matchdayTheme).
const (
	standingsRowH    = 32.0
	standingsHeaderH = 36.0
)

func standingsBodyRect() math2d.Rect {
	panel := rect(190, 170, 820, 690)
	tableRect := rect(panel.Min.X+20, panel.Min.Y+52, panel.Width()-40, panel.Height()-72)
	return rect(tableRect.Min.X, tableRect.Min.Y+standingsHeaderH, tableRect.Width(), tableRect.Height()-standingsHeaderH)
}

func rect(x, y, w, h float64) math2d.Rect {
	return math2d.NewRect(v(x, y), v(w, h))
}

func v(x, y float64) math2d.Vec2 { return math2d.Vec2{X: x, Y: y} }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var (
	bgDeep    = render.RGBA(0.039, 0.071, 0.110, 1)
	bgPanel   = render.RGBA(0.078, 0.129, 0.184, 0.96)
	bgRaised  = render.RGBA(0.106, 0.173, 0.243, 1)
	grass     = render.RGBA(0.184, 0.749, 0.443, 1)
	gold      = render.RGBA(0.941, 0.725, 0.231, 1)
	loss      = render.RGBA(0.898, 0.282, 0.302, 1)
	drawColor = render.RGBA(0.541, 0.608, 0.667, 1)
	textColor = render.RGBA(0.949, 0.961, 0.969, 1)
	muted     = render.RGBA(0.494, 0.561, 0.627, 1)
	line      = render.RGBA(1, 1, 1, 0.08)
	zebra     = render.RGBA(1, 1, 1, 0.035)
)

func uiManagerBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyEscape), Action: input.ActionPause, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: -1},
	}
}
