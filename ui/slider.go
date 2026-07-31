package ui

import (
	"math"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// SliderOptions defines the numeric interval of a horizontal slider.
// Step <= 0 keeps the value continuous; a positive Step snaps values to
// increments measured from Min.
type SliderOptions struct {
	Min  float64
	Max  float64
	Step float64
}

// sliderInputIn consumes pointer interaction for a horizontal slider. The
// caller owns value and should persist the returned value in game state when
// changed is true. A primary press anywhere in rect captures the slider and
// jumps to that position; dragging remains captured outside rect and clamps
// to the numeric interval.
func (u *UI) sliderInputIn(
	id string,
	rect math2d.Rect,
	value float64,
	opts SliderOptions,
	space Space,
) (next float64, changed bool) {
	original := value
	value = normalizeSliderValue(value, opts)
	track, _, ok := sliderGeometry(rect, u.theme.Slider)
	if !ok || opts.Max <= opts.Min {
		return value, value != original
	}

	wid := HashID(id)
	setFromScreen := func(screen math2d.Vec2) {
		pos := u.cameraFor(space).ScreenToWorld(screen)
		value = sliderValueAt(pos.X, track, opts)
	}

	for _, event := range u.frame.Events {
		if event.Button != input.PointerPrimary {
			continue
		}

		pos := u.cameraFor(space).ScreenToWorld(event.Pos)
		switch event.Kind {
		case input.EventButtonDown:
			if rect.Contains(pos) && u.state.activeID == noID {
				u.state.activeID = wid
				u.state.focusID = wid
				setFromScreen(event.Pos)
			}
		case input.EventButtonUp, input.EventPointerLeave:
			if u.state.activeID == wid {
				setFromScreen(event.Pos)
				u.state.activeID = noID
			}
		}
	}

	// Pointer moves are coalesced into frame.Pos. activeID preserves capture
	// across ticks while Buttons reports whether the primary button remains down.
	if u.state.activeID == wid && u.frame.Buttons[input.PointerPrimary] {
		setFromScreen(u.frame.Pos)
	}

	return value, value != original
}

// sliderIn draws a horizontal slider. Call InteractScope.SliderInput with the
// same id, rect and options during Update to make it interactive.
func (u *UI) sliderIn(id string, rect math2d.Rect, value float64, opts SliderOptions, space Space) {
	track, thumbRadius, ok := sliderGeometry(rect, u.theme.Slider)
	if !ok || opts.Max <= opts.Min {
		return
	}

	value = normalizeSliderValue(value, opts)
	fraction := (value - opts.Min) / (opts.Max - opts.Min)
	thumbCenter := math2d.Vec2{
		X: track.Min.X + track.Width()*fraction,
		Y: rect.Center().Y,
	}

	thumbColor := u.theme.Slider.Thumb
	if u.state.activeID == HashID(id) {
		thumbColor = u.theme.Slider.ThumbActive
	} else if u.inside && rect.Contains(u.cursorIn(space)) {
		thumbColor = u.theme.Slider.ThumbHovered
	}

	layer := u.nextLayer()
	commands := []render.Command{
		render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer},
			Rect:         u.rectToWorld(track, space),
			Fill:         u.theme.Slider.Track,
			Stroke:       u.theme.Slider.Border,
			StrokeWidth:  u.sizeToWorld(u.theme.Slider.BorderWidth, space),
			CornerRadius: u.sizeToWorld(track.Height()/2, space),
		},
	}
	if fraction > 0 {
		fill := math2d.NewRect(track.Min, math2d.Vec2{X: track.Width() * fraction, Y: track.Height()})
		commands = append(commands, render.DrawRect{
			BaseCommand:  render.BaseCommand{LayerValue: layer + 1},
			Rect:         u.rectToWorld(fill, space),
			Fill:         u.theme.Slider.Fill,
			CornerRadius: u.sizeToWorld(track.Height()/2, space),
		})
	}
	commands = append(commands, render.DrawCircle{
		BaseCommand: render.BaseCommand{LayerValue: layer + 2},
		Center:      u.toWorld(thumbCenter, space),
		Radius:      u.sizeToWorld(thumbRadius, space),
		Fill:        thumbColor,
		Stroke:      u.theme.Slider.Border,
		StrokeWidth: u.sizeToWorld(u.theme.Slider.BorderWidth, space),
	})
	u.commands = append(u.commands, commands...)

	if u.state.focusID == HashID(id) {
		u.focusRingIn(rect, layer+3, space)
	}
}

func normalizeSliderValue(value float64, opts SliderOptions) float64 {
	if opts.Max <= opts.Min {
		return opts.Min
	}
	if math.IsNaN(value) {
		value = opts.Min
	}
	value = clamp(value, opts.Min, opts.Max)
	if opts.Step > 0 && !math.IsInf(opts.Step, 0) {
		steps := math.Round((value - opts.Min) / opts.Step)
		snapped := opts.Min + steps*opts.Step
		// Max remains reachable when the interval is not an exact multiple of
		// Step. Treat it as the final snap point and choose the closest value.
		if math.Abs(opts.Max-value) < math.Abs(snapped-value) {
			snapped = opts.Max
		}
		value = snapped
	}
	return clamp(value, opts.Min, opts.Max)
}

func sliderValueAt(x float64, track math2d.Rect, opts SliderOptions) float64 {
	fraction := clamp((x-track.Min.X)/track.Width(), 0, 1)
	return normalizeSliderValue(opts.Min+fraction*(opts.Max-opts.Min), opts)
}

func sliderGeometry(rect math2d.Rect, theme SliderTheme) (track math2d.Rect, thumbRadius float64, ok bool) {
	if rect.Width() <= 0 || rect.Height() <= 0 || theme.ThumbRadius <= 0 || theme.TrackHeight <= 0 {
		return math2d.Rect{}, 0, false
	}

	thumbRadius = math.Min(theme.ThumbRadius, math.Min(rect.Height()/2, rect.Width()/2))
	trackWidth := rect.Width() - 2*thumbRadius
	if trackWidth <= 0 {
		return math2d.Rect{}, 0, false
	}
	trackHeight := math.Min(theme.TrackHeight, rect.Height())
	track = math2d.NewRect(
		math2d.Vec2{X: rect.Min.X + thumbRadius, Y: rect.Center().Y - trackHeight/2},
		math2d.Vec2{X: trackWidth, Y: trackHeight},
	)
	return track, thumbRadius, true
}
