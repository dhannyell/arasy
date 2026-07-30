package runtime

import (
	"log/slog"
	"math"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestReplayReproducesChecksum(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()

	for tick := range 300 {
		kind := input.EventButtonUp
		if tick%40 < 20 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
	}

	rec := driver.StopRecording()
	if len(rec.Events) == 0 {
		t.Fatal("recording captured no events")
	}
	if rec.Ticks != driver.State().Tick {
		t.Fatalf("recording.Ticks = %d, want %d", rec.Ticks, driver.State().Tick)
	}

	replay := Replay(&inputBackGame{}, slog.Default(), rec)

	if replay.State().Tick != driver.State().Tick {
		t.Fatalf("replay tick = %d, want %d", replay.State().Tick, driver.State().Tick)
	}
	if got, want := replay.Checksum(), driver.Checksum(); got != want {
		t.Fatalf("replay checksum = %d, want %d", got, want)
	}
}

func TestReplayUsesRecordedGamepadDeadzone(t *testing.T) {
	live := &deadzoneReplayGame{deadzone: .2}
	driver := NewDriver(live, slog.Default())
	driver.StartRecording()
	driver.ApplyInput([]input.Event{{Source: input.Gamepad(input.GamepadLeftStickX), Kind: input.EventAxisChanged, Value: .5}})
	for range 35 {
		driver.Step()
	}
	recording := driver.StopRecording()

	if recording.GamepadDeadzone != .2 {
		t.Fatalf("recorded deadzone = %v, want .2", recording.GamepadDeadzone)
	}

	// The fresh game intentionally has a different local setting. Replay must
	// use the value recorded with the input journal, not this machine's config.
	replayed := &deadzoneReplayGame{deadzone: .8}
	Replay(replayed, slog.Default(), recording)
	if math.Abs(replayed.lastValue-.375) > 1e-12 {
		t.Fatalf("replayed axis = %v, want .375 from recorded deadzone", replayed.lastValue)
	}

	viewed := &deadzoneReplayGame{deadzone: .8}
	ReplayForViewing(viewed, slog.Default(), recording)
	if math.Abs(viewed.lastValue-.375) > 1e-12 {
		t.Fatalf("viewing replay axis = %v, want .375 from recorded deadzone", viewed.lastValue)
	}

	if err := VerifyRecording(&deadzoneReplayGame{deadzone: .8}, slog.Default(), recording); err != nil {
		t.Fatalf("VerifyRecording with recorded deadzone: %v", err)
	}
}

func TestRecordingInvalidGamepadDeadzoneUsesFreshGameConfiguration(t *testing.T) {
	for name, deadzone := range map[string]float64{
		"zero":     0,
		"negative": -1,
		"one":      1,
		"nan":      math.NaN(),
	} {
		t.Run(name, func(t *testing.T) {
			game := &deadzoneReplayGame{deadzone: .8}
			Replay(game, slog.Default(), Recording{
				Ticks:           1,
				Seed:            7,
				GamepadDeadzone: deadzone,
				Events: []input.Event{{
					Source: input.Gamepad(input.GamepadLeftStickX),
					Kind:   input.EventAxisChanged,
					Value:  .5,
				}},
			})
			if game.lastValue != 0 {
				t.Fatalf("replayed axis = %v, want local .8 deadzone to suppress .5", game.lastValue)
			}
		})
	}
}

type deadzoneReplayGame struct {
	deadzone  float64
	lastValue float64
	player    scene.EntityID
}

func (g *deadzoneReplayGame) Setup() Config {
	return Config{
		SimulationHz:    60,
		GamepadDeadzone: g.deadzone,
		InputBindings:   []input.Binding{{Source: input.Gamepad(input.GamepadLeftStickX), Action: input.ActionMoveX, Scale: 1}},
	}
}

func (g *deadzoneReplayGame) Init(ctx *UpdateContext) {
	g.player = ctx.World.Spawn()
	ctx.World.SetTransform(g.player, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
}

func (g *deadzoneReplayGame) Update(ctx *UpdateContext, _ float64) {
	g.lastValue = ctx.Input.Value(input.ActionMoveX)
	transform, _ := ctx.World.Transform(g.player)
	transform.Position.X += g.lastValue
	ctx.World.SetTransform(g.player, transform)
}

func (g *deadzoneReplayGame) Render(*RenderContext) []render.Command { return nil }

func TestReplayDivergesWhenInputDiffers(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()
	for tick := range 60 {
		kind := input.EventButtonUp
		if tick >= 20 {
			kind = input.EventButtonDown
		}
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: kind},
		})
		driver.Step()
	}
	rec := driver.StopRecording()
	want := driver.Checksum()

	// Drop the event that presses D, so the replay never moves the player.
	tampered := Recording{Ticks: rec.Ticks}
	for _, e := range rec.Events {
		if e.Kind == input.EventButtonDown {
			continue
		}
		tampered.Events = append(tampered.Events, e)
	}

	replay := Replay(&inputBackGame{}, slog.Default(), tampered)
	if got := replay.Checksum(); got == want {
		t.Fatal("expected checksum to diverge when replayed with different input, but it matched")
	}
}

// TestResetRestartsActiveRecording: a reset restarts tick 0, so keeping the
// pre-reset events would collide with the new run's ticks — Replay would
// apply both runs' events on the same ticks and reproduce a state that never
// existed. The capture must restart with the run, keeping StopRecording
// reproducible.
func TestResetRestartsActiveRecording(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.StartRecording()

	// Pre-reset run: D held on ticks 0..4. If these events survived the
	// reset, the replay below would move the player from tick 0.
	for range 5 {
		driver.ApplyInput([]input.Event{
			{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
		})
		driver.Step()
	}

	driver.Control(ControlCommand{Kind: ControlReset})
	driver.AdvanceTime(0)

	// Post-reset run: idle on ticks 0..4 (where the stale events would
	// land), then a press/release pair.
	for tick := range 10 {
		switch tick {
		case 5:
			driver.ApplyInput([]input.Event{
				{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonDown},
			})
		case 8:
			driver.ApplyInput([]input.Event{
				{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD}, Kind: input.EventButtonUp},
			})
		}
		driver.Step()
	}

	rec := driver.StopRecording()
	if rec.Ticks != 10 {
		t.Fatalf("recording ticks = %d, want 10 (post-reset run only)", rec.Ticks)
	}

	replay := Replay(&inputBackGame{}, slog.Default(), rec)
	if got, want := replay.Checksum(), driver.Checksum(); got != want {
		t.Fatalf("replay checksum = %d, want %d — recording kept pre-reset events", got, want)
	}
}
