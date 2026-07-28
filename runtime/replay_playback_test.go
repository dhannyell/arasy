package runtime_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// playbackGame moves its single entity along X while a bound key is held, so
// tests can produce input-dependent, per-tick checksum divergence.
type playbackGame struct {
	player scene.EntityID
}

func (g *playbackGame) Setup() runtime.Config {
	return runtime.Config{
		InputBindings: []input.Binding{
			{
				Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
				Action: input.ActionMoveX,
				Scale:  1,
			},
		},
		HistoryLen: 4,
	}
}

func (g *playbackGame) Init(ctx *runtime.UpdateContext) {
	g.player = ctx.World.Spawn()
}

func (g *playbackGame) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.World.SetVelocity(g.player, scene.Velocity{
		Linear: math2d.Vec2{X: ctx.Input.Value(input.ActionMoveX) * 600},
	})
}

func (g *playbackGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

// replayViewingState is the value snapshot for replayViewingGame, embedded
// via runtime.StateSlot so the game opts into keyframe restore.
type replayViewingState struct {
	Player  scene.EntityID
	Updates uint64
}

// replayViewingGame is a StatefulGame with deterministic, input-independent
// movement, used to check that ReplayForViewing keeps the keyframe recorder
// populated during reconstruction. initCalls counts Game.Init invocations,
// the same proxy driver_rewind/scrub tests use to prove a seek restored a
// keyframe instead of resetting to tick 0.
type replayViewingGame struct {
	runtime.StateSlot[replayViewingState]
	initCalls int
}

func (g *replayViewingGame) Setup() runtime.Config {
	return runtime.Config{HistoryLen: 8, KeyframeInterval: 2}
}

func (g *replayViewingGame) Init(ctx *runtime.UpdateContext) {
	g.initCalls++
	g.State.Player = ctx.World.Spawn()
	ctx.World.SetTransform(g.State.Player, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
}

func (g *replayViewingGame) Update(ctx *runtime.UpdateContext, dt float64) {
	g.State.Updates++
	ctx.World.SetTransform(g.State.Player, scene.Transform{
		Position: math2d.Vec2{X: float64(g.State.Updates)},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
}

func (g *replayViewingGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

// replayControlGame is a minimal game used to exercise the watch-mode
// control matrix without any behavior of its own.
type replayControlGame struct{}

func (replayControlGame) Setup() runtime.Config                          { return runtime.Config{} }
func (replayControlGame) Init(*runtime.UpdateContext)                    {}
func (replayControlGame) Update(*runtime.UpdateContext, float64)         {}
func (replayControlGame) Render(*runtime.RenderContext) []render.Command { return nil }

func newReplayControlDriver() *runtime.Driver {
	return runtime.NewDriver(replayControlGame{}, slog.New(slog.DiscardHandler))
}

// TestReplayPlayEntersWatchModeAndUnpauses covers ControlReplayPlay entering
// watch mode from outside it.
func TestReplayPlayEntersWatchModeAndUnpauses(t *testing.T) {
	driver := newReplayControlDriver()
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	state := driver.State()
	if !state.ReplayMode {
		t.Fatal("expected ReplayMode true after ControlReplayPlay")
	}
	if state.Paused {
		t.Fatal("expected Paused false after ControlReplayPlay")
	}
}

// TestControlPlayExitsWatchMode covers the explicit "take control" gesture:
// ControlPlay while watching leaves watch mode and resumes live play.
func TestControlPlayExitsWatchMode(t *testing.T) {
	driver := newReplayControlDriver()
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlPlay})

	state := driver.State()
	if state.ReplayMode {
		t.Fatal("expected ControlPlay to exit watch mode")
	}
	if state.Paused {
		t.Fatal("expected Paused false after ControlPlay")
	}
}

// TestControlStepExitsWatchMode: manual stepping is also an explicit
// "take control" gesture, same as ControlPlay.
func TestControlStepExitsWatchMode(t *testing.T) {
	driver := newReplayControlDriver()
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlStep})

	state := driver.State()
	if state.ReplayMode {
		t.Fatal("expected ControlStep to exit watch mode")
	}
	if !state.Paused {
		t.Fatal("expected Paused true after ControlStep")
	}
}

// TestScrubDuringWatchModeStaysInModeAndPauses: repositioning is not taking
// control, so ControlScrubToTick and ControlBack must leave watch mode
// untouched, only pausing playback.
func TestScrubDuringWatchModeStaysInModeAndPauses(t *testing.T) {
	tests := []struct {
		name string
		cmd  runtime.ControlCommand
	}{
		{"ScrubToTick", runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0}},
		{"Back", runtime.ControlCommand{Kind: runtime.ControlBack}},
		{"BackToTick", runtime.ControlCommand{Kind: runtime.ControlBackToTick, Tick: 0}},
		{"BackSeconds", runtime.ControlCommand{Kind: runtime.ControlBackSeconds, Seconds: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := newReplayControlDriver()
			driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
			driver.Control(tt.cmd)

			state := driver.State()
			if !state.ReplayMode {
				t.Fatalf("expected %s to keep watch mode", tt.name)
			}
			if !state.Paused {
				t.Fatalf("expected %s to pause playback", tt.name)
			}
		})
	}
}

