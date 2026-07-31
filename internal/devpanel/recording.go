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
}

func (r *recorder) active() bool { return r.recording }

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
}

// clear forgets the in-memory recording state, which is what adopting a
// replayed driver has to do: the capture belonged to the driver being
// replaced.
func (r *recorder) clear() {
	r.recording = false
	r.pending = false
	r.stopped = nil
}
