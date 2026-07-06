package ui

import (
	"math"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

const (
	tableCellPaddingX = 8
	tableTextSize     = 14
)

type TableColumn struct {
	Title string
	Width float64
	Align render.TextAlign
}

type TableCellStyle struct {
	Color render.Color
}

func (u *UI) Card(rect math2d.Rect, accent render.Color) { u.CardIn(rect, accent, SpaceRef) }

func (u *UI) CardIn(rect math2d.Rect, accent render.Color, space Space) {
	layer := u.nextLayer()
	card := u.theme.Card
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(rect, space),
		Fill:         card.Background,
		Stroke:       card.Border,
		StrokeWidth:  u.sizeToWorld(card.BorderWidth, space),
		CornerRadius: u.sizeToWorld(card.CornerRadius, space),
	})

	if card.AccentWidth <= 0 || accent.A <= 0 {
		return
	}
	accentRect := math2d.NewRect(rect.Min, math2d.Vec2{X: card.AccentWidth, Y: rect.Height()})
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: layer + 1},
		Rect:        u.rectToWorld(accentRect, space),
		Fill:        accent,
	})
}

func (u *UI) Badge(text string, center math2d.Vec2, bg render.Color) {
	u.BadgeIn(text, center, bg, SpaceRef)
}

func (u *UI) BadgeIn(text string, center math2d.Vec2, bg render.Color, space Space) {
	badge := u.theme.Badge
	size := render.MeasureText(text, badge.TextSize)
	rect := math2d.NewRect(
		math2d.Vec2{
			X: center.X - size.X/2 - badge.PaddingX,
			Y: center.Y - size.Y/2 - badge.PaddingY,
		},
		math2d.Vec2{
			X: size.X + badge.PaddingX*2,
			Y: size.Y + badge.PaddingY*2,
		},
	)
	if bg.A <= 0 {
		bg = badge.Background
	}

	layer := u.nextLayer()
	u.commands = append(u.commands,
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer},
			Rect:         u.rectToWorld(rect, space),
			Fill:         bg,
			Stroke:       badge.Border,
			StrokeWidth:  u.sizeToWorld(badge.BorderWidth, space),
			CornerRadius: u.sizeToWorld(badge.CornerRadius, space),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    u.toWorld(center, space),
			Text:        text,
			Color:       badge.TextColor,
			Size:        u.sizeToWorld(badge.TextSize, space),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		},
	)
}

func (u *UI) Table(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, cell func(row, col int) string) int {
	return u.TableIn(id, rect, cols, rowCount, selected, cell, SpaceRef)
}

func (u *UI) TableHitIn(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, space Space) int {
	_ = cols
	_ = selected
	wid := HashID(id)
	body := tableBodyRect(rect, u.theme.Header.Height)
	offset := 0.0
	if u.state.scroll != nil {
		offset = u.state.scroll[wid]
	}

	clicked := -1
	for _, e := range u.frame.Events {
		if e.Button != input.PointerPrimary {
			continue
		}
		pos := u.cameraFor(space).ScreenToWorld(e.Pos)
		switch e.Kind {
		case input.EventButtonDown:
			if body.Contains(pos) && u.state.activeID == noID {
				u.state.activeID = wid
				u.state.focusID = wid
			}
		case input.EventButtonUp, input.EventPointerLeave:
			if u.state.activeID != wid {
				continue
			}
			if e.Kind == input.EventButtonUp && body.Contains(pos) {
				row := int(math.Floor((pos.Y - body.Min.Y + offset) / u.theme.Row.Height))
				if row >= 0 && row < rowCount {
					clicked = row
				}
			}
			u.state.activeID = noID
		}
	}
	return clicked
}

func (u *UI) TableIn(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, cell func(row, col int) string, space Space) int {
	return u.TableStyledIn(id, rect, cols, rowCount, selected, cell, nil, space)
}

