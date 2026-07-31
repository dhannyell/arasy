package ui

import (
	"math"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Dropdown splits ownership the way the rest of this package does: the
// selected index is the caller's — it is a filter or a setting the game means
// something by — while whether the menu happens to be open right now is the
// widget's, parked in ui.State where nobody has to declare it.
//
// The open list is a popup: it draws in the reserved overlay layer block so it
// covers widgets declared after it, and it claims those pixels in ui.State so
// the widgets underneath do not answer clicks meant for the list. The list
// itself is the ordinary virtualized List, so a thousand options cost what
// eight do.

// dropdownInputIn is called in Update. It toggles the menu, picks an item and
// closes on a click anywhere else, returning the index the caller should hold.
func (u *UI) dropdownInputIn(id string, rect math2d.Rect, count, selected int, space Space) (int, bool) {
	wid := HashID(id)
	u.ensureOpen()
	u.focusableIn(id, rect, space)

	open := u.state.open[wid]
	if u.activatedIn(id, rect, space) {
		u.setDropdownOpen(id, rect, count, !open, space)
		return selected, false
	}

	if !open {
		return selected, false
	}

	popup := u.dropdownPopupRect(rect, count)
	u.scrollOffsetIn(dropdownPopupID(id), popup, float64(count)*u.theme.Row.Height, space)
	if item := u.listHitIn(dropdownPopupID(id), popup, u.theme.Row.Height, count, space); item >= 0 {
		u.setDropdownOpen(id, rect, count, false, space)
		return item, item != selected
	}

	// Any press that reached this far landed outside both the button and the
	// list: the menu closes without changing the selection.
	if u.pressedOutsidePopup(rect, space) {
		u.setDropdownOpen(id, rect, count, false, space)
	}
	return selected, false
}

// setDropdownOpen records the open state together with the screen rectangle
// the popup claims, which is what blockedAt consults for every other widget.
// The claim is filed under the list's id, not the button's, so the list itself
// still answers the clicks that land on it.
func (u *UI) setDropdownOpen(id string, rect math2d.Rect, count int, open bool, space Space) {
	listID := HashID(dropdownPopupID(id))
	u.state.open[HashID(id)] = open
	if !open {
		if u.state.popupOwner == listID {
			u.state.popupOwner = noID
			u.state.popupRect = math2d.Rect{}
		}
		return
	}

	popup := u.dropdownPopupRect(rect, count)
	cam := u.cameraFor(space)
	u.state.popupOwner = listID
	u.state.popupRect = math2d.NewRectFromCorners(
		cam.WorldToScreen(popup.Min),
		cam.WorldToScreen(popup.Max),
	)
}

// pressedOutsidePopup reports a primary press this tick that missed both the
// button and the open list. It reads raw pointer events rather than clickedIn
// because closing is about where the press landed, not about completing a
// click on anything.
func (u *UI) pressedOutsidePopup(button math2d.Rect, space Space) bool {
	for _, e := range u.frame.Events {
		if e.Kind != input.EventButtonDown || e.Button != input.PointerPrimary {
			continue
		}
		if u.state.popupRect.Contains(e.Pos) {
			continue
		}
		if button.Contains(u.cameraFor(space).ScreenToWorld(e.Pos)) {
			continue
		}
		return true
	}
	return false
}

// dropdownDrawIn draws the closed button, and the open list above everything.
func (u *UI) dropdownDrawIn(id string, rect math2d.Rect, labels []string, selected int, space Space) {
	theme := u.theme.Dropdown
	if rect.Width() <= 0 || rect.Height() <= 0 {
		return
	}

	wid := HashID(id)
	u.ensureOpen()
	open := u.state.open[wid]

	fill := theme.Background
	switch {
	case open:
		fill = theme.BackgroundOpen
	case u.hoveredIn(rect, space):
		fill = theme.BackgroundHovered
	}

	layer := u.nextLayer()
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(rect, space),
		Fill:         fill,
		Stroke:       theme.Border,
		StrokeWidth:  u.sizeToWorld(theme.BorderWidth, space),
		CornerRadius: u.sizeToWorld(theme.CornerRadius, space),
	})

	label := ""
	if selected >= 0 && selected < len(labels) {
		label = labels[selected]
	}
	u.commands = append(u.commands, render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: layer + 1},
		Position:    u.toWorld(math2d.Vec2{X: rect.Min.X + theme.PaddingX, Y: rect.Center().Y}, space),
		Text:        label,
		Color:       theme.TextColor,
		Size:        u.sizeToWorld(theme.TextSize, space),
		Font:        u.theme.Font,
		Align:       render.AlignLeft,
		Baseline:    render.BaselineMiddle,
	})
	u.dropdownArrow(rect, open, layer+1, space)

	if u.state.focusID == wid {
		u.focusRingIn(rect, layer+2, space)
	}
	if !open {
		return
	}

	popup := u.dropdownPopupRect(rect, len(labels))
	u.beginOverlay()
	defer u.endOverlay()

	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: u.nextLayer()},
		Rect:         u.rectToWorld(popup, space),
		Fill:         theme.PopupBackground,
		Stroke:       theme.Border,
		StrokeWidth:  u.sizeToWorld(theme.BorderWidth, space),
		CornerRadius: u.sizeToWorld(theme.CornerRadius, space),
	})
	u.listIn(dropdownPopupID(id), popup, u.theme.Row.Height, len(labels), selected, func(row int, rowRect math2d.Rect) {
		u.commands = append(u.commands, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
			Position:    u.toWorld(math2d.Vec2{X: rowRect.Min.X + theme.PaddingX, Y: rowRect.Center().Y}, space),
			Text:        labels[row],
			Color:       theme.TextColor,
			Size:        u.sizeToWorld(theme.TextSize, space),
			Font:        u.theme.Font,
			Align:       render.AlignLeft,
			Baseline:    render.BaselineMiddle,
		})
	}, space)
}

func (u *UI) dropdownArrow(rect math2d.Rect, open bool, layer int, space Space) {
	theme := u.theme.Dropdown
	size := math.Min(theme.ArrowSize, rect.Height()/2)
	center := math2d.Vec2{X: rect.Max.X - theme.PaddingX - size/2, Y: rect.Center().Y}
	half := size / 2
	top, bottom := center.Y-half, center.Y+half
	if open {
		top, bottom = bottom, top
	}
	u.commands = append(u.commands, render.DrawPolygon{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Points: []math2d.Vec2{
			u.toWorld(math2d.Vec2{X: center.X - half, Y: top}, space),
			u.toWorld(math2d.Vec2{X: center.X + half, Y: top}, space),
			u.toWorld(math2d.Vec2{X: center.X, Y: bottom}, space),
		},
		Fill: theme.TextColor,
	})
}

// dropdownPopupRect hangs the list under the button, as tall as the options
// need up to the theme's cap.
func (u *UI) dropdownPopupRect(rect math2d.Rect, count int) math2d.Rect {
	rows := max(min(count, u.theme.Dropdown.MaxVisibleRows), 1)
	return math2d.NewRect(
		math2d.Vec2{X: rect.Min.X, Y: rect.Max.Y},
		math2d.Vec2{X: rect.Width(), Y: float64(rows) * u.theme.Row.Height},
	)
}

func dropdownPopupID(id string) string { return id + "#popup" }

func (u *UI) ensureOpen() {
	if u.state.open == nil {
		u.state.open = make(map[ID]bool)
	}
}