// TestControlPauseIsEquivalentToReplayPauseInsideWatchMode: pausing is
// unambiguous, so the plain pause command behaves like the dedicated one
// while watching.
func TestControlPauseIsEquivalentToReplayPauseInsideWatchMode(t *testing.T) {
	for _, kind := range []runtime.ControlKind{runtime.ControlPause, runtime.ControlReplayPause} {
		driver := newReplayControlDriver()
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
		driver.Control(runtime.ControlCommand{Kind: kind})

		state := driver.State()
		if !state.ReplayMode {
			t.Fatalf("kind %v: expected watch mode to remain on", kind)
		}
		if !state.Paused {
			t.Fatalf("kind %v: expected playback to pause", kind)
		}
	}
}

// TestControlResetExitsWatchModeAndResets covers the reset command leaving
// watch mode along with the rest of the driver state.
func TestControlResetExitsWatchModeAndResets(t *testing.T) {
	driver := newReplayControlDriver()
	driver.Step()
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReset})

	if driver.State().ReplayMode {
		t.Fatal("expected ControlReset to exit watch mode")
	}

	driver.AdvanceTime(0) // pendingReset only applies on the next advance
	if driver.State().Tick != 0 {
		t.Fatalf("tick after reset = %d, want 0", driver.State().Tick)
	}
}

// TestReplayStepEntersWatchModeAndPauses covers ControlReplayStep entering
// watch mode from outside it, mirroring ControlReplayPlay, but pausing
// immediately like ControlStep instead of running freely.
func TestReplayStepEntersWatchModeAndPauses(t *testing.T) {
	driver := newReplayControlDriver()
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayStep})

	state := driver.State()
	if !state.ReplayMode {
		t.Fatal("expected ControlReplayStep to enter watch mode")
	}
	if !state.Paused {
		t.Fatal("expected ControlReplayStep to pause")
	}
}

// TestPlaybackReproducesLiveChecksumsWithoutMutatingJournal watches a
// recorded run from tick 0 and checks that every tick's checksum matches the
// live run exactly, and that the journal's reachable range is untouched.
func TestPlaybackReproducesLiveChecksumsWithoutMutatingJournal(t *testing.T) {
	game := &playbackGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	const runTicks = 30
	checksums := make([]uint64, 0, runTicks+1)
	checksums = append(checksums, driver.Checksum())
	for tick := range runTicks {
		kind := input.EventButtonUp
		if tick%10 < 5 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}
	wantMaxTick := driver.State().MaxRecordedTick

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	for tick := 1; tick <= runTicks; tick++ {
		driver.Step()
		if got := driver.State().Tick; got != uint64(tick) {
			t.Fatalf("playback tick = %d, want %d", got, tick)
		}
		if got, want := driver.Checksum(), checksums[tick]; got != want {
			t.Fatalf("playback checksum at tick %d = %d, want %d", tick, got, want)
		}
	}

	if got := driver.State().MaxRecordedTick; got != wantMaxTick {
		t.Fatalf("MaxRecordedTick after playback = %d, want unchanged %d", got, wantMaxTick)
	}
}

