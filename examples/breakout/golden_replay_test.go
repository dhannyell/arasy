package breakout

import (
	"flag"
	"fmt"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/internal/replaytest"
	"github.com/dhannyell/frameforge/runtime"
)

var update = flag.Bool("update", false, "rewrite golden replay fixtures")

// TestGoldenReplay drives ~1800 ticks (30s at 60Hz) of real breakout
// gameplay through the live API — click Play, launch the ball, sweep the
// paddle back and forth, auto-relaunch after every life lost — and verifies
// it against a committed FFRP recording plus a fingerprint of the final
// MatchState. Checksum anchors alone would catch most of this drift (World
// holds the ball/paddle/bricks), but the fingerprint also locks in
// score/lives, which the anchors only summarize as an opaque hash.
func TestGoldenReplay(t *testing.T) {
	var last *Breakout
	newGame := func() runtime.Game {
		last = &Breakout{}
		return last
	}
	fingerprint := func(game runtime.Game) string {
		return fmt.Sprintf("%+v", game.(*Breakout).State)
	}

	script := func(d *runtime.Driver) {
		// menuLayout() centers Play (and Play-Again/Game-Over) at (640, 510)
		// in the 1280x960 reference canvas, which is also this Config's
		// default viewport — no letterbox math needed (see
		// TestBreakoutScrubDeterministic for the math when it differs).
		const playX, playY = 640.0, 510.0
		const totalTicks = 1800
		const paddleWindow = 45
		paddleKeys := []input.SourceCode{input.KeyD, input.KeyA}

		// JustPressed only fires on a fresh down edge (Mapper compares
		// pressed-before vs pressed-after the tick's events), so Space must
		// be released before the next life's StateReady can re-trigger it —
		// otherwise the key latches "held" forever after the first launch
		// and the game gets stuck in StateReady for the rest of the run.
		releaseSpaceNextTick := false

		for tick := uint64(0); tick < totalTicks; tick++ {
			var events []input.Event
			if releaseSpaceNextTick {
				events = append(events, replayKeyUp(input.KeySpace))
				releaseSpaceNextTick = false
			}
			switch last.State.state {
			case StateMenu, StateGameOver, StateWon:
				events = append(events, mouseMove(playX, playY), mouseDown(playX, playY), mouseUp(playX, playY))
			case StateReady:
				events = append(events, replayKeyDown(input.KeySpace))
				releaseSpaceNextTick = true
			case StatePlaying:
				key := paddleKeys[(tick/paddleWindow)%2]
				if tick%paddleWindow == 0 {
					events = append(events, replayKeyDown(key))
				}
				if tick%paddleWindow == paddleWindow-1 {
					events = append(events, replayKeyUp(key))
				}
			}
			if len(events) > 0 {
				d.ApplyInput(events)
			}
			d.Step()
		}

		if last.State.score == 0 {
			t.Fatal("golden replay script produced no score; gameplay is not exercising the paddle/ball/bricks")
		}
	}

	replaytest.Golden(t, *update, "testdata", "breakout", newGame, fingerprint, script)
}

func replayKeyDown(code input.SourceCode) input.Event {
	return input.Event{Source: input.Source{Device: input.DeviceKeyboard, Code: code}, Kind: input.EventButtonDown}
}

func replayKeyUp(code input.SourceCode) input.Event {
	return input.Event{Source: input.Source{Device: input.DeviceKeyboard, Code: code}, Kind: input.EventButtonUp}
}
