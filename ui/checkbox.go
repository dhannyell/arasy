package ui

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Checkbox is caller-owned, like the slider and the text field: a filter being
// on is something the game means, so the flag lives in game state and rewinds
// with it. The widget only reports the toggle.

// checkboxInputIn is called in Update. It returns the flag the caller should
// hold next. Clicking anywhere in rect toggles, and so does activating the
// widget from the keyboard, which is the same path a click takes.
func (u *UI) checkboxInputIn(id string, rect math2d.Rect, checked bool, space Space) (bool, bool) {
	if !u.activatedIn(id, rect, space) {
		return checked, false
	}
	return !checked, true
}

// checkboxIn draws the box, its mark and an optional label to the right.
func (u *UI) checkboxIn(id string, rect math2d.Rect, checked bool, label string, space Space) {
	theme := u.theme.Checkbox
	box := checkboxBoxRect(rect, theme)
	if box.Width() <= 0 || box.Height() <= 0 {
		return
	}

	fill := theme.Background
	switch {
	case checked:
		fill = theme.BackgroundChecked
	case u.hoveredIn(rect, space):
		fill = theme.BackgroundHovered
	}

	layer := u.nextLayer()
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(box, space),
		Fill:         fill,
		Stroke:       theme.Border,
		StrokeWidth:  u.sizeToWorld(theme.BorderWidth, space),
		CornerRadius: u.sizeToWorld(theme.CornerRadius, space),
	})

	if checked {
		// A check mark as two strokes: down to the low point, then up to the
		// top right.
		low := math2d.Vec2{X: box.Min.X + box.Width()*0.42, Y: box.Min.Y + box.Height()*0.72}
		u.commands = append(u.commands,
			render.DrawLine{
				BaseCommand: render.BaseCommand{LayerValue: layer + 1},
				From:        u.toWorld(math2d.Vec2{X: box.Min.X + box.Width()*0.22, Y: box.Center().Y}, space),
				To:          u.toWorld(low, space),
				Width:       u.sizeToWorld(theme.MarkWidth, space),
				Color:       theme.Mark,
			},
			render.DrawLine{
				BaseCommand: render.BaseCommand{LayerValue: layer + 1},
				From:        u.toWorld(low, space),
				To:          u.toWorld(math2d.Vec2{X: box.Min.X + box.Width()*0.78, Y: box.Min.Y + box.Height()*0.28}, space),
				Width:       u.sizeToWorld(theme.MarkWidth, space),
				Color:       theme.Mark,
			},
		)
	}

	if label != "" {
		u.commands = append(u.commands, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 2},
			Position:    u.toWorld(math2d.Vec2{X: box.Max.X + theme.LabelGap, Y: rect.Center().Y}, space),
			Text:        label,
			Color:       theme.TextColor,
			Size:        u.sizeToWorld(theme.TextSize, space),
			Font:        u.theme.Font,
			Align:       render.AlignLeft,
			Baseline:    render.BaselineMiddle,
		})
	}

	if u.state.focusID == HashID(id) {
		u.focusRingIn(rect, layer+3, space)
	}
}

// checkboxBoxRect centres the square on the left of the rect the caller gave,
// leaving the rest of it as the label's — and as part of the hit target, so
// clicking the label toggles too.
func checkboxBoxRect(rect math2d.Rect, theme CheckboxTheme) math2d.Rect {
	size := math.Min(theme.BoxSize, math.Min(rect.Width(), rect.Height()))
	return math2d.NewRect(
		math2d.Vec2{X: rect.Min.X, Y: rect.Center().Y - size/2},
		math2d.Vec2{X: size, Y: size},
	)
}

// focusRingIn draws the themed focus ring around rect. Every focusable widget
// draws the same ring, so keyboard focus reads the same everywhere.
func (u *UI) focusRingIn(rect math2d.Rect, layer int, space Space) {
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(insetRect(rect, -u.theme.Focus.Inset), space),
		Stroke:       u.theme.Focus.Color,
		StrokeWidth:  u.sizeToWorld(u.theme.Focus.Width, space),
		CornerRadius: u.sizeToWorld(u.theme.Focus.CornerRadius, space),
	})
}
