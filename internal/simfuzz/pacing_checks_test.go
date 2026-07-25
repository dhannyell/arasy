package simfuzz

import (
	"testing"
	"time"

	"github.com/dhannyell/arasy/runtime"
)

// checkSpeedIndependence runs P9 through AdvanceTime, the only path where speed
// is meaningful. It compares every emitted tick against a Step-driven reference
// while the second driver changes speed and accumulates wall-clock fragments.
func checkSpeedIndependence(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	if script.MaxTicks < 8 {
		t.Skip("script too short for speed independence")
	}
	byTick := groupEventsByTick(script.Events)

	const ticks = 40
	limit := min(ticks, script.MaxTicks)

	reference := make([]uint64, 1, limit+1)
	base := runtime.NewDriver(NewGame(script), quietLog)
	reference[0] = base.Checksum()
	for tick := range limit {
		base.ApplyInput(byTick[uint64(tick)])
		base.Step()
		reference = append(reference, base.Checksum())
	}

	speeds := []float64{0.25, 0.5, 1, 2, 4, 8}
	timed := runtime.NewDriver(NewGame(script), quietLog)
	appliedTick := uint64(^uint64(0))
	const wallQuantum = 2 * time.Millisecond
	for timed.State().Tick < uint64(limit) {
		tick := timed.State().Tick
		if appliedTick != tick {
			timed.Control(runtime.ControlCommand{
				Kind:  runtime.ControlSetSpeed,
				Speed: speeds[int(tick)%len(speeds)],
			})
			timed.ApplyInput(byTick[tick])
			appliedTick = tick
		}
		stats := timed.AdvanceTime(wallQuantum)
		if stats.Steps > 1 {
			t.Fatalf("wall quantum advanced %d ticks; speed oracle requires at most one\nrepro: %s",
				stats.Steps, dumpRepro(t, "FuzzSpeedIndependence", data))
		}
		if stats.Steps == 1 {
			gotTick := timed.State().Tick
			if got, want := timed.Checksum(), reference[gotTick]; got != want {
				t.Fatalf("speed-dependent divergence at tick %d: %d != %d\nrepro: %s",
					gotTick, got, want, dumpRepro(t, "FuzzSpeedIndependence", data))
			}
		}
	}
}
