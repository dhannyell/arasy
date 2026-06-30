package ui

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func (u *UI) Button(id, label string, rect math2d.Rect) {
	wid := HashID(id)
	hot := rect.Contains(u.cursorPos) && u.inside

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
			Rect:        u.screenRectToWorld(rect),
			Fill:        style.Background,
			Stroke:      style.Border,
			StrokeWidth: u.screenSizeToWorld(style.BorderWidth),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    u.screenToWorld(rect.Center()),
			Text:        label,
			Color:       style.TextColor,
			Size:        u.screenSizeToWorld(style.TextSize),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		})
}

func (u *UI) Panel(rect math2d.Rect) {
	layer := u.nextLayer()

	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Rect:        u.screenRectToWorld(rect),
		Fill:        u.theme.Panel.Background,
		Stroke:      u.theme.Panel.Border,
		StrokeWidth: u.screenSizeToWorld(u.theme.Panel.BorderWidth),
	})
}

func (u *UI) Label(text string, pos math2d.Vec2) {
	layer := u.nextLayer()

	u.commands = append(u.commands, render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: layer},
		Position:    u.screenToWorld(pos),
		Text:        text,
		Color:       u.theme.Label.Color,
		Size:        u.screenSizeToWorld(u.theme.Label.Size),
		Align:       render.AlignCenter,
		Baseline:    render.BaselineMiddle,
	})
}

func (u *UI) ProgressBar(rect math2d.Rect, value float64) {
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
			Rect:        u.screenRectToWorld(rect),
			Fill:        u.theme.Progress.Background,
			Stroke:      u.theme.Progress.Border,
			StrokeWidth: u.screenSizeToWorld(u.theme.Progress.BorderWidth),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Rect:        u.screenRectToWorld(fillRect),
			Fill:        u.theme.Progress.Fill,
		},
	)
}
