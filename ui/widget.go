package ui

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func (u *UI) buttonIn(id, label string, rect math2d.Rect, space Space) {
	wid := HashID(id)
	hot := rect.Contains(u.cursorIn(space)) && u.inside

	hoverTarget := 0.0
	if hot {
		hoverTarget = 1.0
	}
	pressTarget := 0.0
	if u.state.activeID == wid {
		pressTarget = 1.0
	}

	// Approach blends: Normal(0)↔Hovered(1) and normal(0)↔Active(1).
	// speed=0 or dt=0 (Frame(0)/Commands) produces an instant jump.
	hoverBlend := u.Approach(id+"_h", hoverTarget, u.theme.Button.HoverSpeed)
	pressBlend := u.Approach(id+"_p", pressTarget, u.theme.Button.PressSpeed)

	style := lerpButtonStyle(
		lerpButtonStyle(u.theme.Button.Normal, u.theme.Button.Hovered, hoverBlend),
		u.theme.Button.Active,
		pressBlend,
	)

	layer := u.nextLayer()
	u.commands = append(u.commands,
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer},
			Rect:         u.rectToWorld(rect, space),
			Fill:         style.Background,
			Stroke:       style.Border,
			StrokeWidth:  u.sizeToWorld(style.BorderWidth, space),
			CornerRadius: u.sizeToWorld(u.theme.Focus.CornerRadius, space),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    u.toWorld(rect.Center(), space),
			Text:        label,
			Color:       style.TextColor,
			Size:        u.sizeToWorld(style.TextSize, space),
			Font:        u.theme.Font,
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		})
	if u.state.focusID == wid {
		focusRect := insetRect(rect, -u.theme.Focus.Inset)
		u.commands = append(u.commands, render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer + 2},
			Rect:         u.rectToWorld(focusRect, space),
			Stroke:       u.theme.Focus.Color,
			StrokeWidth:  u.sizeToWorld(u.theme.Focus.Width, space),
			CornerRadius: u.sizeToWorld(u.theme.Focus.CornerRadius, space),
		})
	}
}

func insetRect(rect math2d.Rect, inset float64) math2d.Rect {
	return math2d.NewRectFromCorners(
		math2d.Vec2{X: rect.Min.X + inset, Y: rect.Min.Y + inset},
		math2d.Vec2{X: rect.Max.X - inset, Y: rect.Max.Y - inset},
	)
}

func lerpButtonStyle(a, b ButtonStyle, t float64) ButtonStyle {
	lerp := func(x, y float64) float64 { return x + (y-x)*t }
	return ButtonStyle{
		Background:  a.Background.Lerp(b.Background, t),
		Border:      a.Border.Lerp(b.Border, t),
		BorderWidth: lerp(a.BorderWidth, b.BorderWidth),
		TextColor:   a.TextColor.Lerp(b.TextColor, t),
		TextSize:    lerp(a.TextSize, b.TextSize),
	}
}

func (u *UI) panelIn(rect math2d.Rect, space Space) {
	layer := u.nextLayer()

	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Rect:        u.rectToWorld(rect, space),
		Fill:        u.theme.Panel.Background,
		Stroke:      u.theme.Panel.Border,
		StrokeWidth: u.sizeToWorld(u.theme.Panel.BorderWidth, space),
	})
}

func (u *UI) labelIn(text string, pos math2d.Vec2, space Space) {
	layer := u.nextLayer()

	u.commands = append(u.commands, render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Position:    u.toWorld(pos, space),
		Text:        text,
		Color:       u.theme.Label.Color,
		Size:        u.sizeToWorld(u.theme.Label.Size, space),
		Font:        u.theme.Font,
		Align:       render.AlignCenter,
		Baseline:    render.BaselineMiddle,
	})
}

func (u *UI) progressBarIn(rect math2d.Rect, value float64, space Space) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	layer := u.nextLayer()
	fillRect := math2d.NewRect(rect.Min, math2d.Vec2{
		X: rect.Width() * value,
		Y: rect.Height(),
	})

	u.commands = append(u.commands,
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Rect:        u.rectToWorld(rect, space),
			Fill:        u.theme.Progress.Background,
			Stroke:      u.theme.Progress.Border,
			StrokeWidth: u.sizeToWorld(u.theme.Progress.BorderWidth, space),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Rect:        u.rectToWorld(fillRect, space),
			Fill:        u.theme.Progress.Fill,
		},
	)
}
