package devpanel

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

const (
	idPlayPause = "devpanel#play-pause"
	idStep      = "devpanel#step"
	idReset     = "devpanel#reset"
	idSpeed     = "devpanel#speed"
)

// speedRange mirrors the interval the driver clamps to, so the slider cannot
// ask for a speed the driver will silently refuse.
var speedRange = ui.SliderOptions{Min: 0.25, Max: 8, Step: 0.25}

type simLayout struct {
	buttons    []math2d.Rect
	speedLabel math2d.Rect
	speed      math2d.Rect
}

func simRects(rect math2d.Rect) simLayout {
	r := newRows(rect)
	return simLayout{
		buttons:    ui.Columns(r.next(), 3, rowGap),
		speedLabel: r.next(),
		speed:      r.next(),
	}
}

func (p *Panel) updateSim(in ui.InteractScope, rect math2d.Rect, state runtime.DriveState) {
	lay := simRects(rect)

	if in.Clicked(idPlayPause, lay.buttons[0]) {
		kind := runtime.ControlPause
		if state.Paused {
			kind = runtime.ControlPlay
		}
		p.control(runtime.ControlCommand{Kind: kind})
	}
	if in.Clicked(idStep, lay.buttons[1]) {
		p.control(runtime.ControlCommand{Kind: runtime.ControlStep})
	}
	if in.Clicked(idReset, lay.buttons[2]) {
		p.control(runtime.ControlCommand{Kind: runtime.ControlReset})
	}

	// While the thumb is not being dragged the slider mirrors the driver, so
	// a speed changed by any other path shows up here. During a drag it holds
	// the value the pointer is on, which is the one the user is looking at.
	if p.state.ActiveID() != ui.HashID(idSpeed) {
		p.speed = state.Speed
	}
	next, changed := in.SliderInput(idSpeed, lay.speed, p.speed, speedRange)
	p.speed = next
	if changed {
		p.control(runtime.ControlCommand{Kind: runtime.ControlSetSpeed, Speed: next})
	}
}

func (p *Panel) drawSim(draw ui.DrawScope, rect math2d.Rect, state runtime.DriveState) {
	lay := simRects(rect)

	playPause := "Pause"
	if state.Paused {
		playPause = "Play"
	}
	draw.Button(idPlayPause, playPause, lay.buttons[0])
	draw.Button(idStep, "+1 tick", lay.buttons[1])
	draw.Button(idReset, "Reset", lay.buttons[2])

	p.text(draw, "speed "+formatFloat(p.speed), leftOf(lay.speedLabel),
		p.theme.Text.Color, p.theme.Text.Body)
	draw.Slider(idSpeed, lay.speed, p.speed, speedRange)
}

// leftOf is the anchor of a left-aligned, vertically centred label in a row.
func leftOf(rect math2d.Rect) math2d.Vec2 {
	return math2d.Vec2{X: rect.Min.X, Y: rect.Center().Y}
}
