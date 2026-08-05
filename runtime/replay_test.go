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

// The reason anchors hash SimChecksum: retuning authored visuals must not
// invalidate a recording. The divergence surfaces as a content warning on
// load, never as an anchor failure.
func TestVerifyRecordingSurvivesSpriteRecolor(t *testing.T) {
	live := &recolorGame{color: render.RGBA(1, 0, 0, 1)}
	driver := NewDriver(live, slog.Default())
	driver.StartRecording()
	for range 120 {
		driver.Step()
	}
	rec := driver.StopRecording()
	if len(rec.Anchors) == 0 {
		t.Fatal("recording captured no anchors")
	}
	if rec.ContentChecksum == 0 {
		t.Fatal("recording captured no content fingerprint")
	}

	recolored := &recolorGame{color: render.RGBA(0, 1, 0, 1)}
	if err := VerifyRecording(recolored, slog.Default(), rec); err != nil {
		t.Fatalf("a sprite recolor failed verification: %v", err)
	}
}

// A sprite driven by a tween is overwritten every tick, so recoloring its
// authored value is free too: the tween carries the deterministic clock and
// sits in the anchor, the sprite it writes does not. This is the dynamic
// half of the recolor guarantee above.
func TestVerifyRecordingSurvivesRecolorUnderTween(t *testing.T) {
	live := &recolorGame{color: render.RGBA(1, 0, 0, 1), tween: true}
	driver := NewDriver(live, slog.Default())
	driver.StartRecording()
	for range 120 {
		driver.Step()
	}
	rec := driver.StopRecording()

	recolored := &recolorGame{color: render.RGBA(0, 1, 0, 1), tween: true}
	if err := VerifyRecording(recolored, slog.Default(), rec); err != nil {
		t.Fatalf("a recolor under an active tween failed verification: %v", err)
	}
}

// The recording's simulation rate is authoritative, like Seed: a .arrp
// captured at 60 Hz must verify even after the game's config moves to
// 120 Hz, because each replayed Step must integrate dt = 1/60 like the live
// run did. Without the override the physics advance at the live rate and the
// first post-step anchor diverges.
func TestVerifyRecordingUsesRecordedSimulationRate(t *testing.T) {
	live := &rateGame{hz: 60}
	driver := NewDriver(live, slog.Default())
	driver.StartRecording()
	for range 90 {
		driver.Step()
	}
	rec := driver.StopRecording()
	if rec.SimulationHz != 60 {
		t.Fatalf("recording rate = %d, want 60", rec.SimulationHz)
	}

	retuned := &rateGame{hz: 120}
	if err := VerifyRecording(retuned, slog.Default(), rec); err != nil {
		t.Fatalf("a SimulationHz retune failed verification: %v", err)
	}
}

// rateGame moves an entity by velocity, so its positions depend on the fixed
// dt and any simulation-rate mismatch shows up in the checksum anchors.
type rateGame struct {
	hz int
}

func (g *rateGame) Setup() Config { return Config{SimulationHz: g.hz} }

func (g *rateGame) Init(ctx *UpdateContext) {
	e := ctx.World.Spawn()
	ctx.World.SetTransform(e, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(e, scene.Velocity{Linear: math2d.Vec2{X: 3, Y: 1}})
}

func (g *rateGame) Update(ctx *UpdateContext, _ float64) {}

func (g *rateGame) Render(ctx *RenderContext) []render.Command { return nil }

type recolorGame struct {
	color  render.Color
	tween  bool
	sprite scene.EntityID
}

func (g *recolorGame) Setup() Config { return Config{SimulationHz: 60} }

func (g *recolorGame) Init(ctx *UpdateContext) {
	g.sprite = ctx.World.Spawn()
	ctx.World.SetTransform(g.sprite, scene.Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(g.sprite, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})
	ctx.World.SetCircleSprite(g.sprite, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: 1, Color: g.color},
		Radius:     2,
	})
	if g.tween {
		ctx.World.SetCircleSpriteTween(g.sprite, scene.CircleSpriteTween{
			From:     scene.CircleSprite{Radius: 2},
			To:       scene.CircleSprite{Radius: 4},
			Duration: 5,
		})
	}
}

func (g *recolorGame) Update(ctx *UpdateContext, _ float64) {}

func (g *recolorGame) Render(ctx *RenderContext) []render.Command { return nil }

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
