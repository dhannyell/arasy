package ui

import (
	"math"
	"strconv"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Tabs comes in two ownership modes, and which one to reach for follows the
// same line the rest of this package draws.
//
// Caller-owned (tabsInputValueIn / tabsAtIn) is the mode for state the game
// means something by: the caller passes the active index in and gets the next
// one back, exactly like sliderInputIn does with its value and tableIn with
// its selected row. Screen navigation belongs here — it lives in the game's
// own state, so it rewinds with the simulation, shows up in a replay
// fingerprint, and can be written by paths the UI never sees (a keyboard
// shortcut cycling screens) without anything to keep in sync.
//
// Widget-owned (tabsInputIn / tabsIn) is a thin shell over that core which
// parks the index in ui.State keyed by widget id. It is for state nobody
// would think to declare or to save — a tooling overlay's open tab. It gets
// cloned with the rest of ui.State, so it survives rewind too.
//
// Both modes split Update from Render the same way: selection is processed in
// Update from the tick's pointer events, and drawing only reads. Drawing
// never selects.

// TabsBodyRect returns the content area of a tab widget occupying rect (rect
// minus the header band), using the theme's bar height.
func (u *UI) TabsBodyRect(rect math2d.Rect) math2d.Rect {
	return tabsBodyRect(rect, u.theme.Tabs.BarHeight)
}

// tabsInputValueIn is called in Update. It is the caller-owned core: current
// is the index the caller holds, and the returned index is the one it should
// hold next. Taking the headers as rects rather than a bar rect and a count
// leaves every layout question — widths, gaps, which edge the row is anchored
// to — with the caller, who is the only one who can answer them.
func (u *UI) tabsInputValueIn(id string, headers []math2d.Rect, current int, space Space) (int, bool) {
	active := clampTab(current, len(headers))
	for i, header := range headers {
		if !u.clickedIn(tabHeaderID(id, i), header, space) {
			continue
		}
		if i != active {
			return i, true
		}
		break
	}
	// An out-of-range index the caller handed us was clamped above, which is
	// a change it needs to hear about — same contract as sliderInputIn.
	return active, active != current
}

// tabsInputIn is the widget-owned shell: it reads the active tab out of
// ui.State, runs the caller-owned core over an equal-width header row, and
// stores the result back.
func (u *UI) tabsInputIn(id string, rect math2d.Rect, count int, space Space) (int, bool) {
	wid := HashID(id)
	u.ensureTabs()

	headers := tabHeaderRects(rect, count, u.theme.Tabs.BarHeight)
	active, changed := u.tabsInputValueIn(id, headers, u.state.tabs[wid], space)
	u.state.tabs[wid] = active
	return active, changed
}

// tabsIn is the widget-owned shell for Render: it draws the band behind an
// equal-width header row and returns the active tab index so the caller can
// draw that tab's content in TabsBodyRect.
func (u *UI) tabsIn(id string, rect math2d.Rect, labels []string, space Space) int {
	wid := HashID(id)
	u.ensureTabs()

	tabs := u.theme.Tabs
	active := clampTab(u.state.tabs[wid], len(labels))
	u.state.tabs[wid] = active

	layer := u.nextLayer()
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Rect:        u.rectToWorld(tabsBarRect(rect, tabs.BarHeight), space),
		Fill:        tabs.Bar,
		Stroke:      tabs.Border,
		StrokeWidth: u.sizeToWorld(tabs.BorderWidth, space),
	})

	u.tabsAtIn(tabHeaderRects(rect, len(labels), tabs.BarHeight), labels, active, space)
	return active
}

// tabsAtIn draws one header per rect in the theme's tab styling. It is the
// caller-owned draw: no state is read or written, and the band behind the
// headers belongs to the caller, who chose the layout and may not have laid
// the headers out as one contiguous row.
func (u *UI) tabsAtIn(headers []math2d.Rect, labels []string, active int, space Space) {
	tabs := u.theme.Tabs
	layer := u.nextLayer()

	cursor := u.cursorIn(space)
	for i, header := range headers {
		if i >= len(labels) {
			break
		}
		label := labels[i]
		fill := tabs.TabInactive
		switch {
		case i == active:
			fill = tabs.TabActive
		case u.inside && header.Contains(cursor):
			fill = tabs.TabHovered
		}
		u.commands = append(u.commands,
			render.DrawRect{
				BaseCommand:  render.BaseCommand{LayerValue: layer},
				Rect:         u.rectToWorld(header, space),
				Fill:         fill,
				Stroke:       tabs.Border,
				StrokeWidth:  u.sizeToWorld(tabs.BorderWidth, space),
				CornerRadius: u.sizeToWorld(tabs.CornerRadius, space),
			},
			render.DrawText{
				BaseCommand: render.BaseCommand{LayerValue: layer + 1},
				Position:    u.toWorld(header.Center(), space),
				Text:        label,
				Color:       tabs.TextColor,
				Size:        u.sizeToWorld(tabs.TextSize, space),
				Font:        u.theme.Font,
				Align:       render.AlignCenter,
				Baseline:    render.BaselineMiddle,
			})
	}
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

// tabHeaderRects lays count equal-width headers across the band. It is the
// default layout of the widget-owned mode; callers that own their index pass
// their own header rects instead.
func tabHeaderRects(rect math2d.Rect, count int, barH float64) []math2d.Rect {
	headers := make([]math2d.Rect, max(count, 0))
	for i := range headers {
		headers[i] = tabHeaderRect(rect, i, count, barH)
	}
	return headers
}

// tabHeaderRect returns the i-th header of count equal-width headers.
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