// TestPlaybackDiscardsLiveInputWithoutForking applies live input mid-playback
// (staged, like during a seek) and checks it never reaches the journal: the
// checksums produced while watching still match the live run, and scrubbing
// back afterward still lands on the exact original state.
func TestPlaybackDiscardsLiveInputWithoutForking(t *testing.T) {
	game := &playbackGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	const runTicks = 20
	checksums := make([]uint64, 0, runTicks+1)
	checksums = append(checksums, driver.Checksum())
	for tick := range runTicks {
		kind := input.EventButtonUp
		if tick < 10 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}
	wantMaxTick := driver.State().MaxRecordedTick

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	// KeyA carries no binding in this game, so it cannot perturb the
	// checksum either — it only exercises the staging/discard path.
	for tick := 1; tick <= runTicks; tick++ {
		driver.Step()
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyA}, Kind: input.EventButtonDown},
		})
		if got, want := driver.Checksum(), checksums[tick]; got != want {
			t.Fatalf("playback checksum at tick %d = %d, want %d", tick, got, want)
		}
	}

	if got := driver.State().MaxRecordedTick; got != wantMaxTick {
		t.Fatalf("MaxRecordedTick after playback = %d, want unchanged %d", got, wantMaxTick)
	}

	mid := uint64(runTicks / 2)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: mid})
	driver.AdvanceTime(0)
	if got, want := driver.Checksum(), checksums[mid]; got != want {
		t.Fatalf("checksum after post-playback scrub to %d = %d, want %d", mid, got, want)
	}
}

// TestPlaybackEmitsCuesWhileScrubStaysSilent contrasts the two non-destructive
// movements over the same recorded range: scrubbing stays silent (the cue was
// already heard live), watching delivers the cue.
func TestPlaybackEmitsCuesWhileScrubStaysSilent(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	for range 10 {
		driver.Step()
	}
	driver.ConsumeAudio()

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.ConsumeAudio()

	// Scrubbing forward over the tick-3 cue stays silent.
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 5})
	driver.AdvanceTime(0)
	if out := driver.ConsumeAudio(); len(out.Cues) != 0 {
		t.Fatalf("scrub over cue point: expected 0 cues, got %d", len(out.Cues))
	}

	// Watching over the same range delivers the cue.
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.ConsumeAudio()
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
	for range 5 {
		driver.Step()
	}

	out := driver.ConsumeAudio()
	if len(out.Cues) != 1 {
		t.Fatalf("playback over cue point: expected 1 cue, got %d", len(out.Cues))
	}
	if out.Cues[0].Tick != 3 || out.Cues[0].Clip != "beep" {
		t.Fatalf("playback cue = %+v, want {Tick:3 Clip:beep}", out.Cues[0])
	}
}

// TestPlaybackAutoPausesAtEndAndCanStillFork checks that playback clamps and
// pauses at the recorded end while staying in watch mode, and that taking
// control from there resumes live play with no loss of state.
func TestPlaybackAutoPausesAtEndAndCanStillFork(t *testing.T) {
	game := &playbackGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	const runTicks = 10
	for range runTicks {
		driver.Step()
	}
	endChecksum := driver.Checksum()

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	for range runTicks + 5 { // requests more than available; must clamp
		driver.Step()
	}

	state := driver.State()
	if state.Tick != uint64(runTicks) {
		t.Fatalf("tick after playback to the end = %d, want %d", state.Tick, runTicks)
	}
	if !state.Paused {
		t.Fatal("expected playback to auto-pause at the end")
	}
	if !state.ReplayMode {
		t.Fatal("expected playback to remain in watch mode at the end")
	}
	if got := driver.Checksum(); got != endChecksum {
		t.Fatalf("checksum at end of playback = %d, want %d", got, endChecksum)
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlPlay})
	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
	})
	driver.Step()

	if got, want := driver.State().Tick, uint64(runTicks)+1; got != want {
		t.Fatalf("tick after resuming from the end = %d, want %d", got, want)
	}
	if driver.State().ReplayMode {
		t.Fatal("expected ControlPlay to have exited watch mode")
	}
}

