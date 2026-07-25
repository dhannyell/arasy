package simfuzz

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/runtime"
)

// checkResetReproducibility runs P7: a ControlReset mid-run must rebuild the
// world deterministically. The per-tick checksums after the reset must be
// identical to those of a fresh driver run from the same seed and input,
// because doReset reseeds the RNG, rebuilds World/input/UI/audio and calls Init.
// Any nondeterminism in that reconstruction (map iteration, un-reseeded RNG,
// Init order) diverges here.
func checkResetReproducibility(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	if script.MaxTicks < 4 {
		t.Skip("script too short for mid-run reset")
	}
	byTick := groupEventsByTick(script.Events)

	// Reference: a straight run collecting per-tick checksums.
	ref := make([]uint64, 0, script.MaxTicks+1)
	refDriver := runtime.NewDriver(NewGame(script), quietLog)
	ref = append(ref, refDriver.Checksum())
	for tick := 0; tick < script.MaxTicks; tick++ {
		refDriver.ApplyInput(byTick[uint64(tick)])
		refDriver.Step()
		ref = append(ref, refDriver.Checksum())
	}

	// Reset mid-run, then replay the full input log from tick 0. The post-reset
	// checksums must match the reference from the same tick index.
	mid := script.MaxTicks / 2
	driver := runtime.NewDriver(NewGame(script), quietLog)
	for tick := range mid {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
	}
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReset})
	driver.Step() // consume the reset tick (rebuilds the world, stays at tick 0)

	post := make([]uint64, 0, script.MaxTicks+1)
	post = append(post, driver.Checksum())
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.ApplyInput(byTick[uint64(tick)])
		driver.Step()
		post = append(post, driver.Checksum())
	}

	// The post-reset tick-0 checksum must equal the reference tick-0 checksum.
	if got, want := post[0], ref[0]; got != want {
		t.Fatalf("post-reset tick 0 checksum %d != reference %d\nrepro: %s",
			got, want, dumpRepro(t, "FuzzResetReproducibility", data))
	}
	// Every post-reset tick must match the reference at the same index.
	for i := 1; i < len(post); i++ {
		if post[i] != ref[i] {
			t.Fatalf("post-reset tick %d checksum %d != reference %d\nrepro: %s",
				i, post[i], ref[i], dumpRepro(t, "FuzzResetReproducibility", data))
		}
	}
}

// checkScrubBidirectional runs P8: repeated scrub jumps (N -> K1 -> K2 -> ... ->
// N) must each land on the exact original checksum, exercising the keyframe
// archive thinning path that a single round-trip (P3) never reaches. The jump
// targets are derived from the script bytes so the fuzzer can explore the space.
func checkScrubBidirectional(t testing.TB, data []byte) {
	t.Helper()
	script := DecodeScript(data)
	driver, _, checksums := runHistoryDriver(script)
	n := len(checksums) - 1
	if n < 30 {
		t.Skip("script too short to exercise archived keyframes")
	}
	if driver.State().HistoryLen == 0 {
		t.Fatalf("history game captured no dense keyframes\nrepro: %s",
			dumpRepro(t, "FuzzScrubBidirectional", data))
	}
	targets := decodeScrubTargets(data, n)
	for _, target := range targets {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)
		if got := driver.State().Tick; got != target {
			t.Fatalf("scrub to %d: tick = %d\nrepro: %s",
				target, got, dumpRepro(t, "FuzzScrubBidirectional", data))
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum %d != original %d\nrepro: %s",
				target, got, want, dumpRepro(t, "FuzzScrubBidirectional", data))
		}
	}
}

