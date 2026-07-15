package rhythm

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/runtime"
)

func TestRhythmAudioStartAndTrack(t *testing.T) {
	game := newTestGame(t)
	driver := runtime.NewDriver(game, slog.Default())
	driver.ApplyInput([]input.Event{keyDown(input.KeySpace)})
	driver.Step()

	output := driver.ConsumeAudio()
	if !hasCue(output.Cues, "start") {
		t.Fatal("starting a match did not emit the start cue")
	}

	for range simHz * 3 {
		if game.State.Phase == phasePlaying {
			break
		}
		driver.Step()
	}
	if game.State.Phase != phasePlaying {
		t.Fatalf("phase = %d, want playing", game.State.Phase)
	}
	output = driver.ConsumeAudio()
	if track, ok := output.Tracks["music"]; !ok || track.Clip != "song" {
		t.Fatalf("music track = %#v, want song", output.Tracks)
	}
}

func TestRhythmScrubRestoresMatchState(t *testing.T) {
	game := newTestGame(t)
	driver := runtime.NewDriver(game, slog.Default())
	snapshots := []MatchState{game.State}

	driver.ApplyInput([]input.Event{keyDown(input.KeySpace)})
	driver.Step()
	snapshots = append(snapshots, game.State)
	for range 320 {
		driver.Step()
		snapshots = append(snapshots, game.State)
	}

	for _, target := range []uint64{250, 40, 300, 0, 120} {
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
	driver.ApplyInput([]input.Event{keyDown(input.KeySpace)})
	driver.Step()
	for range 320 {
		driver.Step()
	}
	recording := driver.StopRecording()

	replay := newTestGame(t)
	replayedDriver := runtime.Replay(replay, slog.Default(), recording)
	if got, want := replay.State, live.State; got != want {
		t.Fatalf("replayed state = %#v, want %#v", got, want)
	}
	if got, want := replayedDriver.State().Tick, recording.Ticks; got != want {
		t.Fatalf("replay tick = %d, want %d", got, want)
	}
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
	laneKeys := [laneCount]input.SourceCode{input.KeyD, input.KeyF, input.KeyJ, input.KeyK}
	targetTick := game.State.SongStartTick + uint64(chart[firstTap].TimeMS*simHz/1000)
	for driver.State().Tick < targetTick {
		driver.Step()
	}
	driver.ApplyInput([]input.Event{keyDown(laneKeys[chart[firstTap].Lane])})
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

func hasCue(cues []audio.Cue, clip audio.ClipID) bool {
	for _, cue := range cues {
		if cue.Clip == clip {
			return true
		}
	}
	return false
}
