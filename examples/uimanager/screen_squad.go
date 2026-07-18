package uimanager

import (
	"fmt"
	"math"
	"slices"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
)

const (
	squadRowH    = 56.0
	squadHeaderH = 28.0
)

// squad list panel + body rects
func squadListPanel() math2d.Rect   { return rect(190, 170, 740, 690) }
func squadDetailPanel() math2d.Rect { return rect(970, 170, 770, 690) }
func squadBodyRect() math2d.Rect {
	p := squadListPanel()
	return rect(p.Min.X+20, p.Min.Y+52, p.Width()-40, p.Height()-72)
}

// squadListRect is the scrollable row area of the squad body, i.e.
// squadBodyRect() minus the column header band drawn above it.
func squadListRect() math2d.Rect {
	body := squadBodyRect()
	return rect(body.Min.X, body.Min.Y+squadHeaderH, body.Width(), body.Height()-squadHeaderH)
}

func (g *Manager) updateSquad(ctx *runtime.UpdateContext) {
	squad := g.State.Clubs[0].Squad
	// ScrollOffset consumes the tick's wheel delta — the render-side List
	// call also touches scroll state, but Render's UI carries no PointerFrame
	// (no wheel data), so wheel input must be applied here in Update.
	in := ctx.UI.In(ui.SpaceRef)
	in.ScrollOffset("lst_squad", squadListRect(), squadRowH*float64(len(squad)))
	if row := in.ListHit("lst_squad", squadListRect(), squadRowH, len(squad)); row >= 0 {
		if g.State.SelectedPlayer == row {
			g.State.SelectedPlayer = -1
		} else {
			g.State.SelectedPlayer = row
		}
	}
}

func (g *Manager) drawSquad(ctx *runtime.RenderContext) []render.Command {
	s := g.State
	cmds := make([]render.Command, 0, 200)
	cmds = append(cmds, drawSquadListBody(s, ctx)...)
	cmds = append(cmds, drawSquadDetail(s)...)
	return cmds
}

// ─── squad list (left panel) ────────────────────────────────────────────────

func drawSquadListBody(s ManagerState, ctx *runtime.RenderContext) []render.Command {
	squad := s.Clubs[0].Squad
	panel := squadListPanel()
	body := squadBodyRect()

	cmds := slantedPanel(panel, bgPanel, line, 1, 20, 8)
	cmds = append(cmds, titleTab(fmt.Sprintf("SQUAD (%d)", len(squad)), panel.Min.X+24, panel.Min.Y-16, 170, grass)...)

	headerY := body.Min.Y
	cmds = append(cmds, rectCmd(12, rect(body.Min.X, headerY, body.Width(), squadHeaderH), render.RGBA(1, 1, 1, 0.04))...)
	cmds = append(cmds, text("POS", v(body.Min.X+26, headerY+14), muted, 10, render.AlignLeft, render.BaselineMiddle, 13)...)
	cmds = append(cmds, text("NAME", v(body.Min.X+82, headerY+14), muted, 10, render.AlignLeft, render.BaselineMiddle, 13)...)
	cmds = append(cmds, text("AGE", v(body.Max.X-220, headerY+14), muted, 10, render.AlignRight, render.BaselineMiddle, 13)...)
	cmds = append(cmds, text("OVR", v(body.Max.X-140, headerY+14), muted, 10, render.AlignRight, render.BaselineMiddle, 13)...)
	cmds = append(cmds, text("FORM", v(body.Max.X-20, headerY+14), muted, 10, render.AlignRight, render.BaselineMiddle, 13)...)

	// Rows scroll/cull/zebra/selected through DrawScope.List — it emits directly
	// to the UI's command stream (correctly captured by the screen's
	// transition group), so only the panel chrome above is returned here.
	ctx.UI.In(ui.SpaceRef).List("lst_squad", squadListRect(), squadRowH, len(squad), s.SelectedPlayer, func(i int, rowRect math2d.Rect) {
		p := squad[i]
		ctx.UI.Emit(lineCmd(11, v(rowRect.Min.X, rowRect.Max.Y), v(rowRect.Max.X, rowRect.Max.Y), 1, line))

		badgeR := rect(rowRect.Min.X+18, rowRect.Min.Y+14, 48, 28)
		posCol := posColor(p.Pos)
		ctx.UI.Emit(slantedPanel(badgeR, posCol, render.Color{}, 0, 6, 13)...)
		ctx.UI.Emit(text(posAbbr(p.Pos), badgeR.Center(), bgDeep, 11, render.AlignCenter, render.BaselineMiddle, 14)...)

		nameX := rowRect.Min.X + 82
		ctx.UI.Emit(text(p.Name, v(nameX, rowRect.Center().Y), textColor, 15, render.AlignLeft, render.BaselineMiddle, 13)...)

		ageX := rowRect.Max.X - 220
		ctx.UI.Emit(text(fmt.Sprintf("%d", p.Age), v(ageX, rowRect.Center().Y), muted, 13, render.AlignRight, render.BaselineMiddle, 13)...)

		ovr := overallRating(p)
		ovrX := rowRect.Max.X - 140
		ctx.UI.Emit(text(fmt.Sprintf("%d", ovr), v(ovrX, rowRect.Center().Y), overallColor(ovr), 20, render.AlignRight, render.BaselineMiddle, 13)...)

		barW := 90.0
		barR := rect(rowRect.Max.X-barW-16, rowRect.Min.Y+20, barW, 8)
		ctx.UI.Emit(drawSegmentBar(barR, p.Form, formBarColor(p.Form), 13)...)
	})

	return cmds
}

