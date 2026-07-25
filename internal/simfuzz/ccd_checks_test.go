package simfuzz

import (
	"testing"

	"github.com/dhannyell/arasy/runtime"
)

// checkSubstepTunneling runs P11: a shaped scenario (fast projectile +
// Ballistics.Substeps>1 + thin static wall) drives the continuous-collision
// path. It must stay NaN-free every tick, never cross the wall, and reproduce
// each per-tick checksum.
func checkSubstepTunneling(t testing.TB, data []byte) {
	t.Helper()
	scenario := decodeSubstepScenario(data)
	script := scenario.Script
	game := NewGame(script)
	driver := runtime.NewDriver(game, quietLog)
	checksums := make([]uint64, 0, script.MaxTicks+1)
	checksums = append(checksums, driver.Checksum())
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver.Step()
		if err := game.scanErr; err != nil {
			t.Fatalf("substep tick %d: %v\nrepro: %s",
				tick+1, err, dumpRepro(t, "FuzzSubstepTunneling", data))
		}
		checksums = append(checksums, driver.Checksum())
		if !game.State.HasFirstTransform {
			t.Fatalf("substep projectile disappeared at tick %d\nrepro: %s",
				tick+1, dumpRepro(t, "FuzzSubstepTunneling", data))
		}
		if game.State.FirstTransform.Position.X > scenario.WallX {
			t.Fatalf("substep projectile tunneled at tick %d: x=%f wall=%f\nrepro: %s",
				tick+1, game.State.FirstTransform.Position.X, scenario.WallX,
				dumpRepro(t, "FuzzSubstepTunneling", data))
		}
	}

	game2 := NewGame(script)
	driver2 := runtime.NewDriver(game2, quietLog)
	for tick := 0; tick < script.MaxTicks; tick++ {
		driver2.Step()
		if err := game2.scanErr; err != nil {
			t.Fatalf("substep run2 tick %d: %v\nrepro: %s",
				tick+1, err, dumpRepro(t, "FuzzSubstepTunneling", data))
		}
		if got, want := driver2.Checksum(), checksums[tick+1]; got != want {
			t.Fatalf("substep determinism divergence at tick %d: %d != %d\nrepro: %s",
				tick+1, got, want, dumpRepro(t, "FuzzSubstepTunneling", data))
		}
	}
}
