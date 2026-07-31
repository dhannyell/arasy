//go:build windows && amd64

package native

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordingPortLoadsTheNewestFile(t *testing.T) {
	port := &recordingPort{dir: t.TempDir()}

	write := func(name string, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(port.dir, name), []byte(data), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	write("arasy-replay-20260101-120000.arrp", "older")
	write("arasy-replay-20260101-130000.arrp", "newer")
	// A file that is not a recording must not win just by sorting last.
	write("zzz-notes.txt", "ignore me")

	data, name, ok, err := port.Load()
	if err != nil || !ok {
		t.Fatalf("Load = (ok=%v, err=%v), want a file", ok, err)
	}
	if string(data) != "newer" {
		t.Fatalf("loaded %q, want the newest recording", data)
	}
	if !strings.HasSuffix(name, ".arrp") {
		t.Fatalf("loaded %q, want a .arrp", name)
	}
}

// Nothing recorded yet is an ordinary state, not a failure: the panel just
// has nothing to offer.
func TestRecordingPortLoadIsQuietWhenEmpty(t *testing.T) {
	empty := &recordingPort{dir: t.TempDir()}
	if _, _, ok, err := empty.Load(); ok || err != nil {
		t.Fatalf("empty directory Load = (ok=%v, err=%v), want nothing and no error", ok, err)
	}

	missing := &recordingPort{dir: filepath.Join(t.TempDir(), "never-created")}
	if _, _, ok, err := missing.Load(); ok || err != nil {
		t.Fatalf("missing directory Load = (ok=%v, err=%v), want nothing and no error", ok, err)
	}
}

func TestRecordingPortSaveCreatesTheDirectoryAndRoundTrips(t *testing.T) {
	port := &recordingPort{dir: filepath.Join(t.TempDir(), "recordings")}

	if err := port.Save("arasy-replay.arrp", []byte("payload")); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, _, ok, err := port.Load()
	if err != nil || !ok {
		t.Fatalf("Load after Save = (ok=%v, err=%v), want the saved file", ok, err)
	}
	if string(data) != "payload" {
		t.Fatalf("round-tripped %q, want %q", data, "payload")
	}
}

// Two recordings from one session must not overwrite each other, and the
// later one has to sort last so Load picks it.
func TestTimestampedNameSortsChronologically(t *testing.T) {
	base := time.Date(2026, 7, 30, 9, 5, 0, 0, time.UTC)
	first := timestampedName("arasy-replay.arrp", base)
	second := timestampedName("arasy-replay.arrp", base.Add(time.Minute))

	if first == second {
		t.Fatal("two saves a minute apart produced the same name")
	}
	if !(first < second) {
		t.Fatalf("%q does not sort before %q", first, second)
	}
	if filepath.Ext(second) != ".arrp" {
		t.Fatalf("%q lost its extension", second)
	}
}
