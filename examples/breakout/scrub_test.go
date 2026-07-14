package breakout

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/runtime"
)

// mouseMove, mouseDown, mouseUp build pointer events in screen-pixel space,
// matching what the web canvas forwards from the browser.
func mouseMove(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove, X: x, Y: y,
	}
}

func mouseDown(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonDown, X: x, Y: y,
	}
}

func mouseUp(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonUp, X: x, Y: y,
	}
}

func keySpaceDown() input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeySpace},
		Kind:   input.EventButtonDown,
	}
}

// TestBreakoutScrubDeterministic reproduces the full user flow — click Play,
// press Space to launch the ball, let it bounce — then scrubs the timeline
// and verifies every jump lands on the exact original checksum.  This is the
// same pattern the runtime scrub tests use, but driven through the real
// Breakout game so UI-gated state transitions (Menu → Ready → Playing) are
// exercised end-to-end.
func TestBreakoutScrubDeterministic(t *testing.T) {
	game := &Breakout{}
	driver := runtime.NewDriver(game, slog.Default())

	// Use a viewport that differs from the config default (1280×960) to mimic
	// a real browser canvas — the reference canvas is AspectFit-letterboxed
	// into 1920×1080, so the Play button lands at different screen pixels.
	// The scrubber must thread this viewport into the replay; if it doesn't,
	// the click hit-test diverges and the reconstructed state is wrong.
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}))

	// menuLayout() centres the panel at (640, 480) and the Play button at
	// (640, 510) in the 1280×960 reference canvas.  With AspectFit into
	// 1920×1080 (16:9 vs 4:3), the reference fits by height: scale =
	// 1080/960 = 1.125, width = 1280*1.125 = 1440, x-offset = (1920-1440)/2
	// = 240.  Play centre in screen px: (240 + 640*1.125, 510*1.125) =
	// (240 + 720, 573.75) = (960, 573.75).
	const playX, playY = 960.0, 573.75

	// Record the checksum at every tick from 0 onward.
	checksums := make([]uint64, 0, 53)
	checksums = append(checksums, driver.Checksum()) // tick 0

	// Tick 0 — click Play (move + down + up in the same tick).
	driver.ApplyInput([]input.Event{
		mouseMove(playX, playY),
		mouseDown(playX, playY),
		mouseUp(playX, playY),
	})
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 1

	if game.State.state != StateReady {
		t.Fatalf("after Play click: state = %v, want StateReady", game.State.state)
	}

	// Tick 1 — press Space to launch the ball.
	driver.ApplyInput([]input.Event{keySpaceDown()})
	driver.Step()
	checksums = append(checksums, driver.Checksum()) // tick 2

	if game.State.state != StatePlaying {
		t.Fatalf("after Space: state = %v, want StatePlaying", game.State.state)
	}

	// Run 50 more ticks.
	for range 50 {
		driver.Step()
		checksums = append(checksums, driver.Checksum())
	}

	// Scrub backward, forward, and backward again — every jump must land on
	// the exact original checksum.
	for _, target := range []uint64{40, 10, 50, 5, 30, 2, 0, 1} {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)

		if driver.State().Tick != target {
			t.Fatalf("scrub to %d: tick = %d, want %d", target, driver.State().Tick, target)
		}
		if got, want := driver.Checksum(), checksums[target]; got != want {
			t.Fatalf("scrub to %d: checksum = %d, want %d (divergent scrub)", target, got, want)
		}
	}
}
