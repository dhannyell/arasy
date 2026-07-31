package devpanel

import (
	"errors"
	"testing"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
)

// fakePort stands in for a host's file I/O. Save keeps the bytes; Load hands
// back whatever the test armed it with, which is how a recording made in one
// half of a test is loaded in the other.
type fakePort struct {
	saved     []byte
	savedName string
	saveErr   error

	pending  []byte
	loadName string
	loadErr  error
	// async models a host whose Load opens a picker: it reports nothing to
	// load and is expected to deliver later through ApplyRecording.
	async bool
}

func (p *fakePort) Save(name string, data []byte) error {
	if p.saveErr != nil {
		return p.saveErr
	}
	p.savedName = name
	p.saved = append([]byte(nil), data...)
	return nil
}

func (p *fakePort) Load() ([]byte, string, bool, error) {
	switch {
	case p.loadErr != nil:
		return nil, "", false, p.loadErr
	case p.async || p.pending == nil:
		return nil, "", false, nil
	default:
		return p.pending, p.loadName, true, nil
	}
}

// recordRun drives a full Record → run → Stop → Save through the panel's own
// buttons and returns the bytes the host was handed.
func recordRun(t *testing.T, p *Panel, port *fakePort, ticks int) []byte {
	t.Helper()
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())

	// Record queues a reset first, so the recording starts from tick 0 rather
	// than from the middle of a run Replay could never reconstruct. The reset
	// consumes one step without advancing; the frame after it starts the
	// capture.
	clickPanel(p, lay.record[0].Center())
	p.driver.Step()
	p.Render(frameCamera(), 16*time.Millisecond, Stats{})
	if !p.rec.active() {
		t.Fatal("Record did not start a recording")
	}

	advance(p, ticks)

	clickPanel(p, lay.record[0].Center())
	if p.rec.active() {
		t.Fatal("Stop did not stop the recording")
	}

	clickPanel(p, lay.record[1].Center())
	if port.saved == nil {
		t.Fatalf("Save wrote nothing (status %q)", p.rec.status)
	}
	if port.savedName != recordingFileName {
		t.Fatalf("saved as %q, want %q", port.savedName, recordingFileName)
	}
	return port.saved
}

func TestRecordStopSaveWritesThroughThePort(t *testing.T) {
	port := &fakePort{}
	p := openPanel(t, Options{Recording: port, NewGame: func() runtime.Game { return testGame{} }}, tabReplay)

	data := recordRun(t, p, port, 12)

	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		t.Fatalf("saved bytes are not a readable recording: %v", err)
	}
	if rec.Ticks != 12 {
		t.Fatalf("recording covers %d ticks, want 12", rec.Ticks)
	}
}

func TestSaveWithoutARecordingReportsIt(t *testing.T) {
	port := &fakePort{}
	p := openPanel(t, Options{Recording: port}, tabReplay)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())

	clickPanel(p, lay.record[1].Center())

	if port.saved != nil {
		t.Fatal("saved something with no recording")
	}
	if p.rec.status == "" {
		t.Fatal("Save with nothing to save said nothing")
	}
}

func TestLoadLeavesThePanelPausedAtTickZeroInWatchMode(t *testing.T) {
	port := &fakePort{}
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	port.pending = recordRun(t, p, port, 8)
	port.loadName = "run.arrp"

	var adopted *runtime.Driver
	p.adopt = func(d *runtime.Driver) { adopted = d }
	before := p.driver

	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())

	if p.driver == before {
		t.Fatal("Load did not swap the driver")
	}
	if adopted != p.driver {
		t.Fatal("the host was not told about the new driver")
	}

	// The rewind to the start is a control command like any other, so it
	// lands on the host's next pass over the driver.
	p.driver.AdvanceTime(0)

	state := p.driver.State()
	if !state.Paused {
		t.Fatal("loaded replay is running; it should wait to be watched")
	}
	if state.Tick != 0 {
		t.Fatalf("loaded replay sits at tick %d, want 0", state.Tick)
	}
	if !state.ReplayMode {
		t.Fatal("loaded replay is not in watch mode")
	}
	if state.MaxRecordedTick != 8 {
		t.Fatalf("scrub range ends at %d, want the recorded 8", state.MaxRecordedTick)
	}
	if p.activeTab != tabReplay {
		t.Fatalf("active tab = %d, want the Replay tab after a load", p.activeTab)
	}
}

