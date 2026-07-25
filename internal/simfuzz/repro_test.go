package simfuzz

import (
	"bytes"
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

func verifyCommittedReplay(t *testing.T, replayPath string, script Script) {
	t.Helper()
	replayData, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("read committed replay %s: %v", replayPath, err)
	}
	rec, err := runtime.ReadRecordingFromBytes(replayData)
	if err != nil {
		t.Fatalf("decode committed replay: %v", err)
	}
	if err := runtime.VerifyRecording(NewGame(script), quietLog, rec); err != nil {
		t.Fatalf("verify committed replay: %v", err)
	}
	_, live := recordScript(script)
	if got := runtime.Replay(NewGame(script), quietLog, rec).Checksum(); got != live {
		t.Fatalf("committed replay checksum = %d, want live %d", got, live)
	}
}
