package simfuzz

import (
	"bytes"
	goruntime "runtime"
	"testing"

	"github.com/dhannyell/arasy/runtime"
)

// checkNumericSanity runs P4: after every tick, no live entity may hold a NaN
// or Inf in any simulation component. The scan runs in PostStep (it has the
// World). Component data and analog event values are both decoded into finite
// ranges (see axisValueFromBytes and the spawn readers), so any non-finite
// value found here was produced by the simulation itself rather than handed
// to it.
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

// maxParseAllocPerByte bounds how much the parser may allocate per byte of
// input, on top of a fixed floor. The header carries element counts that are
// not redundant with the payload length, so without this the target scores a
// rejected input as a pass no matter how much memory it reserved on the way to
// the error — which is exactly how a 66-byte header that reserved 381 MB
// survived here.
const (
	maxParseAllocFloor   = 512 << 10
	maxParseAllocPerByte = 512
)

// checkRecordingCodec runs P5: a recording parsed from arbitrary bytes must
// re-serialize stably — Read(bytes).Marshal() == Read(Read(bytes).Marshal()).Marshal().
// Invalid input must be rejected with an error rather than a panic, and must
// not allocate unboundedly relative to the input size.
func checkRecordingCodec(t testing.TB, data []byte) {
	t.Helper()
	rec, allocated, err := readRecordingMeasured(data)
	budget := uint64(maxParseAllocFloor) + uint64(len(data))*maxParseAllocPerByte
	if allocated > budget {
		t.Fatalf("parsing %d bytes allocated %d bytes, budget %d\nrepro: %s",
			len(data), allocated, budget, dumpRepro(t, "FuzzRecordingCodec", data))
	}
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

// readRecordingMeasured parses data and reports how many bytes the parse
// allocated. TotalAlloc is monotonic and unaffected by collection, so the
// delta is the real allocation volume rather than a live-heap sample.
func readRecordingMeasured(data []byte) (runtime.Recording, uint64, error) {
	var before, after goruntime.MemStats
	goruntime.ReadMemStats(&before)
	rec, err := runtime.ReadRecordingFromBytes(data)
	goruntime.ReadMemStats(&after)
	return rec, after.TotalAlloc - before.TotalAlloc, err
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
