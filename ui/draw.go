package ui

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Draw emits raw, theme-independent shapes stacked in call order (each call
// takes the next layer via nextLayer()). Unlike Panel/Button/Label, these
// carry explicit colors and sizes instead of pulling from the theme — use
// them for game-specific chrome (HUD panels, badges, motifs) that doesn't
// belong to a themed widget.

// FillRect draws a solid rectangle with no stroke.
func (u *UI) FillRect(rect math2d.Rect, fill render.Color) { u.FillRectIn(rect, fill, SpaceRef) }

func (u *UI) FillRectIn(rect math2d.Rect, fill render.Color, space Space) {
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Rect:        u.rectToWorld(rect, space),
		Fill:        fill,
	})
}

// DrawText draws unrotated text at pos with an explicit color/size.
func (u *UI) DrawText(text string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline) {
	u.DrawTextIn(text, pos, color, size, align, baseline, SpaceRef)
}

func (u *UI) DrawTextIn(text string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline, space Space) {
	u.DrawTextRotIn(text, pos, color, size, 0, align, baseline, space)
}

// DrawTextRotIn draws text rotated by rotation radians around pos.
func (u *UI) DrawTextRotIn(text string, pos math2d.Vec2, color render.Color, size, rotation float64, align render.TextAlign, baseline render.TextBaseline, space Space) {
	u.commands = append(u.commands, render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Position:    u.toWorld(pos, space),
		Text:        text,
		Color:       color,
		Size:        u.sizeToWorld(size, space),
		Align:       align,
		Baseline:    baseline,
		Rotation:    rotation,
	})
}

// DrawCircle draws a circle with optional fill and/or stroke.
func (u *UI) DrawCircle(center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64) {
	u.DrawCircleIn(center, radius, fill, stroke, strokeWidth, SpaceRef)
}

func (u *UI) DrawCircleIn(center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64, space Space) {
	u.commands = append(u.commands, render.DrawCircle{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Center:      u.toWorld(center, space),
		Radius:      u.sizeToWorld(radius, space),
		Fill:        fill,
		Stroke:      stroke,
		StrokeWidth: u.sizeToWorld(strokeWidth, space),
	})
}

// DrawLine draws a straight line segment.
func (u *UI) DrawLine(from, to math2d.Vec2, width float64, color render.Color) {
	u.DrawLineIn(from, to, width, color, SpaceRef)
}

func (u *UI) DrawLineIn(from, to math2d.Vec2, width float64, color render.Color, space Space) {
	u.commands = append(u.commands, render.DrawLine{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		From:        u.toWorld(from, space),
		To:          u.toWorld(to, space),
		Width:       u.sizeToWorld(width, space),
		Color:       color,
	})
}

// DrawPoly draws a filled and/or stroked polygon from points.
func (u *UI) DrawPoly(fill, stroke render.Color, strokeWidth float64, points ...math2d.Vec2) {
	u.DrawPolyIn(fill, stroke, strokeWidth, SpaceRef, points...)
}

func (u *UI) DrawPolyIn(fill, stroke render.Color, strokeWidth float64, space Space, points ...math2d.Vec2) {
	worldPoints := make([]math2d.Vec2, len(points))
	for i, p := range points {
		worldPoints[i] = u.toWorld(p, space)
	}
	u.commands = append(u.commands, render.DrawPolygon{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Points:      worldPoints,
		Fill:        fill,
		Stroke:      stroke,
		StrokeWidth: u.sizeToWorld(strokeWidth, space),
	})
}
