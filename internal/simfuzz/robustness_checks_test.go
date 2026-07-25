package simfuzz

import (
	"bytes"
	"testing"

	"github.com/dhannyell/arasy/runtime"
)

// checkNumericSanity runs P4: after every tick, no live entity may hold a NaN
// or Inf in any simulation component. The scan runs in PostStep (it has the
// World); input floats are forced finite by the decoder, so any non-finite
// value found here was produced by the simulation itself.
func checkNumericSanity(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	game := NewGame(script)
	driver := runtime.NewDriver(game, quietLog)
	byTick := groupEventsByTick(script.Events)
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
		if err := game.scanErr; err != nil {
			t.Fatalf("tick %d: %v\nrepro: %s",
				tick+1, err, dumpRepro(t, "FuzzNumericSanity", data))
		}
	}
}

// checkRecordingCodec runs P5: a recording parsed from arbitrary bytes must
// re-serialize stably — Read(bytes).Marshal() == Read(Read(bytes).Marshal()).Marshal().
// Invalid input returns an error (never panics) and is accepted.
func checkRecordingCodec(t testing.TB, data []byte) {
	t.Helper()
	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		return
	}
	out, err := rec.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v\nrepro: %s", err, dumpRepro(t, "FuzzRecordingCodec", data))
	}
	rec2, err := runtime.ReadRecordingFromBytes(out)
	if err != nil {
		t.Fatalf("re-read: %v\nrepro: %s", err, dumpRepro(t, "FuzzRecordingCodec", data))
	}
	out2, err := rec2.MarshalBinary()
	if err != nil {
		t.Fatalf("re-marshal: %v\nrepro: %s", err, dumpRepro(t, "FuzzRecordingCodec", data))
	}
	if !bytes.Equal(out, out2) {
		t.Fatalf("recording round-trip not stable\nrepro: %s",
			dumpRepro(t, "FuzzRecordingCodec", data))
	}
}

// checkNoPanic runs P6: the engine must not panic on arbitrary input. It runs
// the decoded (bound) event log, then a second pass with an arbitrary
// (possibly invalid) event stream built from the raw bytes. A panic is caught
// by the fuzz framework and fails the target.
func checkNoPanic(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	byTick := groupEventsByTick(script.Events)

	driver := runtime.NewDriver(NewGame(script), quietLog)
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
	}

	arbByTick := groupEventsByTick(decodeArbitraryEvents(data, script.MaxTicks))
	driver2 := runtime.NewDriver(NewGame(script), quietLog)
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver2.ApplyInput(arbByTick[uint64(tick)])
		driver2.Step()
	}
}
