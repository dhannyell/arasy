package devpanel

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

const (
	idScrub      = "devpanel#scrub"
	idTick       = "devpanel#tick"
	idGoTick     = "devpanel#go-tick"
	idBack       = "devpanel#back"
	idBackSecond = "devpanel#back-second"
)

type timelineLayout struct {
	scrub      math2d.Rect
	scrubLabel math2d.Rect
	tick       math2d.Rect
	goTick     math2d.Rect
	back       []math2d.Rect
}

func timelineRects(rect math2d.Rect) timelineLayout {
	r := newRows(rect)
	scrub := r.next()
	label := r.next()
	tickRow := r.next()
	tick, goTick := ui.CutLeft(tickRow, tickRow.Width()*0.6)
	_, goTick = ui.CutLeft(goTick, rowGap)

	return timelineLayout{
		scrub:      scrub,
		scrubLabel: label,
		tick:       tick,
		goTick:     goTick,
		back:       ui.Columns(r.next(), 2, rowGap),
	}
}

func (p *Panel) updateTimeline(in ui.InteractScope, rect math2d.Rect, state runtime.DriveState) {
	lay := timelineRects(rect)
	maxTick := maxRecordedTick(state)
	scrubRange := ui.SliderOptions{Max: float64(maxTick), Step: 1}

	// Scrubbing seeks on release, not while dragging: every intermediate
	// position would be a reconstruction of the world, and dragging across a
	// thousand ticks would rebuild a thousand of them. The thumb still
	// follows the pointer — that preview is the whole point of a scrub bar.
	dragging := p.state.ActiveID() == ui.HashID(idScrub)
	if !dragging && !p.scrubActive {
		p.scrub = float64(state.Tick)
	}
	next, _ := in.SliderInput(idScrub, lay.scrub, p.scrub, scrubRange)
	p.scrub = next

	dragging = p.state.ActiveID() == ui.HashID(idScrub)
	if p.scrubActive && !dragging {
		p.control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: uint64(p.scrub)})
	}
	p.scrubActive = dragging

	result := in.TextInput(idTick, lay.tick, p.tickText, ui.TextInputOptions{Placeholder: "tick"})
	p.tickText = result.Text
	if result.Submitted || in.Clicked(idGoTick, lay.goTick) {
		if tick, ok := parseTick(p.tickText); ok {
			p.control(runtime.ControlCommand{Kind: runtime.ControlBackToTick, Tick: tick})
		}
	}

	if in.Clicked(idBack, lay.back[0]) {
		p.control(runtime.ControlCommand{Kind: runtime.ControlBack})
	}
	if in.Clicked(idBackSecond, lay.back[1]) {
		p.control(runtime.ControlCommand{Kind: runtime.ControlBackSeconds, Seconds: 1})
	}
}

func (p *Panel) drawTimeline(draw ui.DrawScope, rect math2d.Rect, state runtime.DriveState) {
	lay := timelineRects(rect)
	maxTick := maxRecordedTick(state)

	draw.Slider(idScrub, lay.scrub, p.scrub, ui.SliderOptions{Max: float64(maxTick), Step: 1})
	p.text(draw, "scrub "+formatUint(uint64(p.scrub))+"/"+formatUint(maxTick),
		leftOf(lay.scrubLabel), p.theme.Text.Color, p.theme.Text.Body)

	draw.TextInput(idTick, lay.tick, p.tickText, ui.TextInputOptions{Placeholder: "tick"})
	draw.Button(idGoTick, "Go", lay.goTick)
	draw.Button(idBack, "Back 1 tick", lay.back[0])
	draw.Button(idBackSecond, "Back 1 s", lay.back[1])
}
