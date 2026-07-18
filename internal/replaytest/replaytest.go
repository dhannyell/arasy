// Package replaytest verifies committed FFRP golden replays: a recorded
// input log whose checksum anchors and final game state must reproduce
// exactly on every architecture the CI runs.
package replaytest

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/frameforge/runtime"
)

// Script drives a live run that produces the golden recording.
// It must be deterministic: fixed event ticks, no wall clock, no I/O.
type Script func(d *runtime.Driver)

// Golden verifies (or, with update, regenerates) a golden replay:
//   - <dir>/<name>.ffrp        — the recording (events + anchors)
//   - <dir>/<name>.state.txt   — fingerprint of the final game state
//
// newGame must return a fresh Game each call (Golden calls it up to three
// times: once to record or verify anchors, once more to replay for the
// fingerprint). fingerprint must be deterministic; demo state structs are
// value types (the StateSlot contract), so formatting with "%+v" is stable.
func Golden(t *testing.T, update bool, dir, name string, newGame func() runtime.Game,
	fingerprint func(game runtime.Game) string, script Script) {
	t.Helper()

	recPath := filepath.Join(dir, name+".ffrp")
	statePath := filepath.Join(dir, name+".state.txt")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	if update {
		game := newGame()
		driver := runtime.NewDriver(game, log)
		driver.StartRecording()
		script(driver)
		rec := driver.StopRecording()

		if len(rec.Anchors) == 0 || len(rec.Events) == 0 {
			t.Fatalf("golden replay %s: script recorded %d anchors and %d events; an empty script produces a useless golden",
				name, len(rec.Anchors), len(rec.Events))
		}

		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
		f, err := os.Create(recPath)
		if err != nil {
			t.Fatalf("create %s: %v", recPath, err)
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {
				t.Fatalf("close %s: %v", recPath, err)
			}
		}(f)
		if _, err := rec.WriteTo(f); err != nil {
			t.Fatalf("write %s: %v", recPath, err)
		}

		if err := os.WriteFile(statePath, []byte(fingerprint(game)), 0o644); err != nil {
			t.Fatalf("write %s: %v", statePath, err)
		}
		t.Logf("updated %s and %s", recPath, statePath)
		return
	}

	data, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test <pkg> -run TestGoldenReplay -update; "+
			"if the simulation change is intentional, regenerate and explain the diff in the PR)", recPath, err)
	}
	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		t.Fatalf("decode golden %s: %v", recPath, err)
	}

	if err := runtime.VerifyRecording(newGame(), log, rec); err != nil {
		t.Fatalf("golden replay %s: %v (if the simulation change is intentional, "+
			"regenerate with -update and explain the diff in the PR)", name, err)
	}

	replayGame := newGame()
	runtime.Replay(replayGame, log, rec)
	got := fingerprint(replayGame)

	wantBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read golden %s: %v", statePath, err)
	}
	if want := string(wantBytes); got != want {
		t.Fatalf("golden replay %s: final state diverged\n got:  %s\n want: %s\n"+
			"(if the simulation change is intentional, regenerate with -update and explain the diff in the PR)",
			name, got, want)
	}
}
