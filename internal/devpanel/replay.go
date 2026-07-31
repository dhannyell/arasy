//go:build arr_dev

package devpanel

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

const (
	idRecord      = "devpanel#record"
	idSave        = "devpanel#save"
	idLoad        = "devpanel#load"
	idRecordings  = "devpanel#recordings"
	idReplayScrub = "devpanel#replay-scrub"
	idWatchPlay   = "devpanel#watch-play"
	idWatchPause  = "devpanel#watch-pause"
	idWatchStep   = "devpanel#watch-step"
	idTakeControl = "devpanel#take-control"
)

type replayLayout struct {
	record     []math2d.Rect
	picker     math2d.Rect
	hasPicker  bool
	scrub      math2d.Rect
	scrubLabel math2d.Rect
	watch      []math2d.Rect
	take       math2d.Rect
	status     math2d.Rect
}

// replayRects lays the tab out. The picker row only exists on a host that can
// list what it saved; in a browser the file picker is the list, so the row
// would be an empty control taking up space.
func replayRects(rect math2d.Rect, hasPicker bool) replayLayout {
	r := newRows(rect)
	record := ui.Columns(r.next(), 3, rowGap)

	var picker math2d.Rect
	if hasPicker {
		picker = r.next()
	}

	// The bar and its counter share one row. Watching wants the position
	// picker right above the transport it drives, and the panel has no room
	// to spend a whole row on a number.
	scrubRow := r.next()
	scrub, label := ui.CutLeft(scrubRow, scrubRow.Width()*0.7)
	_, label = ui.CutLeft(label, rowGap)

	return replayLayout{
		record:     record,
		picker:     picker,
		hasPicker:  hasPicker,
		scrub:      scrub,
		scrubLabel: label,
		watch:      ui.Columns(r.next(), 3, rowGap),
		take:       ui.Columns(r.next(), 2, rowGap)[0],
		status:     r.remaining(),
	}
}

func (p *Panel) updateReplay(in ui.InteractScope, rect math2d.Rect, state runtime.DriveState) {
	lay := replayRects(rect, p.rec.hasPicker())

	if in.Clicked(idRecord, lay.record[0]) {
		p.rec.toggle(p.driver)
	}
	if in.Clicked(idSave, lay.record[1]) {
		p.rec.save(p.driver)
	}
	if in.Clicked(idLoad, lay.record[2]) {
		p.loadRecording()
	}

	if lay.hasPicker {
		selected, _ := in.DropdownInput(idRecordings, lay.picker, len(p.rec.labels()), p.rec.selected)
		p.rec.selected = selected
	}

	// Scrubbing a recording picks the point to watch from. It repositions
	// without leaving watch mode, which is what separates it from Play and
	// Step — those are the gesture that takes control and forks.
	p.updateScrub(in, idReplayScrub, lay.scrub, state)

	// Watching is only possible over a timeline that exists. The controls
	// stay drawn either way, so their place on the panel does not move.
	if p.watchable(state) {
		if in.Clicked(idWatchPlay, lay.watch[0]) {
			p.control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
		}
		if in.Clicked(idWatchPause, lay.watch[1]) {
			p.control(runtime.ControlCommand{Kind: runtime.ControlReplayPause})
		}
		if in.Clicked(idWatchStep, lay.watch[2]) {
			p.control(runtime.ControlCommand{Kind: runtime.ControlReplayStep})
		}
	}

	// Taking control is the only way out of watch mode, and it is a plain
	// Play: the driver forks the timeline from wherever the watcher stopped,
	// exactly as it does when a rewound run is resumed.
	if state.ReplayMode && in.Clicked(idTakeControl, lay.take) {
		p.control(runtime.ControlCommand{Kind: runtime.ControlPlay})
		p.rec.status = "took control at tick " + formatUint(state.Tick) + "; branches from here"
	}
}

