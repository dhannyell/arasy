package simfuzz

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/runtime"
)

// quietLog discards engine log output so fuzz runs stay quiet and fast.
var quietLog = slog.New(slog.NewTextHandler(io.Discard, nil))

// runDriver builds a fresh fuzz game and driver from script, replays the
// script's input event log tick by tick, and returns the driver, game and the
// checksum at every tick (index 0 is the post-Init tick-0 state).
type fuzzGameFactory func(Script) *Game

func runDriver(script Script) (*runtime.Driver, *Game, []uint64) {
	return runDriverWith(script, NewGame)
}

func runHistoryDriver(script Script) (*runtime.Driver, *Game, []uint64) {
	return runDriverWith(script, newHistoryGame)
}

func runDriverWith(script Script, newGame fuzzGameFactory) (*runtime.Driver, *Game, []uint64) {
	game := newGame(script)
	driver := runtime.NewDriver(game, quietLog)
	byTick := groupEventsByTick(script.Events)
	checksums := make([]uint64, 0, script.MaxTicks+1)
	checksums = append(checksums, driver.Checksum())
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}
	return driver, game, checksums
}

type reproPaths struct {
	corpus string
	replay string
}

func pathsForRepro(target string, data []byte) reproPaths {
	h := fnv.New64a()
	_, _ = h.Write(data)
	base := fmt.Sprintf("%016x", h.Sum64())
	dir := filepath.Join("testdata", "repro", target)
	return reproPaths{
		corpus: filepath.Join(dir, base+".txt"),
		replay: filepath.Join(dir, base+".arrp"),
	}
}

func encodeCorpusEntry(data []byte) []byte {
	return []byte("go test fuzz v1\n[]byte(" + strconv.Quote(string(data)) + ")\n")
}

func scriptForRepro(target string, data []byte) Script {
	if target == "FuzzSubstepTunneling" {
		return decodeSubstepScenario(data).Script
	}
	return DecodeScript(data)
}

func recordScript(script Script) (runtime.Recording, uint64) {
	driver := runtime.NewDriver(NewGame(script), quietLog)
	driver.StartRecording()
	byTick := groupEventsByTick(script.Events)
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
	}
	return driver.StopRecording(), driver.Checksum()
}

// dumpRepro writes both a valid Go fuzz corpus entry and an ARRP replay to
// testdata/repro/<target>/. The corpus file recreates the exact target input;
// the replay makes the resulting runtime timeline independently inspectable.
// It never hides the original assertion when artifact creation fails.
func dumpRepro(t testing.TB, target string, data []byte) string {
	t.Helper()
	paths := pathsForRepro(target, data)
	dir := filepath.Dir(paths.corpus)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("dumpRepro: mkdir %s: %v", dir, err)
		return dir
	}

	written := make([]string, 0, 2)
	if err := os.WriteFile(paths.corpus, encodeCorpusEntry(data), 0o644); err != nil {
		t.Logf("dumpRepro: write corpus %s: %v", paths.corpus, err)
	} else {
		written = append(written, paths.corpus)
	}

	var replay []byte
	if target == "FuzzRecordingCodec" {
		if _, err := runtime.ReadRecordingFromBytes(data); err == nil {
			replay = append([]byte(nil), data...)
		}
	}
	if len(replay) == 0 {
		rec, _ := recordScript(scriptForRepro(target, data))
		var buf bytes.Buffer
		if _, err := rec.WriteTo(&buf); err != nil {
			t.Logf("dumpRepro: encode replay %s: %v", paths.replay, err)
		} else {
			replay = buf.Bytes()
		}
	}
	if len(replay) > 0 {
		if err := os.WriteFile(paths.replay, replay, 0o644); err != nil {
			t.Logf("dumpRepro: write replay %s: %v", paths.replay, err)
		} else {
			written = append(written, paths.replay)
		}
	}
	if len(written) == 0 {
		return dir
	}
	return strings.Join(written, ", ")
}

// checkDeterminism runs P1: two independent drivers from the same script must
// produce an identical checksum at every tick.
func checkDeterminism(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	_, _, a := runDriver(script)
	_, _, b := runDriver(script)
	// Both runs step the same script, so a length difference is itself a
	// divergence. Comparing only the shared prefix would hide it.
	if len(a) != len(b) {
		t.Fatalf("tick count diverged: %d != %d\nrepro: %s",
			len(a), len(b), dumpRepro(t, "FuzzDeterminism", data))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("checksum diverged at tick %d: %d != %d\nrepro: %s",
				i, a[i], b[i], dumpRepro(t, "FuzzDeterminism", data))
		}
	}
}

// checkReplayFidelity runs P2: record the live run, serialize and re-read the
// recording, VerifyRecording must pass, and a replay must reach the live run's
// final checksum.
func checkReplayFidelity(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	driver := runtime.NewDriver(NewGame(script), quietLog)
	driver.StartRecording()
	byTick := groupEventsByTick(script.Events)
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
	}
	live := driver.Checksum()
	rec := driver.StopRecording()

	var buf bytes.Buffer
	if _, err := rec.WriteTo(&buf); err != nil {
		t.Fatalf("recording write: %v\nrepro: %s", err, dumpRepro(t, "FuzzReplayFidelity", data))
	}
	rec2, err := runtime.ReadRecordingFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("recording re-read: %v\nrepro: %s", err, dumpRepro(t, "FuzzReplayFidelity", data))
	}
	if err := runtime.VerifyRecording(NewGame(script), quietLog, rec2); err != nil {
		t.Fatalf("VerifyRecording: %v\nrepro: %s", err, dumpRepro(t, "FuzzReplayFidelity", data))
	}
	if got := runtime.Replay(NewGame(script), quietLog, rec2).Checksum(); got != live {
		t.Fatalf("replay checksum %d != live %d\nrepro: %s",
			got, live, dumpRepro(t, "FuzzReplayFidelity", data))
	}
}

// checkRewindConvergence runs P3: advance to N, scrub back to K<N, scrub
// forward to N again — the checksum at N must match the live run. The driver
// replays the journal internally, so the harness does not re-apply input.
func checkRewindConvergence(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	driver, _, checksums := runHistoryDriver(script)
	n := len(checksums) - 1
	if n < 8 {
		t.Skip("script too short for rewind")
	}
	k := n / 2
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: uint64(k)})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: uint64(n)})
	driver.AdvanceTime(0)
	if got := driver.State().Tick; got != uint64(n) {
		t.Fatalf("tick after rewind round-trip = %d, want %d\nrepro: %s",
			got, n, dumpRepro(t, "FuzzRewindConvergence", data))
	}
	if got, want := driver.Checksum(), checksums[n]; got != want {
		t.Fatalf("rewind divergence at tick %d: %d != %d\nrepro: %s",
			n, got, want, dumpRepro(t, "FuzzRewindConvergence", data))
	}
}
