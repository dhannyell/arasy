package runtime_test

import (
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/audio"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// audioGame is a test game that emits audio cues and tracks at known ticks.
// Used by TestAudio* to validate cue stamps, draining, tracks, rewind/scrub
// semantics, and determinism.
type audioGame struct {
	player scene.EntityID
}

func (g *audioGame) Setup() runtime.Config {
	assets, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"audio":[{"id":"beep","file":"beep.wav"},{"id":"beep2","file":"beep2.wav"},{"id":"beep3","file":"beep3.wav"},{"id":"music","file":"music.wav"},{"id":"alt_music","file":"alt_music.wav"}]}`)},
		"alt_music.wav": {Data: []byte("alt")},
		"beep.wav":      {Data: []byte("beep")},
		"beep2.wav":     {Data: []byte("beep2")},
		"beep3.wav":     {Data: []byte("beep3")},
		"music.wav":     {Data: []byte("music")},
	})
	if err != nil {
		panic(err)
	}
	return runtime.Config{
		Bounds: math2d.NewRect(
			math2d.Vec2{X: -100, Y: -100},
			math2d.Vec2{X: 200, Y: 200},
		),
		Assets:     assets,
		HistoryLen: 60,
	}
}

func (g *audioGame) Init(ctx *runtime.UpdateContext) {
	g.player = ctx.World.Spawn()
	ctx.Audio.SetTrack("music", audio.Track{Clip: "music", Gain: 0.5, Loop: true, FadeSec: 0.5, StartTick: 30})
}

func (g *audioGame) Update(ctx *runtime.UpdateContext, dt float64) {
	// Move the player so the world has deterministic state changes
	// (used by TestAudioDeterminismWithExplicitReplay).
	ctx.World.SetVelocity(g.player, scene.Velocity{Linear: math2d.Vec2{X: 10, Y: 0}})

	if ctx.Tick == 3 {
		ctx.Audio.Play("beep")
	}
	if ctx.Tick == 5 {
		ctx.Audio.SetTrack("music", audio.Track{Clip: "alt_music", Gain: 0.8, Loop: true})
	}
	if ctx.Tick == 7 {
		ctx.Audio.Play("beep2")
		ctx.Audio.Play("beep3")
	}
	if ctx.Tick == 8 {
		ctx.Audio.Play("beep")
	}
}

func (g *audioGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

type pausedTrackGame struct{}

func (pausedTrackGame) Setup() runtime.Config { return runtime.Config{} }

func (pausedTrackGame) Init(ctx *runtime.UpdateContext) {
	ctx.Audio.SetTrack("music", audio.Track{Clip: "music", Loop: true, StartTick: 1})
}

func (pausedTrackGame) Update(ctx *runtime.UpdateContext, _ float64) {
	switch ctx.Tick {
	case 0, 1, 4:
		ctx.Audio.PauseTrack("music")
	case 2:
		ctx.Audio.SetTrack("music", audio.Track{Clip: "music", Gain: 0.5, Loop: true, StartTick: 1})
	case 3, 6:
		ctx.Audio.ResumeTrack("music")
	}
}

func (pausedTrackGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

type anchoredTrackGame struct{}

func (anchoredTrackGame) Setup() runtime.Config { return runtime.Config{} }

func (anchoredTrackGame) Init(*runtime.UpdateContext) {}

func (anchoredTrackGame) Update(ctx *runtime.UpdateContext, _ float64) {
	switch ctx.Tick {
	case 1:
		ctx.Audio.SetTrack("music", audio.Track{Clip: "music", Loop: true})
	case 2:
		ctx.Audio.SetTrack("music", audio.Track{Clip: "music", Gain: 0.5, Loop: true})
	case 3:
		ctx.Audio.SetTrack("music", audio.Track{Clip: "other", Loop: true})
	case 4:
		ctx.Audio.SetTrack("music", audio.Track{Clip: "music", Loop: true, StartTick: 99})
	}
}

func (anchoredTrackGame) Render(*runtime.RenderContext) []render.Command { return nil }

// TestAudioCueStampedAndDrainedOnce verifies that a cue fired at tick 3
// appears in ConsumeAudio with the correct tick, seq, gain, and clip, and
// that a subsequent consume returns empty cues (drained) but still carries
// the desired tracks.
func TestAudioCueStampedAndDrainedOnce(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Run 5 ticks (0..4). At tick 3, Update fires Play("beep").
	for range 5 {
		driver.Step()
	}

	// First consume: the cue from tick 3 must be present.
	out1 := driver.ConsumeAudio()

	if len(out1.Cues) != 1 {
		t.Fatalf("expected 1 cue, got %d", len(out1.Cues))
	}
	if out1.Cues[0].Tick != 3 {
		t.Errorf("expected cue.Tick == 3, got %d", out1.Cues[0].Tick)
	}
	if out1.Cues[0].Seq != 0 {
		t.Errorf("expected cue.Seq == 0, got %d", out1.Cues[0].Seq)
	}
	if out1.Cues[0].Gain != 1.0 {
		t.Errorf("expected cue.Gain == 1.0, got %f", out1.Cues[0].Gain)
	}
	if out1.Cues[0].Clip != "beep" {
		t.Errorf("expected cue.Clip == 'beep', got %s", out1.Cues[0].Clip)
	}
	if out1.Tracks["music"].Clip != "music" {
		t.Errorf("expected track 'music' Clip == 'music', got %s", out1.Tracks["music"].Clip)
	}
	if out1.Tick != 5 {
		t.Errorf("expected audio output tick 5, got %d", out1.Tick)
	}
	if out1.Tracks["music"].StartTick != 30 {
		t.Errorf("expected track start tick 30, got %d", out1.Tracks["music"].StartTick)
	}

	// Second consume: cues must be empty (drained), tracks unchanged.
	out2 := driver.ConsumeAudio()

	if len(out2.Cues) != 0 {
		t.Errorf("expected 0 cues on second consume, got %d", len(out2.Cues))
	}
	if out2.Tracks["music"].Clip != "music" {
		t.Errorf("expected track 'music' to remain after drain, got %s", out2.Tracks["music"].Clip)
	}
}

func TestAudioOutputCarriesCurrentTickFraction(t *testing.T) {
	driver := runtime.NewDriver(&audioGame{}, slog.New(slog.DiscardHandler))
	driver.AdvanceTime(time.Second / 120)

	out := driver.ConsumeAudio()
	if out.Tick != 0 {
		t.Fatalf("audio output tick = %d, want 0", out.Tick)
	}
	if out.Alpha < 0.49 || out.Alpha > 0.51 {
		t.Fatalf("audio output alpha = %f, want approximately 0.5", out.Alpha)
	}
}

// TestAudioSeqDeterministic verifies that Play calls within the same tick
// receive sequential Seq values (0, 1, ...) and that the sequence resets
// to 0 at the start of the next tick.
func TestAudioSeqDeterministic(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Step 9 times (0..8):
	//   tick 3: Play("beep")                  → Seq 0
	//   tick 7: Play("beep2"), Play("beep3")  → Seq 0, Seq 1
	//   tick 8: Play("beep")                  → Seq 0 (reset)
	for range 9 {
		driver.Step()
	}

	out := driver.ConsumeAudio()
	if len(out.Cues) != 4 {
		t.Fatalf("expected 4 cues, got %d", len(out.Cues))
	}

	// Tick 3: single cue, Seq 0.
	if out.Cues[0].Tick != 3 || out.Cues[0].Seq != 0 || out.Cues[0].Clip != "beep" {
		t.Errorf("cue[0] = {Tick:%d Seq:%d Clip:%s}, want {3 0 beep}",
			out.Cues[0].Tick, out.Cues[0].Seq, out.Cues[0].Clip)
	}

	// Tick 7, first cue: Seq 0.
	if out.Cues[1].Tick != 7 || out.Cues[1].Seq != 0 || out.Cues[1].Clip != "beep2" {
		t.Errorf("cue[1] = {Tick:%d Seq:%d Clip:%s}, want {7 0 beep2}",
			out.Cues[1].Tick, out.Cues[1].Seq, out.Cues[1].Clip)
	}

	// Tick 7, second cue: Seq 1.
	if out.Cues[2].Tick != 7 || out.Cues[2].Seq != 1 || out.Cues[2].Clip != "beep3" {
		t.Errorf("cue[2] = {Tick:%d Seq:%d Clip:%s}, want {7 1 beep3}",
			out.Cues[2].Tick, out.Cues[2].Seq, out.Cues[2].Clip)
	}

	// Tick 8: Seq resets to 0.
	if out.Cues[3].Tick != 8 || out.Cues[3].Seq != 0 || out.Cues[3].Clip != "beep" {
		t.Errorf("cue[3] = {Tick:%d Seq:%d Clip:%s}, want {8 0 beep}",
			out.Cues[3].Tick, out.Cues[3].Seq, out.Cues[3].Clip)
	}
}

// TestAudioTracksReflectDesired verifies that tracks are properly set after a
// SetTrack call, that the returned map is independent (clone isolation), and
// that mutating the consumed output does not affect the driver.
func TestAudioTracksReflectDesired(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Step 6 times (0..5). At tick 5, Update changes the track to alt_music.
	for range 6 {
		driver.Step()
	}

	out := driver.ConsumeAudio()

	if len(out.Cues) != 1 {
		t.Fatalf("expected 1 cue, got %d", len(out.Cues))
	}

	// After tick 5, the track should be alt_music with gain 0.8.
	if out.Tracks["music"].Clip != "alt_music" {
		t.Errorf("expected Clip 'alt_music', got %s", out.Tracks["music"].Clip)
	}
	if out.Tracks["music"].Gain != 0.8 {
		t.Errorf("expected Gain 0.8, got %f", out.Tracks["music"].Gain)
	}

	// Mutate the returned map: must not affect the driver's internal state.
	out.Tracks["music"] = audio.Track{Clip: "hacked"}
	out2 := driver.ConsumeAudio()

	if out2.Tracks["music"].Clip != "alt_music" {
		t.Errorf("clone isolation broken: expected 'alt_music', got %s", out2.Tracks["music"].Clip)
	}
}

// noisyGame emits 3 cues every tick to test the cue queue cap.
type noisyGame struct {
	audioGame
}

func (g *noisyGame) Update(ctx *runtime.UpdateContext, dt float64) {
	for i := 0; i < 3; i++ {
		ctx.Audio.Play("beep")
	}
}

// TestAudioResetClearsAll verifies that after a ControlReset + Step, all
// pending cues are cleared and tracks are restored to the Init state.
func TestAudioResetClearsAll(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Run 10 ticks to accumulate cues (tick 3, 7, 8) and change track (tick 5).
	for range 10 {
		driver.Step()
	}

	// Reset + Step: doReset calls audio.reset() then Init.
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReset})
	driver.Step()

	out := driver.ConsumeAudio()

	// All pending cues from the previous run must be gone.
	if len(out.Cues) != 0 {
		t.Errorf("expected 0 cues after reset, got %d", len(out.Cues))
	}

	// Track should be restored to the Init state.
	if out.Tracks["music"].Clip != "music" {
		t.Errorf("expected track 'music' after reset, got %s", out.Tracks["music"].Clip)
	}
}

// TestAudioRewindRestoresTracksDiscardsPending verifies that rewinding to a
// tick before a SetTrack restores the old track and discards orphan cues from
// the discarded future.
func TestAudioRewindRestoresTracksDiscardsPending(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Run 10 ticks (0..9). Tick 3: Play, tick 5: track change, tick 7/8: Play.
	for range 10 {
		driver.Step()
	}
	// Do NOT consume — cues are still pending.

	// BackToTick to before the track change (tick 2, before tick 5).
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlBackToTick, Tick: 2})
	driver.AdvanceTime(0)

	// Step once (processes tick 2 in the re-simulated timeline).
	driver.Step()

	out := driver.ConsumeAudio()

	// No orphan cues from the discarded future.
	if len(out.Cues) != 0 {
		t.Errorf("expected 0 orphan cues after rewind, got %d", len(out.Cues))
	}

	// Track should be the original one (before tick 5 change).
	if out.Tracks["music"].Clip != "music" {
		t.Errorf("expected original track 'music', got %s", out.Tracks["music"].Clip)
	}
}

// TestAudioScrubSuppressesCuesReconstructsTracks verifies that scrubbing to a
// tick does not replay cues (already heard) but correctly reconstructs the
// track state at the target tick.
func TestAudioScrubSuppressesCuesReconstructsTracks(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Run 10 ticks (0..9). Tick 3: Play, tick 5: track change.
	for range 10 {
		driver.Step()
	}
	driver.ConsumeAudio() // drain everything

	// Scrub to tick 8, which is after the track change (tick 5).
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 8})
	driver.AdvanceTime(0)

	out := driver.ConsumeAudio()

	// Cues must be empty (replay suppressed them).
	if len(out.Cues) != 0 {
		t.Errorf("expected 0 cues after scrub, got %d", len(out.Cues))
	}

	// Track should reflect what was active at tick 8: alt_music (changed at tick 5).
	if out.Tracks["music"].Clip != "alt_music" {
		t.Errorf("expected track 'alt_music' at tick 8 after scrub, got %s", out.Tracks["music"].Clip)
	}
}

// TestAudioForwardScrubSuppressesCues verifies that scrubbing FORWARD (target
// tick greater than the current one, the incremental fast-forward path in
// seekTo) is just as silent as scrubbing back: the re-simulated cues were
// already heard and must not replay.
func TestAudioForwardScrubSuppressesCues(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	for range 10 {
		driver.Step()
	}
	driver.ConsumeAudio() // drain

	// Scrub back to tick 2, then forward to tick 8 (target > current tick).
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 2})
	driver.AdvanceTime(0)
	driver.ConsumeAudio() // drain

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 8})
	driver.AdvanceTime(0)

	out := driver.ConsumeAudio()

	if len(out.Cues) != 0 {
		t.Errorf("expected 0 cues after forward scrub, got %d", len(out.Cues))
	}
	// Track state at tick 8 must still be reconstructed (alt_music from tick 5).
	if out.Tracks["music"].Clip != "alt_music" {
		t.Errorf("expected track 'alt_music' at tick 8 after forward scrub, got %s", out.Tracks["music"].Clip)
	}
}

// TestAudioForkReemits verifies that after scrubbing back and then stepping
// forward past a cue point, the cue fires again in the new timeline.
func TestAudioForkReemits(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	// Run 10 ticks: tick 3 fires Play("beep").
	for range 10 {
		driver.Step()
	}
	driver.ConsumeAudio() // drain

	// Scrub to tick 2 (before the cue at tick 3).
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 2})
	driver.AdvanceTime(0)
	driver.ConsumeAudio() // drain any artifacts

	// Step until past tick 3 in the new branch.
	for range 2 {
		driver.Step()
	}
	// Tick 3 fires again in the new timeline.
	out := driver.ConsumeAudio()

	if len(out.Cues) != 1 {
		t.Fatalf("expected 1 cue on forked timeline, got %d", len(out.Cues))
	}
	if out.Cues[0].Tick != 3 || out.Cues[0].Clip != "beep" {
		t.Errorf("expected cue {Tick:3 Clip:beep}, got {Tick:%d Clip:%s}",
			out.Cues[0].Tick, out.Cues[0].Clip)
	}
}

// TestAudioCapFila verifies that the cue queue is capped at 512 and that the
// oldest cues are dropped in favour of the most recent ones.
func TestAudioCapFila(t *testing.T) {
	game := &noisyGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))

	const cap = 512

	// Run cap ticks without consuming. noisyGame emits 3 cues/tick,
	// totalling 1536 cues — only the last 512 fit.
	for range cap {
		driver.Step()
	}

	out := driver.ConsumeAudio()
	if len(out.Cues) != cap {
		t.Fatalf("expected %d cues (cap), got %d", cap, len(out.Cues))
	}

	// The oldest cues were dropped: the first cue should NOT be from tick 0.
	if out.Cues[0].Tick == 0 {
		t.Errorf("expected oldest cues to be dropped, but first cue is from tick 0")
	}

	// The last cue should be from the last processed tick: tick 511
	// (after cap=512 Step() calls, the driver has processed ticks 0..511).
	last := out.Cues[len(out.Cues)-1]
	if last.Tick != cap-1 {
		t.Errorf("expected last cue from tick %d, got tick %d", cap-1, last.Tick)
	}
}

// TestAudioDeterminismWithExplicitReplay verifies that recording a run with
// audio cues and replaying it produces the same world checksum — audio must
// not perturb simulation determinism.
func TestAudioDeterminismWithExplicitReplay(t *testing.T) {
	game := &audioGame{}
	driver := runtime.NewDriver(game, slog.New(slog.DiscardHandler))
	driver.StartRecording()

	// Apply input + step. audioGame.Update also plays audio,
	// but the world checksum depends only on the player's position.
	for range 10 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}

	originalChecksum := driver.Checksum()
	rec := driver.StopRecording()

	replay := runtime.Replay(&audioGame{}, slog.New(slog.DiscardHandler), rec)
	replayChecksum := replay.Checksum()

	if replayChecksum != originalChecksum {
		t.Errorf("replay checksum %d != original %d — audio perturbed simulation",
			replayChecksum, originalChecksum)
	}
}

func TestAudioTrackPauseAccumulatesAndSetTrackPreservesState(t *testing.T) {
	driver := runtime.NewDriver(pausedTrackGame{}, slog.New(slog.DiscardHandler))
	for range 7 {
		driver.Step()
	}

	track, ok := driver.ConsumeAudio().Tracks["music"]
	if !ok {
		t.Fatal("music track missing")
	}
	if track.Paused {
		t.Fatal("music track remains paused after ResumeTrack")
	}
	if track.PausedTicks != 5 {
		t.Fatalf("paused ticks = %d, want 5", track.PausedTicks)
	}
	if track.PausedAtTick != 0 {
		t.Fatalf("paused-at tick = %d, want 0 after resume", track.PausedAtTick)
	}
	if track.Gain != 0.5 {
		t.Fatalf("gain after SetTrack = %v, want 0.5", track.Gain)
	}
}

func TestAudioTrackAutoAnchorPreservesSameClipAndHonorsExplicitAnchor(t *testing.T) {
	driver := runtime.NewDriver(anchoredTrackGame{}, slog.New(slog.DiscardHandler))
	stepTo := func(tick uint64) audio.Track {
		t.Helper()
		for driver.State().Tick < tick {
			driver.Step()
		}
		track, ok := driver.ConsumeAudio().Tracks["music"]
		if !ok {
			t.Fatal("music track missing")
		}
		return track
	}

	if got := stepTo(2).StartTick; got != 1 {
		t.Fatalf("initial auto anchor = %d, want 1", got)
	}
	if got := stepTo(3).StartTick; got != 1 {
		t.Fatalf("same-clip auto anchor = %d, want preserved 1", got)
	}
	if got := stepTo(4).StartTick; got != 3 {
		t.Fatalf("replacement auto anchor = %d, want 3", got)
	}
	if got := stepTo(5).StartTick; got != 99 {
		t.Fatalf("explicit anchor = %d, want 99", got)
	}
}