// A host that has to open a file picker answers "nothing now" and delivers
// the bytes later; the outcome has to be the same.
func TestAsyncLoadAppliesLater(t *testing.T) {
	port := &fakePort{}
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	data := recordRun(t, p, port, 5)
	port.async = true
	before := p.driver

	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())
	if p.driver != before {
		t.Fatal("an async port swapped the driver synchronously")
	}

	p.ApplyRecording(data, "later.arrp")

	if p.driver == before {
		t.Fatal("delivered bytes did not swap the driver")
	}
	if !p.driver.State().ReplayMode {
		t.Fatal("delivered replay is not in watch mode")
	}
}

func TestLoadFailureLeavesTheDriverAlone(t *testing.T) {
	port := &fakePort{loadErr: errors.New("no such file")}
	p := openPanel(t, Options{Recording: port, NewGame: func() runtime.Game { return testGame{} }}, tabReplay)
	before := p.driver

	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())

	if p.driver != before {
		t.Fatal("a failed load swapped the driver")
	}
	if p.rec.status == "" {
		t.Fatal("a failed load said nothing")
	}
}

func TestCorruptRecordingIsRejected(t *testing.T) {
	p := openPanel(t, Options{NewGame: func() runtime.Game { return testGame{} }}, tabReplay)
	before := p.driver

	p.ApplyRecording([]byte("not a recording"), "junk.arrp")

	if p.driver != before {
		t.Fatal("corrupt bytes swapped the driver")
	}
}

func TestWatchPlayAndTakeControl(t *testing.T) {
	port := &fakePort{}
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	port.pending = recordRun(t, p, port, 6)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())

	clickPanel(p, lay.watch[0].Center())
	if state := p.driver.State(); !state.ReplayMode || state.Paused {
		t.Fatalf("watch Play left the driver at (replay=%v, paused=%v), want watching and running",
			state.ReplayMode, state.Paused)
	}

	clickPanel(p, lay.watch[1].Center())
	if state := p.driver.State(); !state.ReplayMode || !state.Paused {
		t.Fatalf("watch Pause left the driver at (replay=%v, paused=%v), want watching and paused",
			state.ReplayMode, state.Paused)
	}

	clickPanel(p, lay.take.Center())
	if state := p.driver.State(); state.ReplayMode {
		t.Fatal("Take control left the driver in watch mode")
	}
}

// Watch controls do nothing before there is a timeline to watch — the panel
// must not fake a mode over a run that was never recorded.
func TestWatchControlsInertWithoutARecordedTimeline(t *testing.T) {
	p := openPanel(t, Options{}, tabReplay)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())

	clickPanel(p, lay.watch[0].Center())

	if p.driver.State().ReplayMode {
		t.Fatal("entered watch mode with nothing recorded")
	}
}

func TestLoadWithoutAGameFactoryReportsIt(t *testing.T) {
	p := openPanel(t, Options{Recording: &fakePort{pending: []byte("x")}}, tabReplay)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())

	clickPanel(p, lay.record[2].Center())

	if p.rec.status == "" {
		t.Fatal("load without a factory said nothing")
	}
}

// The panel keeps finishing a deferred recording start while hidden: closing
// the panel is not a reason to strand it.
func TestHiddenPanelStillCompletesADeferredStart(t *testing.T) {
	port := &fakePort{}
	p := openPanel(t, Options{Recording: port}, tabReplay)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())

	clickPanel(p, lay.record[0].Center())
	p.SetVisible(false)
	p.Render(frameCamera(), 16*time.Millisecond, Stats{})

	if p.rec.pending {
		t.Fatal("a hidden panel left the recording waiting to start")
	}
}