func (p *Panel) drawReplay(draw ui.DrawScope, rect math2d.Rect, state runtime.DriveState) {
	lay := replayRects(rect, p.rec.hasPicker())

	record := "Record"
	if p.rec.active() {
		record = "Stop"
	}
	draw.Button(idRecord, record, lay.record[0])
	draw.Button(idSave, "Save", lay.record[1])
	draw.Button(idLoad, "Load", lay.record[2])

	if lay.hasPicker {
		draw.Dropdown(idRecordings, lay.picker, p.rec.labels(), p.rec.selected)
	}

	p.drawScrub(draw, idReplayScrub, lay.scrub, state)
	p.text(draw, p.scrubLabel(state), leftOf(lay.scrubLabel), p.theme.Text.Muted, p.theme.Text.Body)

	draw.Button(idWatchPlay, "Play", lay.watch[0])
	draw.Button(idWatchPause, "Pause", lay.watch[1])
	draw.Button(idWatchStep, "Step", lay.watch[2])
	draw.Button(idTakeControl, "Take control", lay.take)

	if p.rec.status != "" {
		p.text(draw, p.rec.status, math2d.Vec2{
			X: lay.status.Min.X,
			Y: lay.status.Min.Y + p.theme.Text.Caption,
		}, p.theme.Text.Muted, p.theme.Text.Caption)
	}
}

func (p *Panel) watchable(state runtime.DriveState) bool {
	return state.MaxRecordedTick > 0
}

// loadRecording loads the picked recording on a host that lists them, and
// otherwise asks the host to produce some bytes. A host that answers
// immediately is applied on the spot; one that has to open a file picker
// answers later, through ApplyRecording.
func (p *Panel) loadRecording() {
	if p.rec.port == nil {
		p.rec.status = "load unavailable on this host"
		return
	}
	if p.newGame == nil {
		p.rec.status = "load unavailable: no game factory"
		return
	}

	if entry, lister, ok := p.rec.picked(); ok {
		data, err := lister.LoadNamed(entry.Name)
		if err != nil {
			p.log.Error("load recording failed", "name", entry.Name, "err", err)
			p.rec.status = "load failed: " + err.Error()
			return
		}
		p.ApplyRecording(data, entry.Label)
		return
	}
	if p.rec.hasPicker() {
		p.rec.status = "nothing to load - no recordings saved yet"
		return
	}

	data, name, ok, err := p.rec.port.Load()
	if err != nil {
		p.log.Error("load recording failed", "err", err)
		p.rec.status = "load failed: " + err.Error()
		return
	}
	if !ok {
		return
	}
	p.ApplyRecording(data, name)
}

// ApplyRecording verifies bytes and hands the panel a driver replaying them,
// paused at tick 0 in watch mode — a loaded recording is something to watch
// from the start, not a run already at its end. Hosts whose Load is
// asynchronous call this when the bytes arrive.
func (p *Panel) ApplyRecording(data []byte, name string) {
	if p.newGame == nil {
		p.rec.status = "load unavailable: no game factory"
		return
	}

	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		p.log.Error("read recording failed", "name", name, "err", err)
		p.rec.status = "load failed: " + err.Error()
		return
	}
	if err := runtime.VerifyRecording(p.newGame(), p.log, rec); err != nil {
		p.log.Error("verify recording failed", "name", name, "err", err)
		p.rec.status = "verify failed: " + err.Error()
		return
	}

	driver := runtime.ReplayForViewing(p.newGame(), p.log, rec)
	p.adoptDriver(driver)
	p.control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick})
	p.rec.clear()
	p.rec.status = "loaded " + name + " (" + formatUint(rec.Ticks) + " ticks)"
	// Land the user on the tab that explains what just happened. The index is
	// the panel's own, so setting it here is the same path a click takes —
	// nothing reaches into ui.State.
	p.enterTab(tabReplay)
}

func (p *Panel) adoptDriver(driver *runtime.Driver) {
	p.driver = driver
	if p.adopt != nil {
		p.adopt(driver)
	}
}
