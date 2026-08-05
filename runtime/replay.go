package runtime

import (
	"bytes"
	"log/slog"
	"math"
	goruntime "runtime"
	"runtime/debug"
	"strings"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// Recording is the input log and tick count needed for Replay.
type Recording struct {
	Events []input.Event
	Ticks  uint64
	Seed   uint64
	// SimulationHz is the fixed simulation rate the recording was captured
	// at. Replay steps at this rate — the recording is authoritative, like
	// Seed — so a recording reproduces regardless of the game's current
	// SimulationHz. Zero (recordings constructed in memory) falls back to
	// the fresh game's rate.
	SimulationHz uint16
	// GamepadDeadzone is the effective deterministic stick-axis deadzone.
	// Zero, or an invalid value in a Recording constructed in memory, means
	// use the fresh game's configured effective deadzone.
	GamepadDeadzone float64
	Anchors         []Anchor

	// ContentChecksum fingerprints the authored visuals (sprite stores) at
	// StartRecording. Anchors cover simulation state only, so this is what
	// lets a load attribute a divergence to a content edit -- a warning, not
	// an anchor failure. Zero means "not captured" (recordings constructed in
	// memory) and skips the check.
	ContentChecksum uint64

	// Viewport is the surface size active at tick 0 of the recording. A zero
	// value (Width/Height <= 0) means "use the game's Config default", which is
	// the behavior of recordings made before viewport capture existed. Replay
	// applies it before the first tick so viewport-dependent logic (e.g. a UI
	// button hit-test) matches the live run — without it, pointer coordinates
	// captured in a different surface size map to the wrong reference-canvas
	// position and a click that started the game live is missed during replay.
	Viewport math2d.Rect

	// Viewports records SetViewport calls during the recording, each stamped
	// with the tick it was applied. Replay applies them in tick order before
	// the input events of the same tick, mirroring the live
	// SetViewport → ApplyInput → Step order. Without this, pointer coordinates
	// captured in a different surface size map to the wrong reference-canvas
	// position and UI clicks that started the game live are missed during
	// replay.
	Viewports []ViewportChange

	// Provenance identifies what produced the recording, so a failed
	// verification can say what changed — the game, the engine, or the
	// platform — instead of only where it diverged.
	Provenance Provenance
}

// Provenance identifies what produced a recording. Ownership is layered like
// the save format: the engine fills EngineVersion and Platform, the game
// fills the rest through Config. It is diagnostic metadata only — it never
// enters a checksum, never feeds simulation, and never changes what a replay
// reproduces. Metadata explains; it does not decide.
type Provenance struct {
	// Present reports whether the recording carries a provenance section.
	// Absence is valid and means "recorded without provenance" — the state
	// of every test fixture, so recorded bytes stay machine-independent.
	Present bool
	// EngineVersion is the arasy module version compiled into the binary
	// that captured the recording, or "devel" inside the engine repo itself.
	EngineVersion string
	// GameID and GameVersion are copied verbatim from Config.
	GameID      string
	GameVersion string
	// Platform is the GOOS/GOARCH pair the recording was captured on.
	Platform string
	// GameBlob is an opaque payload the game owns through
	// Config.GameMetadata. The engine stores and returns it without
	// interpreting it.
	GameBlob []byte
}

// note is the attribution line diagnostics append to a failed verification.
// Absence is stated, never omitted: "recorded without provenance" tells the
// developer the file itself cannot say what changed.
func (p Provenance) note() string {
	if !p.Present {
		return "recorded without provenance"
	}
	game := strings.TrimSpace(p.GameID + " " + p.GameVersion)
	return "recorded with: game " + game + ", engine " + p.EngineVersion + ", " + p.Platform
}

// ViewportChange is one SetViewport call captured in a Recording, stamped with
// the tick it was applied.
type ViewportChange struct {
	Tick     uint64
	Viewport math2d.Rect
}

// Anchor pins the simulation checksum at one tick of a recording. It hashes
// simulation state only (scene.World.SimChecksum): presentation edits must
// not break replay verification.
type Anchor struct {
	Tick     uint64
	Checksum uint64
}

// StartRecording begins capturing every event passed to ApplyInput.
// Start at tick 0; Replay always reconstructs from the initial state.
func (d *Driver) StartRecording() {
	if d.tick != 0 {
		d.log.Warn("recording started mid-run; Replay re-runs from tick 0 and will not reproduce it", "tick", d.tick)
	}
	d.recordingEnabled = true
	d.recording = nil
	d.recordingStaged = nil
	d.recordingAnchors = nil
	// Capture the viewport active at tick 0 so Replay can restore it; without
	// it, a platform surface different from the Config default (e.g. a
	// responsive browser canvas) shifts pointer coordinates and breaks UI
	// hit-tests during replay.
	d.recordingInitialViewport = d.viewport
	d.recordingContentChecksum = d.world.ContentChecksum()
}

// StopRecording returns the captured input log and final tick.
func (d *Driver) StopRecording() Recording {
	d.recordingEnabled = false
	hz := d.cfg.SimulationHz
	if hz > math.MaxUint16 {
		d.log.Warn("SimulationHz exceeds the recording field range; recording it as zero",
			"hz", hz, "max", math.MaxUint16)
		hz = 0
	}
	events := d.recording
	anchors := d.recordingAnchors
	initial := d.recordingInitialViewport
	viewports := viewportLogToChanges(d.timeline.viewports, d.tick)
	d.recording = nil
	d.recordingStaged = nil
	d.recordingAnchors = nil
	d.recordingInitialViewport = math2d.Rect{}
	content := d.recordingContentChecksum
	d.recordingContentChecksum = 0
	return Recording{
		Events:          events,
		Ticks:           d.tick,
		Seed:            d.cfg.Seed,
		SimulationHz:    uint16(hz),
		GamepadDeadzone: d.cfg.GamepadDeadzone,
		Anchors:         anchors,
		Viewport:        initial,
		Viewports:       viewports,
		ContentChecksum: content,
		Provenance:      d.recordingProvenance(),
	}
}

// recordingProvenance builds the provenance section for a recording the
// driver is about to emit. The game opts in through Config; fixtures and
// tests leave the fields empty, record no provenance, and keep their bytes
// machine-independent.
func (d *Driver) recordingProvenance() Provenance {
	if d.cfg.GameID == "" && d.cfg.GameVersion == "" {
		return Provenance{}
	}
	blob := d.cfg.GameMetadata
	if len(blob) > maxProvenanceBlob {
		// Losing the blob is acceptable; losing the recording over metadata
		// is not. Kept, the oversized blob would make WriteTo reject the
		// whole file at save time.
		d.log.Warn("GameMetadata exceeds the provenance blob limit; recording without it",
			"size", len(blob), "max", maxProvenanceBlob)
		blob = nil
	}
	return Provenance{
		Present:       true,
		EngineVersion: engineVersion(),
		GameID:        d.cfg.GameID,
		GameVersion:   d.cfg.GameVersion,
		Platform:      hostPlatform(),
		// Cloned so a game mutating its metadata buffer after StopRecording
		// cannot silently rewrite an already-emitted recording.
		GameBlob: bytes.Clone(blob),
	}
}

// engineVersion reports the arasy module version compiled into the binary,
// or "devel" when the binary is the engine repo itself (tests, demos).
func engineVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/dhannyell/arasy" {
				return dep.Version
			}
		}
	}
	return "devel"
}