// Watching a recording needs a way to pick where to watch from, not only
// play/pause/step from wherever it happens to sit.
func TestReplayScrubSeeksIntoTheRecording(t *testing.T) {
	port := &fakePort{}
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	port.pending = recordRun(t, p, port, 40)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())
	p.driver.AdvanceTime(0)

	// Drag the bar to the middle of the recording and let go there.
	target := lay.scrub.Center()
	p.HandleInput([]input.Event{moveTo(target), pressAt(target)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(target)}, testViewport())
	p.driver.AdvanceTime(0)

	tick := p.driver.State().Tick
	if tick == 0 || tick > 40 {
		t.Fatalf("scrub landed on tick %d, want somewhere inside the recorded 40", tick)
	}
}

// Repositioning is not the gesture that takes control: only Play and Step
// fork, and a scrub has to leave the watcher watching.
func TestReplayScrubStaysInWatchMode(t *testing.T) {
	port := &fakePort{}
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	port.pending = recordRun(t, p, port, 30)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())
	p.driver.AdvanceTime(0)

	if !p.driver.State().ReplayMode {
		t.Fatal("the loaded recording did not start in watch mode")
	}

	target := lay.scrub.Center()
	p.HandleInput([]input.Event{moveTo(target), pressAt(target)}, testViewport())
	p.HandleInput([]input.Event{releaseAt(target)}, testViewport())
	p.driver.AdvanceTime(0)

	state := p.driver.State()
	if !state.ReplayMode {
		t.Fatal("scrubbing dropped out of watch mode")
	}
	if !state.Paused {
		t.Fatal("scrubbing left the replay running")
	}
}

// Dragging must not seek on every position it passes through: each one would
// be a full reconstruction of the world.
func TestReplayScrubSeeksOnReleaseOnly(t *testing.T) {
	port := &fakePort{}
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	port.pending = recordRun(t, p, port, 30)
	lay := replayRects(p.frameLayout(testViewport()).body, p.rec.hasPicker())
	clickPanel(p, lay.record[2].Center())
	p.driver.AdvanceTime(0)

	start := lay.scrub.Min.Add(math2d.Vec2{X: lay.scrub.Width() * 0.8, Y: lay.scrub.Height() / 2})
	p.HandleInput([]input.Event{moveTo(start), pressAt(start)}, testViewport())
	p.driver.AdvanceTime(0)
	if got := p.driver.State().Tick; got != 0 {
		t.Fatalf("tick moved to %d mid-drag, want the seek held until release", got)
	}

	p.HandleInput([]input.Event{releaseAt(start)}, testViewport())
	p.driver.AdvanceTime(0)
	if p.driver.State().Tick == 0 {
		t.Fatal("releasing the bar did not seek")
	}
}

// listPort is a fakePort that can also enumerate, which is what a desktop
// host does and a browser cannot.
type listPort struct {
	fakePort
	files   map[string][]byte
	order   []string
	listErr error
}

func newListPort() *listPort {
	return &listPort{files: map[string][]byte{}}
}

func (p *listPort) Save(name string, data []byte) error {
	if err := p.fakePort.Save(name, data); err != nil {
		return err
	}
	stored := name + "-" + formatInt(len(p.order))
	p.files[stored] = append([]byte(nil), data...)
	p.order = append(p.order, stored)
	return nil
}

func (p *listPort) List() ([]RecordingEntry, error) {
	if p.listErr != nil {
		return nil, p.listErr
	}
	entries := make([]RecordingEntry, 0, len(p.order))
	for _, name := range p.order {
		entries = append(entries, RecordingEntry{Name: name, Label: "take " + name})
	}
	return entries, nil
}

func (p *listPort) LoadNamed(name string) ([]byte, error) {
	data, ok := p.files[name]
	if !ok {
		return nil, errors.New("no such recording")
	}
	return data, nil
}

