package simfuzz

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/runtime"
)

var goldenReplayPaths = []string{
	filepath.Join("..", "..", "examples", "2048", "testdata", "2048.arrp"),
	filepath.Join("..", "..", "examples", "breakout", "testdata", "breakout.arrp"),
	filepath.Join("..", "..", "examples", "rhythm", "testdata", "rhythm.arrp"),
}

func addSimulationSeeds(f *testing.F, targetSeeds ...[]byte) {
	f.Helper()
	for _, seed := range targetSeeds {
		f.Add(seed)
	}

	f.Add(seedBytes(30))
	f.Add(seedBytes(60, CompTransform|CompVelocity|CompBody|CompCircleCollider))
	f.Add(seedBytes(60,
		CompTransform|CompVelocity|CompBody|CompCircleCollider,
		CompTransform|CompBody|CompRectCollider))
	f.Add(seedBytes(90, allComponents))
	f.Add(maxVelocitySeed())

	for _, path := range goldenReplayPaths {
		if _, rec, ok := readGoldenReplay(f, path); ok {
			f.Add(scriptSeedFromRecording(rec))
		}
	}
}

func addRecordingSeeds(f *testing.F) {
	f.Helper()
	for _, path := range goldenReplayPaths {
		if data, _, ok := readGoldenReplay(f, path); ok {
			f.Add(data)
		}
	}
}

// readGoldenReplay loads a demo golden replay for use as seed material. A
// failure is logged and skipped rather than fatal: seeding is best-effort, and
// TestGoldenReplaySeedsAreReadable owns that signal. Failing here instead
// meant a recording-format change took all twelve fuzz targets down at once —
// and with them `go test ./...` for the whole repository.
func readGoldenReplay(tb testing.TB, path string) ([]byte, runtime.Recording, bool) {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Logf("skipping golden replay seed %s: %v", path, err)
		return nil, runtime.Recording{}, false
	}
	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		tb.Logf("skipping golden replay seed %s: %v", path, err)
		return nil, runtime.Recording{}, false
	}
	return data, rec, true
}

// TestGoldenReplaySeedsAreReadable is the single, clearly named failure for a
// demo golden replay that stops being usable as seed material — a recording
// format change, a renamed demo, a regenerated fixture. It reports every
// broken path rather than stopping at the first.
func TestGoldenReplaySeedsAreReadable(t *testing.T) {
	for _, path := range goldenReplayPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read golden replay seed %s: %v", path, err)
			continue
		}
		if _, err := runtime.ReadRecordingFromBytes(data); err != nil {
			t.Errorf("decode golden replay seed %s: %v", path, err)
		}
	}
}

func maxVelocitySeed() []byte {
	mask := CompTransform | CompVelocity | CompAcceleration | CompBody | CompCircleCollider
	data := concatBytes(
		packF64(0), packF64(0), packF64(0), packF64(1), packF64(1),
		packF64(maxFuzzFloat), packF64(-maxFuzzFloat),
		packF64(maxFuzzFloat), packF64(-maxFuzzFloat),
		packF64(1), packF64(1),
		packF64(0.25),
	)
	return seedBytesWithData(120, mask, data)
}

func seedBytesWithData(ticks int, mask uint32, componentData []byte) []byte {
	ticks = clamp(ticks, 1, MaxTicks)
	buf := []byte{1, 0, 1, byte(ticks), byte(ticks >> 8)}
	buf = binary.LittleEndian.AppendUint32(buf, mask)
	want := compDataLen(mask)
	if len(componentData) > want {
		componentData = componentData[:want]
	}
	buf = append(buf, componentData...)
	if len(componentData) < want {
		buf = append(buf, make([]byte, want-len(componentData))...)
	}
	return buf
}

func scriptSeedFromRecording(rec runtime.Recording) []byte {
	ticks := min(max(int(rec.Ticks), 1), 255)
	buf := seedBytes(ticks, allComponents)

	events := make([]input.Event, 0, min(len(rec.Events), maxFuzzEvents))
	for _, event := range rec.Events {
		if event.Tick >= uint64(ticks) || len(events) == maxFuzzEvents {
			continue
		}
		events = append(events, event)
	}
	buf = append(buf, byte(len(events)))
	previous := uint64(0)
	for _, event := range events {
		sourceIndex := boundSourceIndex(event.Source)
		kind := byte(0)
		if event.Kind == input.EventButtonUp || event.Kind == input.EventPointerLeave {
			kind = 1
		}
		delta := min(event.Tick-previous, 255)
		previous = event.Tick
		buf = append(buf, sourceIndex, kind, byte(delta))
		buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(event.Value))
	}
	return buf
}

func boundSourceIndex(source input.Source) byte {
	for i, candidate := range fuzzBoundSources {
		if source == candidate {
			return byte(i)
		}
	}
	return byte(uint32(source.Code) % uint32(len(fuzzBoundSources)))
}
