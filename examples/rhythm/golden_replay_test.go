package rhythm

import (
	"flag"
	"fmt"
	"testing"

	"github.com/dhannyell/arasy/internal/replaytest"
	"github.com/dhannyell/arasy/runtime"
)

var update = flag.Bool("update", false, "rewrite golden replay fixtures")

// TestGoldenReplay reuses runRhythmGameplay's scripted tap/hold/miss chart
// coverage (audio scheduling, holds, per-tick judging) and verifies it
// against a committed FFRP recording plus a fingerprint of the final
// MatchState.
func TestGoldenReplay(t *testing.T) {
	var last *Game
	newGame := func() runtime.Game {
		last = newTestGame(t)
		return last
	}
	fingerprint := func(game runtime.Game) string {
		return fmt.Sprintf("%+v", game.(*Game).State)
	}

	script := func(d *runtime.Driver) {
		runRhythmGameplay(t, d, last, nil)
	}

	replaytest.Golden(t, *update, "testdata", "rhythm", newGame, fingerprint, script)
}
