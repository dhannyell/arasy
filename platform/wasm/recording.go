//go:build js && wasm

package wasm

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/arasy/runtime"
)

// recordingPanel wires the .arrp recording/replay controls embedded in the
// unified debug panel. It exposes:
//
//   - Record / Stop: toggle driver.StartRecording/StopRecording.
//   - Save: download the stopped recording as a .arrp file via a Blob URL.
//   - Load: pick a .arrp file, read its bytes, VerifyRecording + Replay, and
//     swap the host driver onto the reconstructed one.
//   - a status line synced each frame from DriveState.
//
// The panel drives the engine through the same public API the canvas demo
// uses (StartRecording/StopRecording/VerifyRecording/Replay/MarshalBinary/
// ReadRecordingFromBytes) — zero engine code is aware of the browser.
type recordingPanel struct {
	driver *runtime.Driver
	// newGame builds a fresh game instance for Verify/Replay. Replay mutates
	// its game argument, so each call needs a fresh one; the host owns the
	// factory (the app passes it via Options.GameFactory) rather than the
	// engine having to clone arbitrary user types.
	newGame func() runtime.Game
	log     *slog.Logger

	// recording mirrors driver state so sync() can render an accurate label.
	recording bool
	// stopped holds the last completed recording so the natural Record -> Stop
	// -> Save flow does not discard the captured log.
	stopped *runtime.Recording

	// pendingRecord defers StartRecording until the reset-to-tick-0 completes.
	// Replay always reconstructs from tick 0, so a recording started mid-run
	// would capture only the tail and diverge on replay. When the user clicks
	// Record, the panel queues ControlReset; on the next frame (tick == 0),
	// sync() calls StartRecording — no "mid-run" warning, clean recording.
	pendingRecord bool

	// host points back at the running host so adoptDriver can swap its driver
	// after a successful Load (the rAF loop reads h.driver each frame).
	host *host

	statusEl js.Value
	recordEl js.Value
	fileEl   js.Value

	// callbacks are retained so the Go GC does not release the js.Func while
	// the DOM still references them (dead handlers silently stop firing).
	cbs []js.Func
}

// newRecordingPanel wires the recording controls inside the unified debug panel.
func newRecordingPanel(h *host, newGame func() runtime.Game, log *slog.Logger) *recordingPanel {
	p := &recordingPanel{driver: h.driver, newGame: newGame, log: log, host: h}
	doc := js.Global().Get("document")

	p.statusEl = doc.Call("getElementById", "ar-rec-status")
	p.recordEl = doc.Call("getElementById", "ar-rec-record")
	saveEl := doc.Call("getElementById", "ar-rec-save")
	loadEl := doc.Call("getElementById", "ar-rec-load")
	p.fileEl = doc.Call("getElementById", "ar-rec-file")

	p.retainClick(p.recordEl, p.toggleRecording)
	p.retainClick(saveEl, p.saveRecording)
	p.retainClick(loadEl, func() { p.fileEl.Call("click") })
	p.retainChange(p.fileEl, p.loadRecording)

	return p
}

func (p *recordingPanel) retainClick(el js.Value, fn func()) {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn()
		return nil
	})
	p.cbs = append(p.cbs, cb)
	el.Call("addEventListener", "click", cb)
}

func (p *recordingPanel) retainChange(el js.Value, fn func(file js.Value)) {
	cb := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			fn(args[0].Get("target"))
		}
		return nil
	})
	p.cbs = append(p.cbs, cb)
	el.Call("addEventListener", "change", cb)
}

// toggleRecording flips between StartRecording and StopRecording.
// Starting a recording mid-run would capture only the tail (Replay always
// reconstructs from tick 0), so the panel resets to tick 0 first and defers
// StartRecording to sync() when the reset completes.
func (p *recordingPanel) toggleRecording() {
	if p.recording {
		if p.pendingRecord {
			// Reset was pending but recording hadn't started yet; just cancel.
			p.pendingRecord = false
			p.stopped = nil
		} else {
			rec := p.driver.StopRecording()
			p.stopped = &rec
		}
		p.recording = false
		p.recordEl.Set("textContent", "Record")
		p.setStatus("recording stopped; ready to save")
		return
	}
	// Queue a reset; StartRecording is deferred to sync() at tick 0 so the
	// recording starts from the initial state (no "mid-run" warning).
	p.driver.Control(runtime.ControlCommand{Kind: runtime.ControlReset})
	p.pendingRecord = true
	p.recording = true
	p.stopped = nil
	p.recordEl.Set("textContent", "Stop")
	p.setStatus("resetting to tick 0…")
}

