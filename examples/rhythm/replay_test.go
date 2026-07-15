package rhythm

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/runtime"
)

func TestRhythmAudioCoversGameplayAndScrub(t *testing.T) {
	game := newTestGame(t)
	driver := runtime.NewDriver(game, slog.Default())
	sequence := runRhythmGameplay(t, driver, game, nil)
	assertGameplayResults(t, game.State, sequence)

	output := driver.ConsumeAudio()
	if !hasCue(output.Cues, "start") {
		t.Fatal("starting a match did not emit the start cue")
	}
	track, ok := output.Tracks["music"]
	if !ok || track.Clip != "song" || track.StartTick != game.State.SongStartTick {
		t.Fatalf("music track = %#v, want song scheduled after the countdown", output.Tracks)
	}
	for _, clip := range []audio.ClipID{"hit_high", "hit_low", "miss"} {
		if !hasCue(output.Cues, clip) {
			t.Errorf("gameplay did not emit %q cue", clip)
		}
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: sequence.tapTick})
	driver.AdvanceTime(0)
	output = driver.ConsumeAudio()
	if len(output.Cues) != 0 {
		t.Fatalf("scrub re-emitted %d gameplay cues", len(output.Cues))
	}
	if track, ok := output.Tracks["music"]; !ok || track.Clip != "song" {
		t.Fatalf("scrub restored tracks = %#v, want music", output.Tracks)
	}
}

func TestRhythmScrubRestoresMatchState(t *testing.T) {
	game := newTestGame(t)
	driver := runtime.NewDriver(game, slog.Default())
	snapshots := []MatchState{game.State}
	sequence := runRhythmGameplay(t, driver, game, func() {
		snapshots = append(snapshots, game.State)
	})
	assertGameplayResults(t, game.State, sequence)

	for _, target := range []uint64{
		sequence.finalTick,
		sequence.holdTick + 1,
		sequence.tapTick + 1,
		0,
		sequence.holdEndTick + 1,
		sequence.missTick + 1,
	} {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)
		if got := game.State; got != snapshots[target] {
			t.Fatalf("scrub to %d restored %#v, want %#v", target, got, snapshots[target])
		}
	}
}

func TestRhythmRecordingReplayMatchesState(t *testing.T) {
	live := newTestGame(t)
	driver := runtime.NewDriver(live, slog.Default())
	driver.StartRecording()
	sequence := runRhythmGameplay(t, driver, live, nil)
	recording := driver.StopRecording()
	assertGameplayResults(t, live.State, sequence)

	replay := newTestGame(t)
	replayedDriver := runtime.Replay(replay, slog.Default(), recording)
	if got, want := replay.State, live.State; got != want {
		t.Fatalf("replayed state = %#v, want %#v", got, want)
	}
	if got, want := replayedDriver.State().Tick, recording.Ticks; got != want {
		t.Fatalf("replay tick = %d, want %d", got, want)
	}
	assertGameplayResults(t, replay.State, sequence)
}

func TestRhythmBestScoreDoesNotDecreaseOnRewind(t *testing.T) {
	game := newTestGame(t)
	driver := runtime.NewDriver(game, slog.Default())
	startPlaying(t, driver, game)

	firstTap := -1
	for i, obj := range chart {
		if obj.Kind == objectTap {
			firstTap = i
			break
		}
	}
	if firstTap < 0 {
		t.Skip("chart has no tap notes")
	}
	targetTick := nearestSongTick(game.State.SongStartTick, chart[firstTap].TimeMS)
	for driver.State().Tick < targetTick {
		driver.Step()
	}
	driver.ApplyInput([]input.Event{keyDown(laneKey(t, chart[firstTap].Lane))})
	driver.Step()
	if game.State.Score == 0 {
		t.Fatal("first tap note did not score through the mapped keyboard action")
	}
	best := game.best

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: targetTick - 1})
	driver.AdvanceTime(0)
	if game.best != best {
		t.Fatalf("best after rewind = %d, want %d", game.best, best)
	}
	if game.State.Score >= best {
		t.Fatalf("rewound score = %d, want lower than retained best %d", game.State.Score, best)
	}
}

type rhythmGameplaySequence struct {
	tapID       int
	holdID      int
	missID      int
	tapTick     uint64
	holdTick    uint64
	holdEndTick uint64
	missTick    uint64
	finalTick   uint64
	events      map[uint64][]input.Event
}

func runRhythmGameplay(t *testing.T, driver *runtime.Driver, game *Game, afterStep func()) rhythmGameplaySequence {
	t.Helper()
	step := func(events ...input.Event) {
		if len(events) > 0 {
			driver.ApplyInput(events)
		}
		driver.Step()
		if afterStep != nil {
			afterStep()
		}
	}

	step(keyDown(input.KeySpace))
	step(keyUp(input.KeySpace))
	sequence := newRhythmGameplaySequence(t, game.State.SongStartTick)

	lastTick := sequence.missTick
	if tick := sequence.tapTick + 1; tick > lastTick {
		lastTick = tick
	}
	if tick := sequence.holdEndTick + 1; tick > lastTick {
		lastTick = tick
	}
	for driver.State().Tick <= lastTick {
		step(sequence.events[driver.State().Tick]...)
	}
	sequence.finalTick = driver.State().Tick
	return sequence
}

