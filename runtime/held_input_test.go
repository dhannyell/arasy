package runtime

import (
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// heldGame records how long an action stayed pressed, which is the symptom a
// player sees when a key gets stuck: the game keeps acting on it.
type heldGame struct {
	pressedTicks int
}

func (g *heldGame) Setup() Config {
	return Config{
		HistoryLen: 600,
		InputBindings: []input.Binding{
			{Source: input.Keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
			{Source: input.Gamepad(input.GamepadLeftStickX), Action: input.ActionMoveX, Scale: 1},
		},
	}
}

func (g *heldGame) Init(*UpdateContext) {}

func (g *heldGame) Update(ctx *UpdateContext, _ float64) {
	if ctx.Input.Pressed(input.ActionMoveX) {
		g.pressedTicks++
	}
}

func (g *heldGame) Render(*RenderContext) []render.Command { return nil }

func heldTestDriver(t *testing.T) (*Driver, *heldGame) {
	t.Helper()
	game := &heldGame{}
	driver := NewDriver(game, slog.New(slog.NewTextHandler(io.Discard, nil)))
	driver.SkipSplash()
	return driver, game
}

// actionPressed and actionValue read the action state the game would see.
func actionPressed(d *Driver) bool {
	state := d.input.LastInput()
	return state.Pressed(input.ActionMoveX)
}

func actionValue(d *Driver) float64 {
	state := d.input.LastInput()
	return state.Value(input.ActionMoveX)
}

func keyDown(code input.SourceCode) input.Event {
	return input.Event{Source: input.Keyboard(code), Kind: input.EventButtonDown}
}

func keyUp(code input.SourceCode) input.Event {
	return input.Event{Source: input.Keyboard(code), Kind: input.EventButtonUp}
}

// The bug this file exists for: a key pressed inside a recording is released
// at a tick further along the journal. Take control before that tick and the
// driver stops consuming the journal, so the release never arrives and the
// key stays down for the rest of the session.
func TestTakingControlReleasesAKeyHeldByTheRecording(t *testing.T) {
	driver, game := heldTestDriver(t)

	// Record a run where D goes down at tick 2 and comes up at tick 12.
	driver.StartRecording()
	for tick := range 20 {
		switch tick {
		case 2:
			driver.ApplyInput([]input.Event{keyDown(input.KeyD)})
		case 12:
			driver.ApplyInput([]input.Event{keyUp(input.KeyD)})
		}
		driver.Step()
	}
	recording := driver.StopRecording()

	watcher := ReplayForViewing(&heldGame{}, slog.New(slog.NewTextHandler(io.Discard, nil)), recording)

	// Watch up to tick 6, in the middle of the press.
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 6})
	watcher.AdvanceTime(0)
	if got := watcher.State().Tick; got != 6 {
		t.Fatalf("scrubbed to tick %d, want 6", got)
	}
	if !actionPressed(watcher) {
		t.Fatal("the reconstructed tick does not have the key down; the test is not at the press")
	}

	// Take control mid-press.
	watcher.Control(ControlCommand{Kind: ControlPlay})
	before := watcher.game.(*heldGame).pressedTicks
	for range 30 {
		watcher.Step()
	}

	if actionPressed(watcher) {
		t.Fatal("the key is still down 30 ticks after taking control")
	}
	if after := watcher.game.(*heldGame).pressedTicks; after != before {
		t.Fatalf("the game saw the action pressed for %d more ticks after taking control, want none",
			after-before)
	}
	_ = game
}

// The release is applied as input, not written into the input state, so it
// lands in the journal. A scrub back across the branch point must therefore
// rebuild the same world the live run produced — anything else diverges.
func TestReleaseOnTakingControlIsReproducibleByScrubbing(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.StartRecording()
	for tick := range 20 {
		if tick == 2 {
			driver.ApplyInput([]input.Event{keyDown(input.KeyD)})
		}
		if tick == 12 {
			driver.ApplyInput([]input.Event{keyUp(input.KeyD)})
		}
		driver.Step()
	}
	recording := driver.StopRecording()

	watcher := ReplayForViewing(&heldGame{}, slog.New(slog.NewTextHandler(io.Discard, nil)), recording)
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 6})
	watcher.AdvanceTime(0)

	watcher.Control(ControlCommand{Kind: ControlPlay})
	for range 10 {
		watcher.Step()
	}
	live := watcher.Checksum()
	liveTick := watcher.State().Tick

	// Rewind across the branch point and come back the long way.
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 3})
	watcher.AdvanceTime(0)
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: liveTick})
	watcher.AdvanceTime(0)

	if got := watcher.Checksum(); got != live {
		t.Fatalf("resimulated checksum %016x, want the live %016x", got, live)
	}
	if watcher.State().DeterminismViolated {
		t.Fatal("scrubbing across the branch point reported a divergence")
	}
}

// A stick left away from centre is the same bug with an analog face: the
// value would keep feeding the action forever.
func TestTakingControlRecentresAHeldStick(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.StartRecording()
	for tick := range 20 {
		if tick == 2 {
			driver.ApplyInput([]input.Event{{
				Source: input.Gamepad(input.GamepadLeftStickX),
				Kind:   input.EventAxisChanged,
				Value:  0.9,
			}})
		}
		if tick == 12 {
			driver.ApplyInput([]input.Event{{
				Source: input.Gamepad(input.GamepadLeftStickX),
				Kind:   input.EventAxisChanged,
			}})
		}
		driver.Step()
	}
	recording := driver.StopRecording()

	watcher := ReplayForViewing(&heldGame{}, slog.New(slog.NewTextHandler(io.Discard, nil)), recording)
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 6})
	watcher.AdvanceTime(0)
	if actionValue(watcher) == 0 {
		t.Fatal("the reconstructed tick has a centred stick; the test is not at the deflection")
	}

	watcher.Control(ControlCommand{Kind: ControlPlay})
	watcher.Step()

	if got := actionValue(watcher); got != 0 {
		t.Fatalf("stick still reads %v after taking control, want 0", got)
	}
}