// A host that can list what it saved gets a picker; the panel loads the entry
// chosen rather than guessing at the newest.
func TestLoadPicksTheSelectedRecording(t *testing.T) {
	port := newListPort()
	factory := func() runtime.Game { return testGame{} }
	p := openPanel(t, Options{Recording: port, NewGame: factory}, tabReplay)

	recordRun(t, p, &port.fakePort, 6)
	first := port.order[0]
	recordRun(t, p, &port.fakePort, 14)
	if len(port.order) != 2 {
		t.Fatalf("saved %d recordings, want 2", len(port.order))
	}

	p.rec.refresh()
	// Pick the older one; the newest is what a blind load would have taken.
	p.rec.selected = 0
	if p.rec.entries[0].Name != first {
		t.Fatalf("entry 0 is %q, want the first save %q", p.rec.entries[0].Name, first)
	}

	lay := replayRects(p.frameLayout(testViewport()).body, true)
	clickPanel(p, lay.record[2].Center())
	p.driver.AdvanceTime(0)

	if got := p.driver.State().MaxRecordedTick; got != 6 {
		t.Fatalf("loaded a recording of %d ticks, want the selected 6", got)
	}
}

// Saving puts a new file in the list the user is looking at, and moves the
// selection onto it — that is the one they just made.
func TestSaveRefreshesTheListAndSelectsTheNewEntry(t *testing.T) {
	port := newListPort()
	p := openPanel(t, Options{Recording: port, NewGame: func() runtime.Game { return testGame{} }}, tabReplay)

	recordRun(t, p, &port.fakePort, 5)
	if len(p.rec.entries) != 1 {
		t.Fatalf("list holds %d entries after one save, want 1", len(p.rec.entries))
	}

	recordRun(t, p, &port.fakePort, 5)
	if len(p.rec.entries) != 2 {
		t.Fatalf("list holds %d entries after two saves, want 2", len(p.rec.entries))
	}
	if p.rec.selected != 1 {
		t.Fatalf("selection is %d, want the recording just saved", p.rec.selected)
	}
}

// Entering the tab is when the list is read, not every frame: it is a
// filesystem call, and the files only change when this panel changes them.
func TestEnteringTheReplayTabRefreshesTheList(t *testing.T) {
	port := newListPort()
	p := openPanel(t, Options{Recording: port, NewGame: func() runtime.Game { return testGame{} }}, tabSim)

	port.order = append(port.order, "made-elsewhere")
	port.files["made-elsewhere"] = []byte("x")
	if len(p.rec.entries) != 0 {
		t.Fatal("the list was populated before the tab was ever opened")
	}

	lay := p.frameLayout(testViewport())
	clickPanel(p, lay.headers[tabReplay].Center())

	if len(p.rec.entries) != 1 {
		t.Fatalf("list holds %d entries after opening the tab, want 1", len(p.rec.entries))
	}
}

// An empty directory is a normal state, and Load has to say so rather than
// fall through to a picker this host does not have.
func TestLoadWithAnEmptyListReportsIt(t *testing.T) {
	port := newListPort()
	p := openPanel(t, Options{Recording: port, NewGame: func() runtime.Game { return testGame{} }}, tabReplay)
	before := p.driver

	lay := replayRects(p.frameLayout(testViewport()).body, true)
	clickPanel(p, lay.record[2].Center())

	if p.driver != before {
		t.Fatal("loading from an empty list swapped the driver")
	}
	if p.rec.status == "" {
		t.Fatal("loading from an empty list said nothing")
	}
}

// A host with no listing keeps the plain button, and the row it would have
// occupied is not reserved.
func TestBrowserStylePortKeepsThePlainLoadButton(t *testing.T) {
	p := openPanel(t, Options{Recording: &fakePort{}}, tabReplay)

	if p.rec.hasPicker() {
		t.Fatal("a port that cannot list offered a picker")
	}

	body := p.frameLayout(testViewport()).body
	withPicker := replayRects(body, true)
	without := replayRects(body, false)
	if without.watch[0].Min.Y >= withPicker.watch[0].Min.Y {
		t.Fatal("the picker row took up space on a host that has no picker")
	}
}
