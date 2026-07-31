package devpanel

import (
	"log/slog"

	"github.com/dhannyell/arasy/runtime"
)

// RecordingPort is the host-side file I/O for .arrp recordings — the one part
// of the panel that cannot be written once for every platform, because a
// browser has no filesystem and a desktop has no download.
//
// Load may answer asynchronously: a host that has to open a file picker
// returns ok=false with a nil error and calls Panel.ApplyRecording once the
// bytes arrive. ok=false with a nil error therefore means "nothing right
// now", not "failure".
type RecordingPort interface {
	Save(name string, data []byte) error
	Load() (data []byte, name string, ok bool, err error)
}

// RecordingEntry is one saved recording as the panel lists it. Name is what
// LoadNamed takes back; Label is what a person reads, which is rarely the
// same string — a filename carries a prefix and an extension that are noise
// once you are looking at a list of nothing but recordings.
type RecordingEntry struct {
	Name  string
	Label string
}

// RecordingLister is the optional half of RecordingPort, for hosts that can
// enumerate what they have saved. A browser cannot: it has no directory to
// read, only a file picker, and the picker already is the list. A host that
// does not implement this keeps the plain Load button.
type RecordingLister interface {
	List() ([]RecordingEntry, error)
	LoadNamed(name string) (data []byte, err error)
}

// recordingFileName is what a saved recording is called. Hosts that can pick
// a name (a desktop writing into a directory) may decorate it.
const recordingFileName = "arasy-replay.arrp"

// recorder drives the driver's recorder and remembers enough to make the
// Record → Stop → Save flow work.
type recorder struct {
	port RecordingPort
	log  *slog.Logger

	// recording mirrors the driver so the label can be drawn without asking.
	recording bool
	// pending defers StartRecording until the reset to tick 0 lands. Replay
	// always reconstructs from tick 0, so a recording started mid-run would
	// capture only the tail and diverge on playback.
	pending bool
	// stopped holds the last finished recording, so stopping and then saving
	// does not throw the capture away.
	stopped *runtime.Recording

	status string

	// entries is the last listing of what the host has saved, and selected
	// the row the user picked. The list is refreshed on entering the tab and
	// after a save rather than every frame: it is a filesystem read, and the
	// set of files only changes when this panel changes it.
	entries  []RecordingEntry
	selected int
}

func (r *recorder) active() bool { return r.recording }

// lister returns the port's listing half, if it has one.
func (r *recorder) lister() (RecordingLister, bool) {
	lister, ok := r.port.(RecordingLister)
	return lister, ok
}

// refresh re-reads what the host has saved, keeping the selection on the same
// recording where it can. A new save lands at the end of the list, so the
// selection follows it there — that is the one the user just made.
func (r *recorder) refresh() {
	lister, ok := r.lister()
	if !ok {
		return
	}
	entries, err := lister.List()
	if err != nil {
		r.log.Error("list recordings failed", "err", err)
		r.status = "list failed: " + err.Error()
		return
	}
	r.entries = entries
	r.selected = min(max(r.selected, 0), max(len(entries)-1, 0))
}

// hasPicker reports whether this host can offer a list to choose from.
func (r *recorder) hasPicker() bool {
	_, ok := r.lister()
	return ok
}

// picked returns the selected recording, if there is a list and something in
// it to select.
func (r *recorder) picked() (RecordingEntry, RecordingLister, bool) {
	lister, ok := r.lister()
	if !ok || r.selected < 0 || r.selected >= len(r.entries) {
		return RecordingEntry{}, nil, false
	}
	return r.entries[r.selected], lister, true
}

// labels is what the picker shows. An empty list still needs a row, or the
// widget would collapse into something unclickable with nothing to explain
// why.
func (r *recorder) labels() []string {
	if len(r.entries) == 0 {
		return []string{"no recordings"}
	}
	labels := make([]string, len(r.entries))
	for i, entry := range r.entries {
		labels[i] = entry.Label
	}
	return labels
}

// tick completes a deferred start once the reset has taken effect.
func (r *recorder) tick(driver *runtime.Driver, state runtime.DriveState) {
	if !r.pending || state.Tick != 0 {
		return
	}
	driver.StartRecording()
	r.pending = false
	r.status = "recording from tick 0"
}

func (r *recorder) toggle(driver *runtime.Driver) {
	if r.recording {
		if r.pending {
			// The reset was queued but the recording never started; there is
			// nothing to stop.
			r.pending = false
			r.stopped = nil
		} else {
			stopped := driver.StopRecording()
			r.stopped = &stopped
		}
		r.recording = false
		r.status = "recording stopped; ready to save"
		return
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReset})
	r.pending = true
	r.recording = true
	r.stopped = nil
	r.status = "resetting to tick 0..."
}

func (r *recorder) save(driver *runtime.Driver) {
	if r.port == nil {
		r.status = "save unavailable on this host"
		return
	}
	if r.pending {
		r.status = "recording hasn't started yet - wait for the reset"
		return
	}
	if r.recording {
		stopped := driver.StopRecording()
		r.recording = false
		r.stopped = &stopped
	}
	if r.stopped == nil {
		r.status = "nothing to save - no recording"
		return
	}

	data, err := r.stopped.MarshalBinary()
	if err != nil {
		r.log.Error("marshal recording failed", "err", err)
		r.status = "save failed: " + err.Error()
		return
	}
	if err := r.port.Save(recordingFileName, data); err != nil {
		r.log.Error("save recording failed", "err", err)
		r.status = "save failed: " + err.Error()
		return
	}
	r.status = "saved " + recordingFileName
	// The list the user is looking at is now one entry out of date.
	r.refresh()
	r.selected = max(len(r.entries)-1, 0)
}

// clear forgets the in-memory recording state, which is what adopting a
// replayed driver has to do: the capture belonged to the driver being
// replaced.
func (r *recorder) clear() {
	r.recording = false
	r.pending = false
	r.stopped = nil
}
