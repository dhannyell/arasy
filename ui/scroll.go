package ui

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const (
	scrollWheelStep       = 32
	scrollbarMinThumbSize = 16
	scrollbarWidth        = 6
)

func (u *UI) scrollOffsetIn(id string, view math2d.Rect, contentH float64, space Space) float64 {
	wid := HashID(id)
	u.ensureScroll()

	offset := u.state.scroll[wid]
	if u.inside && view.Contains(u.cursorIn(space)) {
		offset += u.frame.Wheel.Y * scrollWheelStep
	}
	offset = clamp(offset, 0, maxScrollOffset(view, contentH))
	u.state.scroll[wid] = offset
	return offset
}

func (u *UI) scrollAreaIn(id string, view math2d.Rect, contentH float64, space Space) (float64, math2d.Rect) {
	wid := HashID(id)
	u.ensureScroll()

	offset := clamp(u.state.scroll[wid], 0, maxScrollOffset(view, contentH))
	u.state.scroll[wid] = offset
	clip := u.rectToWorld(view, space)

	maxOffset := maxScrollOffset(view, contentH)
	if maxOffset <= 0 || view.Height() <= 0 || contentH <= 0 {
		return offset, clip
	}

	track := math2d.NewRect(
		math2d.Vec2{X: view.Max.X - scrollbarWidth, Y: view.Min.Y},
		math2d.Vec2{X: scrollbarWidth, Y: view.Height()},
	)
	thumbH := math.Max(scrollbarMinThumbSize, view.Height()*view.Height()/contentH)
	thumbH = math.Min(thumbH, view.Height())
	thumbY := view.Min.Y
	if maxOffset > 0 && view.Height() > thumbH {
		thumbY += (offset / maxOffset) * (view.Height() - thumbH)
	}
	thumb := math2d.NewRect(
		math2d.Vec2{X: track.Min.X, Y: thumbY},
		math2d.Vec2{X: scrollbarWidth, Y: thumbH},
	)

	layer := u.nextLayer()
	u.commands = append(u.commands,
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer},
			Rect:         u.rectToWorld(track, space),
			Fill:         render.RGBA(1, 1, 1, 0.10),
			CornerRadius: u.sizeToWorld(scrollbarWidth/2, space),
		},
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer + 1},
			Rect:         u.rectToWorld(thumb, space),
			Fill:         render.RGBA(1, 1, 1, 0.36),
			CornerRadius: u.sizeToWorld(scrollbarWidth/2, space),
		},
	)

	return offset, clip
}

func (u *UI) ensureScroll() {
	if u.state.scroll == nil {
		u.state.scroll = make(map[ID]float64)
	}
}

func maxScrollOffset(view math2d.Rect, contentH float64) float64 {
	return math.Max(0, contentH-view.Height())
}

func clamp(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
