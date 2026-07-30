package ui

import (
	"math"
	"strconv"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Tabs owns the active index the same way it owns any other widget state: it
// lives in ui.State keyed by widget id, so callers don't have to thread it
// through their own state, and it survives rewind with the rest of ui.State.
//
// Like every stateful widget the work is split in two: selection happens in
// Update (tabsInputIn, which reads the tick's pointer events), and Render
// (tabsIn) only reads the persisted index. Drawing never selects.

// TabsBodyRect returns the content area of a tab widget occupying rect (rect
// minus the header band), using the theme's bar height.
func (u *UI) TabsBodyRect(rect math2d.Rect) math2d.Rect {
	return tabsBodyRect(rect, u.theme.Tabs.BarHeight)
}

// tabsInputIn is called in Update. It returns the active tab index and whether
// this tick changed it.
func (u *UI) tabsInputIn(id string, rect math2d.Rect, count int, space Space) (int, bool) {
	wid := HashID(id)
	u.ensureTabs()

	active := clampTab(u.state.tabs[wid], count)
	changed := false
	barH := u.theme.Tabs.BarHeight
	for i := range count {
		if !u.clickedIn(tabHeaderID(id, i), tabHeaderRect(rect, i, count, barH), space) {
			continue
		}
		if i != active {
			active = i
			changed = true
		}
		break
	}

	u.state.tabs[wid] = active
	return active, changed
}

// tabsIn is called in Render. It draws the header band and returns the active
// tab index so the caller can draw that tab's content in TabsBodyRect.
func (u *UI) tabsIn(id string, rect math2d.Rect, labels []string, space Space) int {
	wid := HashID(id)
	u.ensureTabs()

	tabs := u.theme.Tabs
	active := clampTab(u.state.tabs[wid], len(labels))
	u.state.tabs[wid] = active
	bar := tabsBarRect(rect, tabs.BarHeight)

	layer := u.nextLayer()
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Rect:        u.rectToWorld(bar, space),
		Fill:        tabs.Bar,
		Stroke:      tabs.Border,
		StrokeWidth: u.sizeToWorld(tabs.BorderWidth, space),
	})

	cursor := u.cursorIn(space)
	for i, label := range labels {
		header := tabHeaderRect(rect, i, len(labels), tabs.BarHeight)
		fill := tabs.TabInactive
		switch {
		case i == active:
			fill = tabs.TabActive
		case u.inside && header.Contains(cursor):
			fill = tabs.TabHovered
		}
		u.commands = append(u.commands,
			render.DrawRect{
				BaseCommand:  render.BaseCommand{LayerValue: layer + 1},
				Rect:         u.rectToWorld(header, space),
				Fill:         fill,
				Stroke:       tabs.Border,
				StrokeWidth:  u.sizeToWorld(tabs.BorderWidth, space),
				CornerRadius: u.sizeToWorld(tabs.CornerRadius, space),
			},
			render.DrawText{
				BaseCommand: render.BaseCommand{LayerValue: layer + 2},
				Position:    u.toWorld(header.Center(), space),
				Text:        label,
				Color:       tabs.TextColor,
				Size:        u.sizeToWorld(tabs.TextSize, space),
				Font:        u.theme.Font,
				Align:       render.AlignCenter,
				Baseline:    render.BaselineMiddle,
			})
	}

	return active
}

// titledPanelIn draws a panel with a header band carrying title. It is a
// static container: no state, no interaction.
func (u *UI) titledPanelIn(rect math2d.Rect, title string, space Space) {
	header := u.theme.Header
	bar := math2d.NewRect(rect.Min, math2d.Vec2{X: rect.Width(), Y: math.Min(header.Height, rect.Height())})

	u.panelIn(rect, space)

	layer := u.nextLayer()
	u.commands = append(u.commands,
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer},
			Rect:         u.rectToWorld(bar, space),
			Fill:         header.Background,
			Stroke:       header.Border,
			StrokeWidth:  u.sizeToWorld(header.BorderWidth, space),
			CornerRadius: u.sizeToWorld(header.CornerRadius, space),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    u.toWorld(bar.Center(), space),
			Text:        title,
			Color:       header.TextColor,
			Size:        u.sizeToWorld(u.theme.Label.Size, space),
			Font:        u.theme.Font,
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		})
}

func (u *UI) ensureTabs() {
	if u.state.tabs == nil {
		u.state.tabs = make(map[ID]int)
	}
}

// tabHeaderID derives the clickable id of one header from the widget id, so
// two tab widgets on the same screen never share a hit target.
func tabHeaderID(id string, i int) string {
	return id + "#tab" + strconv.Itoa(i)
}

func tabsBarRect(rect math2d.Rect, barH float64) math2d.Rect {
	return math2d.NewRect(rect.Min, math2d.Vec2{
		X: rect.Width(),
		Y: math.Min(barH, rect.Height()),
	})
}

func tabsBodyRect(rect math2d.Rect, barH float64) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: rect.Min.X, Y: rect.Min.Y + barH},
		math2d.Vec2{X: rect.Width(), Y: math.Max(0, rect.Height()-barH)},
	)
}

// tabHeaderRect returns the i-th header of count equal-width headers. Widths
// are equal in this version; per-content widths are a follow-up.
func tabHeaderRect(rect math2d.Rect, i, count int, barH float64) math2d.Rect {
	if count <= 0 {
		return math2d.NewRect(rect.Min, math2d.Vec2{})
	}
	bar := tabsBarRect(rect, barH)
	width := bar.Width() / float64(count)
	return math2d.NewRect(
		math2d.Vec2{X: bar.Min.X + float64(i)*width, Y: bar.Min.Y},
		math2d.Vec2{X: width, Y: bar.Height()},
	)
}

func clampTab(active, count int) int {
	if count <= 0 || active < 0 {
		return 0
	}
	return min(active, count-1)
}
