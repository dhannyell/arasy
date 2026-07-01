package ui

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func (u *UI) Button(id, label string, rect math2d.Rect) { u.ButtonIn(id, label, rect, SpaceRef) }

func (u *UI) ButtonIn(id, label string, rect math2d.Rect, space Space) {
	wid := HashID(id)
	hot := rect.Contains(u.cursorIn(space)) && u.inside

	style := u.theme.Button.Normal
	switch {
	case u.state.activeID == wid:
		style = u.theme.Button.Active
	case hot:
		style = u.theme.Button.Hovered
	}

	layer := u.nextLayer()
	u.commands = append(u.commands,
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Rect:        u.rectToWorld(rect, space),
			Fill:        style.Background,
			Stroke:      style.Border,
			StrokeWidth: u.sizeToWorld(style.BorderWidth, space),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    u.toWorld(rect.Center(), space),
			Text:        label,
			Color:       style.TextColor,
			Size:        u.sizeToWorld(style.TextSize, space),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		})
}

func (u *UI) Panel(rect math2d.Rect) { u.PanelIn(rect, SpaceRef) }

func (u *UI) PanelIn(rect math2d.Rect, space Space) {
	layer := u.nextLayer()

	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Rect:        u.rectToWorld(rect, space),
		Fill:        u.theme.Panel.Background,
		Stroke:      u.theme.Panel.Border,
		StrokeWidth: u.sizeToWorld(u.theme.Panel.BorderWidth, space),
	})
}

func (u *UI) Label(text string, pos math2d.Vec2) { u.LabelIn(text, pos, SpaceRef) }

func (u *UI) LabelIn(text string, pos math2d.Vec2, space Space) {
	layer := u.nextLayer()

	u.commands = append(u.commands, render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Position:    u.toWorld(pos, space),
		Text:        text,
		Color:       u.theme.Label.Color,
		Size:        u.sizeToWorld(u.theme.Label.Size, space),
		Align:       render.AlignCenter,
		Baseline:    render.BaselineMiddle,
	})
}

func (u *UI) ProgressBar(rect math2d.Rect, value float64) { u.ProgressBarIn(rect, value, SpaceRef) }

func (u *UI) ProgressBarIn(rect math2d.Rect, value float64, space Space) {
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