// Stepping out of watch mode is the same gesture as playing out of it.
func TestSteppingOutOfWatchModeReleasesToo(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.StartRecording()
	for tick := range 20 {
		if tick == 2 {
			driver.ApplyInput([]input.Event{keyDown(input.KeyD)})
		}
		driver.Step()
	}
	recording := driver.StopRecording()

	watcher := ReplayForViewing(&heldGame{}, slog.New(slog.NewTextHandler(io.Discard, nil)), recording)
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 6})
	watcher.AdvanceTime(0)

	watcher.Control(ControlCommand{Kind: ControlStep})
	watcher.AdvanceTime(0)
	watcher.Step()

	if actionPressed(watcher) {
		t.Fatal("stepping out of watch mode left the key down")
	}
}

// Watching itself must not release anything: while the driver is consuming
// the journal, a key the recording holds is a key that is genuinely held.
func TestWatchingKeepsTheRecordingsPressesHeld(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.StartRecording()
	for tick := range 20 {
		if tick == 2 {
			driver.ApplyInput([]input.Event{keyDown(input.KeyD)})
		}
		if tick == 12 {
			driver.ApplyInput([]input.Event{keyUp(input.KeyD)})
		}
		driver.Step()
	}
	recording := driver.StopRecording()

	watcher := ReplayForViewing(&heldGame{}, slog.New(slog.NewTextHandler(io.Discard, nil)), recording)
	watcher.Control(ControlCommand{Kind: ControlScrubToTick, Tick: 0})
	watcher.AdvanceTime(0)

	watcher.Control(ControlCommand{Kind: ControlReplayPlay})
	for range 6 {
		watcher.Step()
	}

	if !actionPressed(watcher) {
		t.Fatal("watching released a press the recording was still holding")
	}
}

// Held state is part of the input state, so it has to survive a rewind: a
// snapshot that dropped it would report a key as released at a tick where the
// recording had it down.
func TestHeldInputSurvivesSnapshotRestore(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.ApplyInput([]input.Event{keyDown(input.KeyD)})
	driver.Step()
	snapshot := driver.input.Snapshot()

	driver.ApplyInput([]input.Event{keyUp(input.KeyD)})
	driver.Step()
	if len(driver.input.held) != 0 {
		t.Fatal("the release did not clear the held set")
	}

	driver.input.Restore(cloneDriverInputSnapshot(snapshot))
	if len(driver.input.held) != 1 {
		t.Fatalf("restored %d held sources, want the one that was down", len(driver.input.held))
	}
	if !driver.input.keyboard.Down(input.KeyD) {
		t.Fatal("restoring lost the held key")
	}
}

func TestHeldSetOrdersAndDeduplicates(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.ApplyInput([]input.Event{
		keyDown(input.KeyZ),
		keyDown(input.KeyA),
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft}, Kind: input.EventButtonDown},
		keyDown(input.KeyA),
	})

	held := driver.input.held
	if len(held) != 3 {
		t.Fatalf("held %d sources, want 3 (the repeated press is one)", len(held))
	}
	for i := 1; i < len(held); i++ {
		if !heldBefore(held[i-1].source, held[i].source) {
			t.Fatalf("held set is out of order at %d: %v then %v", i, held[i-1].source, held[i].source)
		}
	}

	events := driver.input.releaseEvents()
	if len(events) != 3 {
		t.Fatalf("built %d release events, want 3", len(events))
	}
	for _, event := range events {
		if event.Kind != input.EventButtonUp {
			t.Fatalf("release for %v is %v, want a button up", event.Source, event.Kind)
		}
	}
}

// A wheel notch is an impulse, not a position. Treating it as held would make
// taking control emit a release for something nobody is holding.
func TestWheelIsNotHeld(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.ApplyInput([]input.Event{{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelY},
		Kind:   input.EventAxisChanged,
		Value:  3,
	}})

	if len(driver.input.held) != 0 {
		t.Fatalf("wheel left %d sources held, want none", len(driver.input.held))
	}
}

// The pointer leaving the surface drops its buttons, so the held set has to
// follow — otherwise taking control would release a button already gone.
func TestPointerLeaveClearsHeldMouseButtons(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft}, Kind: input.EventButtonDown},
		keyDown(input.KeyD),
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseUnknown}, Kind: input.EventPointerLeave},
	})

	if len(driver.input.held) != 1 {
		t.Fatalf("held %d sources after the pointer left, want just the key", len(driver.input.held))
	}
	if driver.input.held[0].source.Device != input.DeviceKeyboard {
		t.Fatalf("held source is %v, want the keyboard press", driver.input.held[0].source)
	}
}

// Mouse releases carry the pointer position, like every other mouse event, so
// a widget resolving the release lands where the pointer actually is.
func TestMouseReleaseEventsCarryThePointerPosition(t *testing.T) {
	driver, _ := heldTestDriver(t)

	driver.ApplyInput([]input.Event{{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonDown,
		X:      120,
		Y:      45,
	}})

	events := driver.input.releaseEvents()
	if len(events) != 1 {
		t.Fatalf("built %d release events, want 1", len(events))
	}
	if got := (math2d.Vec2{X: events[0].X, Y: events[0].Y}); got != (math2d.Vec2{X: 120, Y: 45}) {
		t.Fatalf("release at %v, want the pointer position", got)
	}
}