// hostPlatform is the GOOS/GOARCH pair provenance records for attribution.
func hostPlatform() string {
	return goruntime.GOOS + "/" + goruntime.GOARCH
}

// viewportLogToChanges copies the driver's viewport change log into the
// exportable Recording form, dropping entries at or beyond the recording's
// final tick (they belong to ticks Replay never reaches).
func viewportLogToChanges(log []viewportEntry, ticks uint64) []ViewportChange {
	var changes []ViewportChange
	for _, ve := range log {
		if ve.tick >= ticks {
			continue
		}
		changes = append(changes, ViewportChange{Tick: ve.tick, Viewport: ve.viewport})
	}
	return changes
}

// viewportChangesToLog converts the exportable Recording viewport changes back
// into the driver-internal viewportEntry form used by replayWithViewport.
func viewportChangesToLog(changes []ViewportChange) []viewportEntry {
	log := make([]viewportEntry, len(changes))
	for i, c := range changes {
		log[i] = viewportEntry{tick: c.Tick, viewport: c.Viewport}
	}
	return log
}

// warnOnContentDivergence compares the recording's content fingerprint with
// the fresh game's authored visuals. A difference is attribution, not
// failure: the simulation anchors still verify, the visuals changed since
// the recording was made. Zero means the recording never captured one.
func warnOnContentDivergence(d *Driver, log *slog.Logger, rec Recording) {
	if rec.ContentChecksum == 0 || log == nil {
		return
	}
	if got := d.world.ContentChecksum(); got != rec.ContentChecksum {
		log.Warn("recording content differs from the game's authored visuals; simulation anchors still verify",
			"recorded", rec.ContentChecksum, "current", got)
	}
}

