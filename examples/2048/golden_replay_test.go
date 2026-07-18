package twenty48

import (
	"flag"
	"fmt"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/internal/replaytest"
	"github.com/dhannyell/frameforge/runtime"
)

var update = flag.Bool("update", false, "rewrite golden replay fixtures")

// TestGoldenReplay drives real 2048 gameplay through the live API — click
// Play, cycle through the four move directions to generate merges and
// runtime.Rand-driven spawns, click through Game Over / Win overlays to
// keep playing — and verifies it against a committed FFRP recording plus a
// fingerprint of the final matchState.
//
// This is the case checksum anchors alone do NOT cover: 2048 keeps no
// entities in World (Driver.Checksum() hashes World only), so a divergent
// board/score is invisible to the anchors — the fingerprint carries the
// entire signal here.
func TestGoldenReplay(t *testing.T) {
	var last *Game2048
	newGame := func() runtime.Game {
		last = &Game2048{}
		return last
	}
	fingerprint := func(game runtime.Game) string {
		return fmt.Sprintf("%+v", game.(*Game2048).state.State)
	}

	script := func(d *runtime.Driver) {
		play := overlayPrimaryButtonRect().Center() // same rect: Play, Play Again, and Continue
		click := func() {
			d.ApplyInput([]input.Event{mouseMove(play.X, play.Y), mouseDown(play.X, play.Y), mouseUp(play.X, play.Y)})
			d.Step()
		}
		click() // phaseMenu -> phasePlaying

		moves := []input.SourceCode{input.KeyArrowLeft, input.KeyArrowUp, input.KeyArrowRight, input.KeyArrowDown}
		for i := 0; i < 60; i++ {
			key := moves[i%len(moves)]
			d.ApplyInput([]input.Event{keyEvent(key, input.EventButtonDown)})
			d.Step()
			d.ApplyInput([]input.Event{keyEvent(key, input.EventButtonUp)})
			d.Step()
			for range int(animationTicks) + 1 {
				d.Step()
			}
			if last.state.State.phase == phaseGameOver || last.state.State.phase == phaseWon {
				click()
			}
		}

		if last.state.State.score == 0 {
			t.Fatal("golden replay script never scored a merge")
		}
	}

	replaytest.Golden(t, *update, "testdata", "2048", newGame, fingerprint, script)
}
