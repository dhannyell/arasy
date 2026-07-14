package twenty48

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/runtime"
)

// mouseMove, mouseDown, mouseUp build pointer events in screen-pixel space,
// matching what a browser host forwards.
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

func keyEvent(code input.SourceCode, kind input.EventKind) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceKeyboard, Code: code},
		Kind:   kind,
	}
}

func TestTwenty48PrimaryMouseDragMovesTile(t *testing.T) {
	game := &Game2048{}
	driver := runtime.NewDriver(game, slog.Default())
	game.state.State = matchState{phase: phasePlaying}
	game.state.State.board[1] = 2

	cam := game.Setup().Camera
	start := cam.WorldToScreen(cellCenter(1))
	end := cam.WorldToScreen(cellCenter(1).Sub(math2d.Vec2{X: dragThreshold + 1}))

	driver.ApplyInput([]input.Event{mouseDown(start.X, start.Y)})
	driver.Step()
	if game.state.State.board[1] != 2 {
		t.Fatal("mouse press moved a tile before a drag")
	}

	driver.ApplyInput([]input.Event{
		mouseMove(end.X, end.Y),
		mouseUp(end.X, end.Y),
	})
	driver.Step()

	if game.state.State.board[0] != 2 {
		t.Fatalf("left drag did not move tile to index 0: board = %v", game.state.State.board)
	}
}

// TestTwenty48ScrubDeterministic replays the full user flow — click Play,
// make several moves, let each turn animation run out — then scrubs the
// timeline and verifies every jump restores the exact original game state.
//
// With an empty scene.World the driver checksum is constant, so unlike the
// breakout scrub test this one asserts determinism by comparing matchState
// snapshots directly. The board is the observable projection of the driver
// RNG: if a spawn diverged during replay, the board — and therefore the
// snapshot comparison — would diverge with it.
func TestTwenty48ScrubDeterministic(t *testing.T) {
	game := &Game2048{}
	driver := runtime.NewDriver(game, slog.Default())

	// A viewport wider than the 720x720 reference mimics a real browser
	// surface: the reference canvas is AspectFit-letterboxed, so UI buttons
	// land at different screen pixels than in the reference. The scrubber
	// must thread this viewport into the replay; if it doesn't, the click
	// hit-test diverges and the reconstructed state is wrong.
	driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 720}))

	// AspectFit of the 720x720 reference into 1280x720 keeps scale 1 and
	// offsets x by (1280-720)/2 = 280, y by 0.
	play := overlayPrimaryButtonRect().Center()
	playX, playY := play.X+280, play.Y

	snapshots := make([]matchState, 0, 128)
	snapshots = append(snapshots, game.state.State) // tick 0 (menu)

	// Click Play (move + down + up in the same tick).
	driver.ApplyInput([]input.Event{
		mouseMove(playX, playY),
		mouseDown(playX, playY),
		mouseUp(playX, playY),
	})
	driver.Step()
	snapshots = append(snapshots, game.state.State)

	if game.state.State.phase != phasePlaying {
		t.Fatalf("after Play click: phase = %v, want phasePlaying (%v)", game.state.State.phase, phasePlaying)
	}

	// Slow half: one keypress per turn, released on the next tick, then
	// enough idle ticks for the turn animation to expire on its own.
	slowKeys := []input.SourceCode{
		input.KeyArrowLeft, input.KeyArrowUp, input.KeyArrowRight, input.KeyArrowDown,
	}
	for _, key := range slowKeys {
		driver.ApplyInput([]input.Event{keyEvent(key, input.EventButtonDown)})
		driver.Step()
		snapshots = append(snapshots, game.state.State)

		driver.ApplyInput([]input.Event{keyEvent(key, input.EventButtonUp)})
		driver.Step()
		snapshots = append(snapshots, game.state.State)

		for range int(animationTicks) + 1 {
			driver.Step()
			snapshots = append(snapshots, game.state.State)
		}
	}

	// Fast half: moves at a two-tick cadence, well inside the animation
	// window, so each valid move replaces the previous in-flight animation.
	fastKeys := []input.SourceCode{
		input.KeyArrowLeft, input.KeyArrowUp, input.KeyArrowRight, input.KeyArrowDown,
	}
	interrupted := false
	for _, key := range fastKeys {
		if game.state.State.animation.Active {
			interrupted = true
		}
		driver.ApplyInput([]input.Event{keyEvent(key, input.EventButtonDown)})
		driver.Step()
		snapshots = append(snapshots, game.state.State)

		driver.ApplyInput([]input.Event{keyEvent(key, input.EventButtonUp)})
		driver.Step()
		snapshots = append(snapshots, game.state.State)
	}
	if !interrupted {
		t.Fatal("fast half never pressed a key while an animation was in flight")
	}

	if game.state.State.board == snapshots[1].board {
		t.Fatal("no move changed the board; the flow under test never played a turn")
	}

	// Scrub targets must include a tick captured mid-animation so the
	// by-value animation state inside matchState is restored too.
	midAnimation := uint64(0)
	for tick, snap := range snapshots {
		if snap.animation.Active {
			midAnimation = uint64(tick)
			break
		}
	}
	if midAnimation == 0 {
		t.Fatal("no snapshot captured an active turn animation")
	}

	// Targets cover both halves: 55 and 58 land inside the fast half
	// (ticks 54-61), where moves interrupted each other.
	lastTick := uint64(len(snapshots) - 1)
	targets := []uint64{40, 9, midAnimation, lastTick, 1, 55, 0, 58}

	// best is a high-water mark kept outside the StateSlot: replaying from
	// tick 0 re-runs Updates on the same game value, so a backward scrub
	// must never lower it below the value reached by the original run.
	bestAfterRun := game.best

	for _, target := range targets {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: target})
		driver.AdvanceTime(0)

		if driver.State().Tick != target {
			t.Fatalf("scrub to %d: tick = %d, want %d", target, driver.State().Tick, target)
		}
		if game.state.State != snapshots[target] {
			t.Fatalf("scrub to %d: divergent state\n got  %+v\n want %+v", target, game.state.State, snapshots[target])
		}
		if game.best != bestAfterRun {
			t.Fatalf("scrub to %d: best = %d, want high-water mark %d", target, game.best, bestAfterRun)
		}
	}
}