func (u *UI) TableStyledIn(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, cell func(row, col int) string, style func(row, col int) TableCellStyle, space Space) int {
	if cell == nil {
		cell = func(row, col int) string { return "" }
	}
	if len(cols) == 0 || rowCount < 0 {
		rowCount = max(rowCount, 0)
	}
	widths := resolveTableWidths(rect.Width(), cols)
	header := math2d.NewRect(rect.Min, math2d.Vec2{X: rect.Width(), Y: u.theme.Header.Height})
	body := tableBodyRect(rect, u.theme.Header.Height)
	contentH := float64(rowCount) * u.theme.Row.Height

	layer := u.nextLayer()
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand:  render.BaseCommand{LayerValue: layer},
		Rect:         u.rectToWorld(header, space),
		Fill:         u.theme.Header.Background,
		Stroke:       u.theme.Header.Border,
		StrokeWidth:  u.sizeToWorld(u.theme.Header.BorderWidth, space),
		CornerRadius: u.sizeToWorld(u.theme.Header.CornerRadius, space),
	})

	x := rect.Min.X
	for col, width := range widths {
		u.commands = append(u.commands, render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    u.toWorld(textPosition(x, header, width, cols[col].Align), space),
			Text:        cols[col].Title,
			Color:       u.theme.Header.TextColor,
			Size:        u.sizeToWorld(tableTextSize, space),
			Align:       cols[col].Align,
			Baseline:    render.BaselineMiddle,
		})
		x += width
	}

	offset, clip := u.ScrollAreaIn(id, body, contentH, space)
	rowH := u.theme.Row.Height
	first := int(math.Floor(offset / rowH))
	last := int(math.Ceil((offset + body.Height()) / rowH))
	first = max(first, 0)
	last = min(last, rowCount-1)

	cursor := u.cursorIn(space)
	for row := first; row <= last; row++ {
		rowRect := math2d.NewRect(
			math2d.Vec2{X: body.Min.X, Y: body.Min.Y + float64(row)*rowH - offset},
			math2d.Vec2{X: body.Width(), Y: rowH},
		)
		fill := u.theme.Row.Normal
		if row%2 == 1 {
			fill = u.theme.Row.Zebra
		}
		if selected == row {
			fill = u.theme.Row.Selected
		} else if u.inside && rowRect.Contains(cursor) {
			fill = u.theme.Row.Hovered
		}
		if fill.A > 0 {
			u.commands = append(u.commands, render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: layer + 2, Clip: clip},
				Rect:        u.rectToWorld(rowRect, space),
				Fill:        fill,
			})
		}

		x = rect.Min.X
		for col, width := range widths {
			textColor := u.theme.Row.TextColor
			if style != nil {
				if styled := style(row, col); styled.Color.A > 0 {
					textColor = styled.Color
				}
			}
			u.commands = append(u.commands, render.DrawText{
				BaseCommand: render.BaseCommand{LayerValue: layer + 3, Clip: clip},
				Position:    u.toWorld(textPosition(x, rowRect, width, cols[col].Align), space),
				Text:        cell(row, col),
				Color:       textColor,
				Size:        u.sizeToWorld(tableTextSize, space),
				Align:       cols[col].Align,
				Baseline:    render.BaselineMiddle,
			})
			x += width
		}
	}

	if clicked := u.TableHitIn(id, rect, cols, rowCount, selected, space); clicked >= 0 {
		return clicked
	}
	return -1
}

func tableBodyRect(rect math2d.Rect, headerH float64) math2d.Rect {
	return math2d.NewRect(
		math2d.Vec2{X: rect.Min.X, Y: rect.Min.Y + headerH},
		math2d.Vec2{X: rect.Width(), Y: math.Max(0, rect.Height()-headerH)},
	)
}

func resolveTableWidths(total float64, cols []TableColumn) []float64 {
	widths := make([]float64, len(cols))
	fixed := 0.0
	flex := 0
	for i, col := range cols {
		if col.Width > 0 {
			widths[i] = col.Width
			fixed += col.Width
		} else {
			flex++
		}
	}
	flexWidth := 0.0
	if flex > 0 {
		flexWidth = math.Max(0, total-fixed) / float64(flex)
	}
	for i := range widths {
		if widths[i] == 0 {
			widths[i] = flexWidth
		}
	}
	return widths
}

func textPosition(x float64, rect math2d.Rect, width float64, align render.TextAlign) math2d.Vec2 {
	switch align {
	case render.AlignRight:
		return math2d.Vec2{X: x + width - tableCellPaddingX, Y: rect.Center().Y}
	case render.AlignCenter:
		return math2d.Vec2{X: x + width/2, Y: rect.Center().Y}
	default:
		return math2d.Vec2{X: x + tableCellPaddingX, Y: rect.Center().Y}
	}
}
