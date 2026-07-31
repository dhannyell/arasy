package devpanel

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

// Scrubbing appears on two tabs, over the same timeline: the Timeline tab
// seeks a live run, and the Replay tab picks a point in a recording to watch
// from. The behaviour is the same in both, so it lives here once.
//
// A seek fires on release, never while dragging. Every intermediate position
// would be a full reconstruction of the world, and dragging across a thousand
// ticks would rebuild a thousand of them. The thumb still follows the pointer
// while held — that preview is what a scrub bar is for — and only the tick it
// is let go on becomes a command.

// scrubRange is the seekable interval. The live tick is included because a
// run that has never been rewound is still scrubbable up to now.
func scrubRange(state runtime.DriveState) ui.SliderOptions {
	return ui.SliderOptions{Max: float64(maxRecordedTick(state)), Step: 1}
}

func (p *Panel) updateScrub(in ui.InteractScope, id string, rect math2d.Rect, state runtime.DriveState) {
	widget := ui.HashID(id)

	// While nothing is being dragged the bar mirrors the driver, so a tick
	// reached by any other path — playback, a step, a button — shows up here.
	if p.state.ActiveID() != widget && !p.scrubActive {
		p.scrub = float64(state.Tick)
	}

	next, _ := in.SliderInput(id, rect, p.scrub, scrubRange(state))
	p.scrub = next

	dragging := p.state.ActiveID() == widget
	if p.scrubActive && !dragging {
		p.control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: uint64(p.scrub)})
	}
	p.scrubActive = dragging
}

func (p *Panel) drawScrub(draw ui.DrawScope, id string, rect math2d.Rect, state runtime.DriveState) {
	draw.Slider(id, rect, p.scrub, scrubRange(state))
}

// scrubLabel is the position readout that goes beside a scrub bar: where the
// bar is now over how far it can go.
func (p *Panel) scrubLabel(state runtime.DriveState) string {
	return formatUint(uint64(p.scrub)) + "/" + formatUint(maxRecordedTick(state))
}