// checkDespawnMidRun runs P10: input triggers World.Despawn mid-run; the run
// must stay deterministic (two runs identical per tick), NaN-free after the
// despawn, and a rewind across the despawn tick must converge to the live
// checksum. This is the fuzz coverage for the Despawn path (removal from every
// component store) that the other targets never exercise.
func checkDespawnMidRun(t testing.TB, data []byte) {
	t.Helper()
	script := appendDrivenEntity(DecodeScript(data))
	if script.MaxTicks < 8 {
		t.Skip("script too short for despawn scenario")
	}

	// Force a cleanup/down edge so fuzzed Space state cannot turn the target
	// into a no-op.
	jumpTick := uint64(2)
	despawnEvents := withoutSpaceEvents(script.Events)
	despawnEvents = append(despawnEvents, input.Event{
		Tick:   jumpTick - 1,
		Source: input.Keyboard(input.KeySpace),
		Kind:   input.EventButtonUp,
	})
	despawnEvents = append(despawnEvents, input.Event{
		Tick:   jumpTick,
		Source: input.Keyboard(input.KeySpace),
		Kind:   input.EventButtonDown,
	})
	despawnByTick := groupEventsByTick(despawnEvents)

	// Two independent runs with the despawn trigger must match per tick.
	runA := func() (*runtime.Driver, *Game, []uint64) {
		game := newHistoryGame(script)
		driver := runtime.NewDriver(game, quietLog)
		cs := make([]uint64, 0, script.MaxTicks+1)
		cs = append(cs, driver.Checksum())
		for tick := 0; tick < script.MaxTicks; tick++ {
			driver.ApplyInput(despawnByTick[uint64(tick)])
			driver.Step()
			if tick == int(jumpTick) {
				if game.State.DespawnCount != 1 {
					t.Fatalf("despawn target ran %d times, want exactly one\nrepro: %s",
						game.State.DespawnCount, dumpRepro(t, "FuzzDespawnMidRun", data))
				}
			}
			if err := game.scanErr; err != nil {
				t.Fatalf("tick %d: %v\nrepro: %s", tick+1, err, dumpRepro(t, "FuzzDespawnMidRun", data))
			}
			cs = append(cs, driver.Checksum())
		}
		return driver, game, cs
	}
	_, _, a := runA()
	driverB, _, b := runA()
	if len(a) != len(b) {
		t.Fatalf("despawn tick count diverged: %d != %d\nrepro: %s",
			len(a), len(b), dumpRepro(t, "FuzzDespawnMidRun", data))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("despawn determinism diverged at tick %d: %d != %d\nrepro: %s",
				i, a[i], b[i], dumpRepro(t, "FuzzDespawnMidRun", data))
		}
	}

	// Rewind across the despawn: scrub back before jumpTick, then forward to
	// the end. The driver replays the journal (including the despawn trigger),
	// so the final checksum must match the live run.
	n := len(a) - 1
	driverB.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: jumpTick - 1})
	driverB.AdvanceTime(0)
	driverB.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: uint64(n)})
	driverB.AdvanceTime(0)
	if got, want := driverB.Checksum(), a[n]; got != want {
		t.Fatalf("rewind across despawn diverged: %d != %d\nrepro: %s",
			got, want, dumpRepro(t, "FuzzDespawnMidRun", data))
	}
}

// checkForkAfterScrubDeterminism runs P12: scrubbing back to K and applying NEW
// input forks the timeline; the forked future must equal a clean run that used
// the base input 0..K then the new input K..N. A bug in timeline.fork or
// recording truncation (keeping a stale future event) makes them diverge.
func checkForkAfterScrubDeterminism(t testing.TB, data []byte) {
	t.Helper()
	script := appendDrivenEntity(DecodeScript(data))
	script.Events = withoutSpaceEvents(script.Events)
	script.Events = filterEventsBySource(script.Events, input.Keyboard(input.KeyArrowRight))
	if script.MaxTicks < 8 {
		t.Skip("script too short for fork scenario")
	}
	baseByTick := groupEventsByTick(script.Events)
	forkEvents := decodeForkEvents(data, script.MaxTicks)
	forkByTick := groupEventsByTick(forkEvents)
	n := script.MaxTicks
	k := n / 2

	// Reference: base input 0..k-1, then fork input k..n-1.
	ref := runtime.NewDriver(newHistoryGame(script), quietLog)
	for tick := range n {
		ev := baseByTick[uint64(tick)]
		if tick >= k {
			ev = forkByTick[uint64(tick)]
		}
		ref.ApplyInput(ev)
		ref.Step()
	}
	want := ref.Checksum()

	// Forked: base input 0..n-1, scrub back to k, then fork input k..n-1.
	base := runtime.NewDriver(newHistoryGame(script), quietLog)
	for tick := range n {
		base.ApplyInput(baseByTick[uint64(tick)])
		base.Step()
	}
	baseChecksum := base.Checksum()

	forked := runtime.NewDriver(newHistoryGame(script), quietLog)
	for tick := range n {
		forked.ApplyInput(baseByTick[uint64(tick)])
		forked.Step()
	}
	forked.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: uint64(k)})
	forked.AdvanceTime(0)
	for tick := k; tick < n; tick++ {
		forked.ApplyInput(forkByTick[uint64(tick)])
		forked.Step()
	}
	if got := forked.Checksum(); got != want {
		t.Fatalf("fork after scrub diverged: %d != %d (k=%d n=%d)\nrepro: %s",
			got, want, k, n, dumpRepro(t, "FuzzForkAfterScrubDeterminism", data))
	}
	// A fork that reproduces the abandoned future proves nothing, but it is a
	// property of the generated input, not an engine defect: the fork stream
	// can legitimately re-derive the base state. Skip rather than fail, so the
	// target reports only real divergence.
	if want == baseChecksum {
		t.Skip("fork input did not change the abandoned future")
	}
}