// saveRecording serializes the active recording and triggers a browser
// download of the .arrp bytes via a Blob URL.
func (p *recordingPanel) saveRecording() {
	if p.pendingRecord {
		p.setStatus("recording hasn't started yet — wait for reset")
		return
	}
	rec := p.stopped
	if p.recording {
		stopped := p.driver.StopRecording()
		p.recording = false
		p.recordEl.Set("textContent", "Record")
		p.stopped = &stopped
		rec = p.stopped
	}
	if rec == nil {
		p.setStatus("nothing to save — no recording")
		return
	}

	data, err := rec.MarshalBinary()
	if err != nil {
		p.log.Error("marshal recording failed", "err", err)
		p.setStatus("save failed: " + err.Error())
		return
	}

	doc := js.Global().Get("document")
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	blob := js.Global().Get("Blob").New(
		[]any{array},
		map[string]any{"type": "application/octet-stream"},
	)
	url := js.Global().Get("URL").Call("createObjectURL", blob)
	a := doc.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", "arasy-replay.arrp")
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	doc.Get("body").Call("removeChild", a)
	js.Global().Get("URL").Call("revokeObjectURL", url)
	p.setStatus("saved arasy-replay.arrp")
}

// loadRecording reads the picked .arrp file, verifies it, and swaps the host
// onto the reconstructed driver so the browser keeps rendering the replay.
func (p *recordingPanel) loadRecording(fileEl js.Value) {
	files := fileEl.Get("files")
	if files.Length() == 0 {
		return
	}
	file := files.Index(0)
	name := file.Get("name").String()
	arrayBuffer := file.Call("arrayBuffer")
	p.readArrayBuffer(arrayBuffer, name)
}

// readArrayBuffer awaits File.arrayBuffer() then verifies+replays.
func (p *recordingPanel) readArrayBuffer(promise js.Value, name string) {
	onFulfilled := js.FuncOf(func(this js.Value, args []js.Value) any {
		uint8 := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, uint8.Length())
		js.CopyBytesToGo(data, uint8)
		p.applyRecording(data, name)
		return nil
	})
	onRejected := js.FuncOf(func(this js.Value, args []js.Value) any {
		p.log.Error("read replay file failed", "name", name, "err", args[0].String())
		p.setStatus("load failed: " + args[0].String())
		return nil
	})
	p.cbs = append(p.cbs, onFulfilled, onRejected)
	promise.Call("then", onFulfilled, onRejected)
}

// applyRecording parses, verifies, and replays the bytes, then swaps the host.
func (p *recordingPanel) applyRecording(data []byte, name string) {
	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		p.log.Error("read recording failed", "name", name, "err", err)
		p.setStatus("load failed: " + err.Error())
		return
	}
	if err := runtime.VerifyRecording(p.newGame(), p.log, rec); err != nil {
		p.log.Error("verify recording failed", "name", name, "err", err)
		p.setStatus("verify failed: " + err.Error())
		return
	}
	replayed := runtime.Replay(p.newGame(), p.log, rec)
	p.adoptDriver(replayed)
	p.pendingRecord = false
	p.recording = false
	p.stopped = nil
	p.recordEl.Set("textContent", "Record")
	p.setStatus("replay loaded: " + name + " (ticks=" + itoa(int(rec.Ticks)) + ")")
}

// adoptDriver swaps the panel and host onto a replayed driver (after Load).
func (p *recordingPanel) adoptDriver(d *runtime.Driver) {
	p.driver = d
	if p.host != nil {
		p.host.driver = d
		p.host.resize()
	}
}

func (p *recordingPanel) setStatus(s string) {
	if !p.statusEl.IsNull() {
		p.statusEl.Set("textContent", s)
	}
}

// sync updates the status line from the live DriveState each frame.
// It also completes a pending recording start: after the user clicks Record,
// the panel queues a reset; when the reset completes (tick == 0), sync calls
// StartRecording so the recording begins from the initial state.
func (p *recordingPanel) sync(state runtime.DriveState) {
	if p.pendingRecord && state.Tick == 0 {
		p.driver.StartRecording()
		p.pendingRecord = false
		p.setStatus("recording from tick 0")
		return
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
