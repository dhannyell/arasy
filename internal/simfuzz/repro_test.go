package simfuzz

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/runtime"
)

// TestReproRoundTrip proves both artifacts are useful: the corpus file contains
// the exact fuzz input in Go's native format, while the ARRP timeline verifies
// and reaches the same final world checksum as the live fuzz-game run.
func TestReproRoundTrip(t *testing.T) {
	data := seedBytes(30, CompTransform|CompVelocity|CompCircleCollider)
	target := "TestReproRoundTrip"
	dir := filepath.Join("testdata", "repro", target)
	_ = os.RemoveAll(dir)
	_ = dumpRepro(t, target, data)
	paths := pathsForRepro(target, data)

	corpus, err := os.ReadFile(paths.corpus)
	if err != nil {
		t.Fatalf("read corpus repro %s: %v", paths.corpus, err)
	}
	lines := strings.Split(strings.TrimSpace(string(corpus)), "\n")
	if len(lines) != 2 || lines[0] != "go test fuzz v1" ||
		!strings.HasPrefix(lines[1], "[]byte(") || !strings.HasSuffix(lines[1], ")") {
		t.Fatalf("invalid Go fuzz corpus entry:\n%s", corpus)
	}
	quoted := strings.TrimSuffix(strings.TrimPrefix(lines[1], "[]byte("), ")")
	decoded, err := strconv.Unquote(quoted)
	if err != nil {
		t.Fatalf("unquote corpus repro: %v", err)
	}
	if !bytes.Equal([]byte(decoded), data) {
		t.Fatalf("corpus bytes differ: got %d bytes, want %d", len(decoded), len(data))
	}

	replayData, err := os.ReadFile(paths.replay)
	if err != nil {
		t.Fatalf("read replay repro %s: %v", paths.replay, err)
	}
	rec, err := runtime.ReadRecordingFromBytes(replayData)
	if err != nil {
		t.Fatalf("decode replay repro: %v", err)
	}
	script := DecodeScript(data)
	if err := runtime.VerifyRecording(NewGame(script), quietLog, rec); err != nil {
		t.Fatalf("verify replay repro: %v", err)
	}
	_, live := recordScript(script)
	if got := runtime.Replay(NewGame(script), quietLog, rec).Checksum(); got != live {
		t.Fatalf("replay repro checksum = %d, want live %d", got, live)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Logf("cleanup repro dir: %v", err)
	}
}

func TestCommittedSubstepReplay(t *testing.T) {
	data := []byte("0800000\x00\xff")
	replayPath := filepath.Join(
		"testdata", "replays", "FuzzSubstepTunneling", "quadratic-drag-overflow.arrp",
	)
	verifyCommittedReplay(t, replayPath, decodeSubstepScenario(data).Script)
}

func TestCommittedNumericReplay(t *testing.T) {
	data := []byte("000X0\xff0000000000000000000000000000000000000000000000000000000000000000000000000000000000\xc70")
	replayPath := filepath.Join(
		"testdata", "replays", "FuzzNumericSanity", "negative-damping-overflow.arrp",
	)
	verifyCommittedReplay(t, replayPath, DecodeScript(data))
}

// updateReplays regenerates the committed regression replays. They pin the
// timeline of a fixed bug, so they legitimately move whenever the simulation
// or the script decoder changes on purpose — and then a stale fixture reports
// an anchor mismatch that reads like a physics regression. Regenerate with:
//
//	go test ./internal/simfuzz -run TestCommitted -update
//
// in a dedicated commit that says why the timeline moved. The durable guards
// for these two bugs live in scene/ballistics_test.go and scene/movement_test.go;
// these fixtures prove the failure stays reproducible end to end.
var updateReplays = flag.Bool("update", false, "regenerate committed regression replays")

func verifyCommittedReplay(t *testing.T, replayPath string, script Script) {
	t.Helper()
	if *updateReplays {
		rec, _ := recordScript(script)
		data, err := rec.MarshalBinary()
		if err != nil {
			t.Fatalf("encode committed replay %s: %v", replayPath, err)
		}
		if err := os.WriteFile(replayPath, data, 0o644); err != nil {
			t.Fatalf("write committed replay %s: %v", replayPath, err)
		}
		t.Logf("regenerated %s (%d bytes)", replayPath, len(data))
	}
	replayData, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read committed replay %s: %v", replayPath, err)
	}
	rec, err := runtime.ReadRecordingFromBytes(replayData)
	if err != nil {
		t.Fatalf("decode committed replay: %v", err)
	}
	if len(rec.Anchors) == 0 {
		t.Fatalf("committed replay %s has no checksum anchors", replayPath)
	}
	if err := runtime.VerifyRecording(NewGame(script), quietLog, rec); err != nil {
		t.Fatalf("verify committed replay: %v", err)
	}
	_, live := recordScript(script)
	if got := runtime.Replay(NewGame(script), quietLog, rec).Checksum(); got != live {
		t.Fatalf("committed replay checksum = %d, want live %d", got, live)
	}
}
