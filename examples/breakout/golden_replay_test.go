package breakout

import (
	"flag"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/replaytest"
	"github.com/dhannyell/arasy/runtime"
)

var update = flag.Bool("update", false, "rewrite golden replay fixtures")

// TestGoldenReplay drives ~1800 ticks (30s at 60Hz) of real breakout
// gameplay through the live API — click Play, launch the ball, sweep the
// paddle back and forth, auto-relaunch after every life lost — and verifies
// it against a committed ARRP recording plus a fingerprint of the final
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
		paddleAxes := []float64{.6, -.6}

		// JustPressed only fires on a fresh down edge (Mapper compares
		// pressed-before vs pressed-after the tick's events), so the launch button must
		// be released before the next life's StateReady can re-trigger it —
		// otherwise the button latches "held" forever after the first launch
		// and the game gets stuck in StateReady for the rest of the run.
		releaseLaunchNextTick := false

		for tick := uint64(0); tick < totalTicks; tick++ {
			var events []input.Event
			if releaseLaunchNextTick {
				events = append(events, replayGamepadUp(input.GamepadA))
				releaseLaunchNextTick = false
			}
			switch last.State.state {
			case StateMenu, StateGameOver, StateWon:
				events = append(events, mouseMove(playX, playY), mouseDown(playX, playY), mouseUp(playX, playY))
			case StateReady:
				events = append(events, replayGamepadDown(input.GamepadA))
				releaseLaunchNextTick = true
			case StatePlaying:
				if tick%paddleWindow == 0 {
					// A non-saturated analog value makes the recording exercise
					// the deterministic deadzone and rescaling path.
					events = append(events, replayGamepadAxis(
						input.GamepadLeftStickX,
						paddleAxes[(tick/paddleWindow)%2],
					))
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

	if !*update {
		assertGoldenHasAnalogGamepadAxis(t, "testdata/breakout.arrp")
	}
	replaytest.Golden(t, *update, "testdata", "breakout", newGame, fingerprint, script)
	if *update {
		assertGoldenHasAnalogGamepadAxis(t, "testdata/breakout.arrp")
	}
}

func replayGamepadDown(code input.SourceCode) input.Event {
	return input.Event{Source: input.Gamepad(code), Kind: input.EventButtonDown}
}

func replayGamepadUp(code input.SourceCode) input.Event {
	return input.Event{Source: input.Gamepad(code), Kind: input.EventButtonUp}
}

func replayGamepadAxis(code input.SourceCode, value float64) input.Event {
	return input.Event{Source: input.Gamepad(code), Kind: input.EventAxisChanged, Value: value}
}

func assertGoldenHasAnalogGamepadAxis(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read analog golden guard %s: %v", path, err)
	}
	rec, err := runtime.ReadRecordingFromBytes(data)
	if err != nil {
		t.Fatalf("decode analog golden guard %s: %v", path, err)
	}
	for _, event := range rec.Events {
		magnitude := math.Abs(event.Value)
		if event.Source.Device == input.DeviceGamepad &&
			event.Source.Code == input.GamepadLeftStickX &&
			event.Kind == input.EventAxisChanged &&
			magnitude > 0 && magnitude < 1 {
			return
		}
	}
	t.Fatalf("golden %s has no non-saturated gamepad axis event", path)
}