// ─── squad detail (right panel) ─────────────────────────────────────────────

func drawSquadDetail(s ManagerState) []render.Command {
	cmds := make([]render.Command, 0, 80)
	panel := squadDetailPanel()
	cmds = append(cmds, slantedPanel(panel, bgPanel, line, 1, 20, 8)...)
	cmds = append(cmds, titleTab("DETAILS", panel.Min.X+24, panel.Min.Y-16, 170, gold)...)

	if s.SelectedPlayer < 0 || s.SelectedPlayer >= len(s.Clubs[0].Squad) {
		cmds = append(cmds, drawEmptyDetail(panel)...)
		return cmds
	}

	p := s.Clubs[0].Squad[s.SelectedPlayer]
	cmds = append(cmds, drawPlayerDetail(panel, p)...)
	return cmds
}

func drawEmptyDetail(panel math2d.Rect) []render.Command {
	return text("SELECT A PLAYER", panel.Center(), muted, 24, render.AlignCenter, render.BaselineMiddle, 12)
}

func drawPlayerDetail(panel math2d.Rect, p Player) []render.Command {
	cmds := make([]render.Command, 0, 60)
	cx := panel.Center().X
	topY := panel.Min.Y + 80

	cmds = append(cmds, text(p.Name, v(cx, topY), textColor, 38, render.AlignCenter, render.BaselineMiddle, 12)...)

	badgeR := rect(cx-140, topY+50, 64, 32)
	posCol := posColor(p.Pos)
	cmds = append(cmds, slantedPanel(badgeR, posCol, render.Color{}, 0, 8, 12)...)
	cmds = append(cmds, text(posAbbr(p.Pos), badgeR.Center(), bgDeep, 14, render.AlignCenter, render.BaselineMiddle, 13)...)
	ageStr := fmt.Sprintf("%d YEARS", p.Age)
	cmds = append(cmds, text(ageStr, v(badgeR.Max.X+28, badgeR.Center().Y), muted, 14, render.AlignLeft, render.BaselineMiddle, 12)...)

	formY := topY + 120
	upArrow, formLabel := formDisplay(p.Form)
	if upArrow {
		cmds = append(cmds, drawArrowUp(v(cx-90, formY), 7, gold))
		cmds = append(cmds, text("FORM", v(cx-64, formY), muted, 14, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(formLabel, v(cx+40, formY), grass, 20, render.AlignLeft, render.BaselineMiddle, 12)...)
	} else {
		cmds = append(cmds, drawArrowDown(v(cx-90, formY), 7, loss))
		cmds = append(cmds, text("FORM", v(cx-64, formY), muted, 14, render.AlignLeft, render.BaselineMiddle, 12)...)
		cmds = append(cmds, text(formLabel, v(cx+40, formY), loss, 20, render.AlignLeft, render.BaselineMiddle, 12)...)
	}

	barStartY := topY + 200
	barH := 36.0

	// Layout using full panel width:
	// [label ~50px] [bar: fills available space to number] [number ~50px]
	leftEdge := panel.Min.X + 50
	rightEdge := panel.Max.X - 50

	bars := []struct {
		label string
		value int
		col   render.Color
	}{
		{"ATT", p.Attack, loss},
		{"DEF", p.Defense, dfBlue},
		{"RES", p.Stamina, grass},
	}

	for i, b := range bars {
		y := barStartY + float64(i)*80

		// Label: left-aligned near panel left edge
		cmds = append(cmds, text(b.label, v(leftEdge, y), muted, 16, render.AlignLeft, render.BaselineMiddle, 12)...)

		// Number: right-aligned near panel right edge
		numCol := overallColor(b.value)
		cmds = append(cmds, text(fmt.Sprintf("%d", b.value), v(rightEdge, y), numCol, 36, render.AlignRight, render.BaselineMiddle, 13)...)

		// Segmented bar: fills between label and number
		barRect := rect(leftEdge+64, y-16, rightEdge-leftEdge-148, barH)
		cmds = append(cmds, drawSegmentBar(barRect, float64(b.value)/100.0, b.col, 13)...)

		// Bar marks: 0, 50, 100
		marks := []struct {
			x     float64
			label string
		}{
			{barRect.Min.X, "0"},
			{barRect.Center().X, "50"},
			{barRect.Max.X, "100"},
		}
		for _, m := range marks {
			cmds = append(cmds, text(m.label, v(m.x, barRect.Max.Y+12), muted, 9, render.AlignCenter, render.BaselineMiddle, 13)...)
		}
	}

	return cmds
}

// ─── arrow helpers ──────────────────────────────────────────────────────────

func drawArrowUp(center math2d.Vec2, size float64, col render.Color) render.Command {
	return poly(13, col, nil,
		v(center.X, center.Y-size),
		v(center.X-size, center.Y+size),
		v(center.X+size, center.Y+size),
	)
}

func drawArrowDown(center math2d.Vec2, size float64, col render.Color) render.Command {
	return poly(13, col, nil,
		v(center.X, center.Y+size),
		v(center.X-size, center.Y-size),
		v(center.X+size, center.Y-size),
	)
}

// ─── position helpers ───────────────────────────────────────────────────────

func posGroup(p Position) int {
	switch {
	case p == GK:
		return 0
	case p == CB || p == LB || p == RB:
		return 1
	case p >= DMF && p <= RMF:
		return 2
	default:
		return 3
	}
}

func posAbbr(p Position) string {
	switch {
	case p == GK:
		return "GK"
	case p == CB || p == LB || p == RB:
		return "DF"
	case p >= DMF && p <= RMF:
		return "MF"
	default:
		return "AT"
	}
}

func posColor(p Position) render.Color {
	switch {
	case p == GK:
		return gold
	case p == CB || p == LB || p == RB:
		return dfBlue
	case p >= DMF && p <= RMF:
		return grass
	default:
		return loss
	}
}

// ─── rating helpers ─────────────────────────────────────────────────────────

func overallRating(p Player) int {
	switch {
	case p.Pos == GK:
		return int(math.Round(0.50*float64(p.Defense) + 0.35*float64(p.Stamina) + 0.15*float64(p.Attack)))
	case posGroup(p.Pos) == 1: // DF
		return int(math.Round(0.50*float64(p.Defense) + 0.30*float64(p.Stamina) + 0.20*float64(p.Attack)))
	case posGroup(p.Pos) == 2: // MF
		return int(math.Round(0.35*float64(p.Attack) + 0.35*float64(p.Stamina) + 0.30*float64(p.Defense)))
	default: // AT
		return int(math.Round(0.50*float64(p.Attack) + 0.30*float64(p.Stamina) + 0.20*float64(p.Defense)))
	}
}

func overallColor(ovr int) render.Color {
	if ovr >= 70 {
		return grass
	}
	return gold
}

func formBarColor(f float64) render.Color {
	switch {
	case f >= 0.60:
		return grass
	case f >= 0.35:
		return gold
	default:
		return loss
	}
}

func formDisplay(f float64) (up bool, label string) {
	switch {
	case f >= 0.75:
		return true, "EXCELLENT"
	case f >= 0.55:
		return true, "GOOD"
	case f >= 0.35:
		return false, "REGULAR"
	default:
		return false, "LOW"
	}
}

// ─── dfBlue constant ────────────────────────────────────────────────────────

func initSquadOrder(squad []Player) {
	slices.SortStableFunc(squad, func(a, b Player) int {
		return posGroup(a.Pos) - posGroup(b.Pos)
	})
}