// TestPlaybackHonorsSpeedViaClockPlan checks that playback advance is driven
// by the same clock plan as live play: setting speed changes how many
// recorded ticks a single AdvanceTime call consumes.
func TestPlaybackHonorsSpeedViaClockPlan(t *testing.T) {
	game := &playbackGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	const runTicks = 10
	for range runTicks {
		driver.Step()
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlSetSpeed, Speed: 2.0})

	driver.AdvanceTime(time.Second/60 + time.Nanosecond)

	if got, want := driver.State().Tick, uint64(2); got != want {
		t.Fatalf("tick after one frame of playback at 2x speed = %d, want %d", got, want)
	}
}

// TestPlaybackZeroStepsFrameDoesNotPause covers the first frame of an
// interactive host: FrameTimer.Elapsed always returns zero on its first
// sample, so the clock plan computes zero steps for that call. That must
// stay a no-op, not an auto-pause -- otherwise watch mode never gets past
// its own first frame on any interactive host.
func TestPlaybackZeroStepsFrameDoesNotPause(t *testing.T) {
	game := &playbackGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	const runTicks = 10
	for range runTicks {
		driver.Step()
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	// Zero elapsed, same as an interactive host's first frame: the clock
	// plan computes zero steps.
	driver.AdvanceTime(0)

	state := driver.State()
	if state.Paused {
		t.Fatal("expected a zero-step frame to stay a no-op, not auto-pause")
	}
	if state.Tick != 0 {
		t.Fatalf("tick after a zero-step frame = %d, want unchanged 0", state.Tick)
	}
	if !state.ReplayMode {
		t.Fatal("expected watch mode to remain on")
	}

	// Playback still advances normally afterward.
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if got := driver.State().Tick; got != 1 {
		t.Fatalf("tick after a real frame = %d, want 1", got)
	}
}

// TestReplayAndReplayForViewingEmitCuesAfterReconstruction proves reconstruction
// no longer leaves the driver permanently mute: the reconstructed range
// itself stays silent (those cues were already heard live), but a cue fired
// after the caller resumes live play is audible — for both Replay and
// ReplayForViewing.
func TestReplayAndReplayForViewingEmitCuesAfterReconstruction(t *testing.T) {
	source := runtime.NewDriver(&audioGame{}, slog.New(slog.DiscardHandler))
	source.StartRecording()

	// Record only 2 ticks (0, 1) — before audioGame's tick-3 Play("beep").
	for range 2 {
		source.Step()
	}
	rec := source.StopRecording()

	reconstruct := map[string]func() *runtime.Driver{
		"Replay": func() *runtime.Driver {
			return runtime.Replay(&audioGame{}, slog.New(slog.DiscardHandler), rec)
		},
		"ReplayForViewing": func() *runtime.Driver {
			return runtime.ReplayForViewing(&audioGame{}, slog.New(slog.DiscardHandler), rec)
		},
	}

	for name, build := range reconstruct {
		t.Run(name, func(t *testing.T) {
			driver := build()

			// The reconstructed range itself stays silent: those cues were
			// already heard on the live run.
			if out := driver.ConsumeAudio(); len(out.Cues) != 0 {
				t.Fatalf("first drain after reconstruction: expected 0 cues, got %d", len(out.Cues))
			}

			// ReplayForViewing returns a driver paused in watch mode; take
			// control explicitly before stepping live (a no-op for Replay).
			driver.Control(runtime.ControlCommand{Kind: runtime.ControlPlay})

			// Resume live past the reconstructed range: audioGame fires
			// Play("beep") at tick 3, now under live control.
			for range 2 {
				driver.Step()
			}

			out := driver.ConsumeAudio()
			if len(out.Cues) != 1 {
				t.Fatalf("expected the tick-3 cue to be audible after reconstruction, got %d cues", len(out.Cues))
			}
			if out.Cues[0].Tick != 3 || out.Cues[0].Clip != "beep" {
				t.Fatalf("cue = %+v, want {Tick:3 Clip:beep}", out.Cues[0])
			}
		})
	}
}

// TestReplayForViewingReturnsPausedAtFinalTickInWatchMode covers the shape of
// the driver ReplayForViewing hands back: paused, in watch mode, sitting on
// the last reconstructed tick with the full range scrubbable.
func TestReplayForViewingReturnsPausedAtFinalTickInWatchMode(t *testing.T) {
	source := runtime.NewDriver(&replayViewingGame{}, slog.New(slog.DiscardHandler))
	source.StartRecording()
	for range 20 {
		source.Step()
	}
	rec := source.StopRecording()

	viewing := runtime.ReplayForViewing(&replayViewingGame{}, slog.New(slog.DiscardHandler), rec)

	state := viewing.State()
	if !state.Paused {
		t.Fatal("expected ReplayForViewing to return a paused driver")
	}
	if !state.ReplayMode {
		t.Fatal("expected ReplayForViewing to return a driver in watch mode")
	}
	if state.Tick != rec.Ticks {
		t.Fatalf("tick = %d, want final tick %d", state.Tick, rec.Ticks)
	}
	if state.MaxRecordedTick != rec.Ticks {
		t.Fatalf("MaxRecordedTick = %d, want %d", state.MaxRecordedTick, rec.Ticks)
	}
}

// TestReplayForViewingKeepsRecorderPopulatedForDeepScrub covers the reason
// ReplayForViewing exists instead of always using Replay: a deep scrub past
// the dense recorder window restores from an archived keyframe (no Init
// call, i.e. no fall back to resetting from tick 0) and lands on the exact
// live checksum, because reconstruction captured keyframes as it went.
func TestReplayForViewingKeepsRecorderPopulatedForDeepScrub(t *testing.T) {
	source := runtime.NewDriver(&replayViewingGame{}, slog.New(slog.DiscardHandler))
	source.StartRecording()

	const runTicks = 40
	checksums := make([]uint64, 0, runTicks+1)
	checksums = append(checksums, source.Checksum())
	for range runTicks {
		source.Step()
		checksums = append(checksums, source.Checksum())
	}
	rec := source.StopRecording()

	game := &replayViewingGame{}
	viewing := runtime.ReplayForViewing(game, slog.New(slog.DiscardHandler), rec)
	initCallsAfterLoad := game.initCalls

	const target = 6 // older than the dense window (HistoryLen 8, KeyframeInterval 2)
	viewing.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: target})
	viewing.AdvanceTime(0)

	if game.initCalls != initCallsAfterLoad {
		t.Fatalf("Init calls after deep scrub = %d, want unchanged %d — scrub fell back to resetting from tick 0", game.initCalls, initCallsAfterLoad)
	}
	if got, want := viewing.Checksum(), checksums[target]; got != want {
		t.Fatalf("checksum after deep scrub = %d, want %d", got, want)
	}
}

// TestReplayForViewingMatchesReplayChecksum covers the equivalence between
// the two reconstruction variants: keeping the recorder populated must not
// change the reconstructed simulation state itself.
func TestReplayForViewingMatchesReplayChecksum(t *testing.T) {
	source := runtime.NewDriver(&replayViewingGame{}, slog.New(slog.DiscardHandler))
	source.StartRecording()
	for range 25 {
		source.Step()
	}
	rec := source.StopRecording()

	replay := runtime.Replay(&replayViewingGame{}, slog.New(slog.DiscardHandler), rec)
	viewing := runtime.ReplayForViewing(&replayViewingGame{}, slog.New(slog.DiscardHandler), rec)

	if got, want := viewing.Checksum(), replay.Checksum(); got != want {
		t.Fatalf("ReplayForViewing checksum = %d, want %d (same as Replay)", got, want)
	}
}
