package devpanel

import (
	"errors"
	"testing"
	"time"

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
	lay := replayRects(p.frameLayout(testViewport()).body)

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
	lay := replayRects(p.frameLayout(testViewport()).body)

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

	lay := replayRects(p.frameLayout(testViewport()).body)
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

	lay := replayRects(p.frameLayout(testViewport()).body)
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

	lay := replayRects(p.frameLayout(testViewport()).body)
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
	lay := replayRects(p.frameLayout(testViewport()).body)
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
	lay := replayRects(p.frameLayout(testViewport()).body)

	clickPanel(p, lay.watch[0].Center())

	if p.driver.State().ReplayMode {
		t.Fatal("entered watch mode with nothing recorded")
	}
}

func TestLoadWithoutAGameFactoryReportsIt(t *testing.T) {
	p := openPanel(t, Options{Recording: &fakePort{pending: []byte("x")}}, tabReplay)
	lay := replayRects(p.frameLayout(testViewport()).body)

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
	lay := replayRects(p.frameLayout(testViewport()).body)

	clickPanel(p, lay.record[0].Center())
	p.SetVisible(false)
	p.Render(frameCamera(), 16*time.Millisecond, Stats{})

	if p.rec.pending {
		t.Fatal("a hidden panel left the recording waiting to start")
	}
}