func newRhythmGameplaySequence(t *testing.T, songStartTick uint64) rhythmGameplaySequence {
	t.Helper()
	sequence := rhythmGameplaySequence{tapID: -1, holdID: -1, missID: -1}
	for i, object := range chart {
		if sequence.tapID < 0 && object.Kind == objectTap {
			sequence.tapID = i
		}
		if sequence.holdID < 0 && isHold(object) {
			sequence.holdID = i
		}
	}
	for i, object := range chart {
		if i == sequence.tapID || i == sequence.holdID || object.Kind != objectTap {
			continue
		}
		sequence.missID = i
		break
	}
	if sequence.tapID < 0 || sequence.holdID < 0 || sequence.missID < 0 {
		t.Fatal("chart needs at least two taps and one hold for gameplay replay coverage")
	}

	tap := chart[sequence.tapID]
	hold := chart[sequence.holdID]
	miss := chart[sequence.missID]
	sequence.tapTick = nearestSongTick(songStartTick, tap.TimeMS)
	sequence.holdTick = nearestSongTick(songStartTick, hold.TimeMS)
	sequence.holdEndTick = firstSongTickAtOrAfter(songStartTick, hold.EndMS)
	sequence.missTick = firstSongTickAtOrAfter(songStartTick, miss.TimeMS+missWindowMS+1)
	sequence.events = make(map[uint64][]input.Event, 4)

	add := func(tick uint64, event input.Event) {
		sequence.events[tick] = append(sequence.events[tick], event)
	}
	tapKey := laneKey(t, tap.Lane)
	add(sequence.tapTick, keyDown(tapKey))
	add(sequence.tapTick+1, keyUp(tapKey))
	holdKey := laneKey(t, hold.Lane)
	add(sequence.holdTick, keyDown(holdKey))
	add(sequence.holdEndTick+1, keyUp(holdKey))
	return sequence
}

func assertGameplayResults(t *testing.T, state MatchState, sequence rhythmGameplaySequence) {
	t.Helper()
	if got := state.Results[sequence.tapID]; got != resultMarvelous {
		t.Errorf("tap result = %d, want marvelous", got)
	}
	if got := state.Results[sequence.holdID]; got != resultPerfect {
		t.Errorf("hold result = %d, want completed perfect", got)
	}
	if got := state.Results[sequence.missID]; got != resultMiss {
		t.Errorf("unplayed note result = %d, want miss", got)
	}
	if state.Score == 0 || state.Miss == 0 {
		t.Errorf("gameplay state = score:%d miss:%d, want both scoring and misses", state.Score, state.Miss)
	}
}

func nearestSongTick(songStartTick uint64, timeMS int) uint64 {
	return songStartTick + uint64((timeMS*simHz+500)/1000)
}

func firstSongTickAtOrAfter(songStartTick uint64, timeMS int) uint64 {
	return songStartTick + uint64((timeMS*simHz+999)/1000)
}

func laneKey(t *testing.T, lane int) input.SourceCode {
	t.Helper()
	if lane < 0 || lane >= len(laneActions) {
		t.Fatalf("lane %d has no keyboard action", lane)
	}
	action := laneActions[lane]
	for _, binding := range bindings() {
		if binding.Action == action && binding.Source.Device == input.DeviceKeyboard {
			return binding.Source.Code
		}
	}
	t.Fatalf("lane %d has no keyboard binding", lane)
	return 0
}

func newTestGame(t *testing.T) *Game {
	t.Helper()
	game, err := NewGame()
	if err != nil {
		t.Fatalf("NewGame() error = %v", err)
	}
	return game
}

func startPlaying(t *testing.T, driver *runtime.Driver, game *Game) {
	t.Helper()
	driver.ApplyInput([]input.Event{keyDown(input.KeySpace)})
	driver.Step()
	driver.ApplyInput([]input.Event{keyUp(input.KeySpace)})
	driver.Step()
	for range simHz * 3 {
		if game.State.Phase == phasePlaying {
			return
		}
		driver.Step()
	}
	t.Fatalf("phase = %d, want playing", game.State.Phase)
}

func keyDown(key input.SourceCode) input.Event {
	return input.Event{Source: input.Source{Device: input.DeviceKeyboard, Code: key}, Kind: input.EventButtonDown}
}

func keyUp(key input.SourceCode) input.Event {
	return input.Event{Source: input.Source{Device: input.DeviceKeyboard, Code: key}, Kind: input.EventButtonUp}
}

func hasCue(cues []audio.Cue, clip audio.ClipID) bool {
	for _, cue := range cues {
		if cue.Clip == clip {
			return true
		}
	}
	return false
}