// Replay reconstructs a recording on a fresh Driver, returning it at the
// final tick with no keyframes retained. Use it when only the final state
// matters (verification, tests); use ReplayForViewing when the driver will
// be adopted by an interactive host for scrubbing and playback.
// It mutates the game instance it receives; pass a fresh game unless the
// caller intentionally wants replay to adopt the reconstructed state.
func Replay(game Game, log *slog.Logger, rec Recording) *Driver {
	return replayWithViewport(game, log, rec, rec.Viewport, viewportChangesToLog(rec.Viewports), false)
}

// ReplayForViewing reconstructs a recording like Replay, but keeps the
// keyframe recorder populated during reconstruction and returns the driver
// paused at the final tick in watch mode. Use it when the reconstructed
// driver will be adopted by an interactive host for scrubbing and playback;
// use Replay when only the final state matters (verification, tests).
func ReplayForViewing(game Game, log *slog.Logger, rec Recording) *Driver {
	driver := replayWithViewport(game, log, rec, rec.Viewport, viewportChangesToLog(rec.Viewports), true)
	driver.controls.paused = true
	driver.controls.replayMode = true
	return driver
}

// replayWithViewport also replays viewport changes for UI hit-testing.
// Each viewport entry is applied before input events at the same tick. With
// keepRecorder, the keyframe recorder stays populated during reconstruction
// instead of being dropped, at the cost of a world clone per keyframe.
func replayWithViewport(game Game, log *slog.Logger, rec Recording, initialViewport math2d.Rect, viewportLog []viewportEntry, keepRecorder bool) *Driver {
	driver := newDriverFromRecording(game, log, rec)
	warnOnContentDivergence(driver, log, rec)

	if !keepRecorder {
		// The final state is all that's needed; snapshot capture would clone
		// the whole World every tick for nothing.
		driver.recorder = nil
	}
	// Cues at these ticks were already heard on the live run.
	driver.audio.suppressCues = true

	viewportByTick := make(map[uint64]math2d.Rect)
	for _, ve := range viewportLog {
		if ve.tick < rec.Ticks {
			viewportByTick[ve.tick] = ve.viewport
		}
	}

	// Tick 0 entries override this initial viewport below.
	if initialViewport.Width() > 0 && initialViewport.Height() > 0 {
		driver.viewport = initialViewport
	}

	byTick := make(map[uint64][]input.Event, len(rec.Events))
	for _, event := range rec.Events {
		byTick[event.Tick] = append(byTick[event.Tick], event)
	}

	for tick := uint64(0); tick < rec.Ticks; tick++ {
		// Match live order: SetViewport, ApplyInput, Step.
		if vp, ok := viewportByTick[tick]; ok {
			driver.viewport = vp
		}
		if events, ok := byTick[tick]; ok {
			driver.ApplyInput(events)
		}
		driver.Step()
	}

	// Reconstruction is over: a driver handed to a caller (or adopted by an
	// interactive host) must be able to emit cues again, otherwise it stays
	// mute for the rest of its life.
	driver.audio.suppressCues = false

	return driver
}
