package runtime

import (
	"log/slog"
	"testing"
	"time"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
)

type characterizationGame struct {
	updates int
}

func (g *characterizationGame) Setup() Config                         { return Config{} }
func (g *characterizationGame) Init(ctx *UpdateContext)               {}
func (g *characterizationGame) Update(ctx *UpdateContext, dt float64) { g.updates++ }
func (g *characterizationGame) Render(ctx *RenderContext) []render.Command {
	return nil
}

func TestDriverCharacterizationFractionalSpeedAccumulatesTicks(t *testing.T) {
	driver := NewDriver(&characterizationGame{}, slog.Default())

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 0.5})

	for i, wantTick := range []uint64{0, 1, 1, 2} {
		driver.AdvanceTime(time.Second/60 + time.Nanosecond)
		if driver.State().Tick != wantTick {
			t.Fatalf("tick after advance %d = %v, want %v", i+1, driver.State().Tick, wantTick)
		}
	}
}

func TestDriverCharacterizationAcceleratedSpeedRunsMultipleTicks(t *testing.T) {
	driver := NewDriver(&characterizationGame{}, slog.Default())

	driver.Control(ControlCommand{Kind: ControlSetSpeed, Speed: 2.0})
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)

	if driver.State().Tick != 2 {
		t.Fatalf("tick after 2x speed advance = %v, want 2", driver.State().Tick)
	}
}

func TestDriverCharacterizationPauseAndManualStep(t *testing.T) {
	driver := NewDriver(&characterizationGame{}, slog.Default())

	driver.Control(ControlCommand{Kind: ControlPause})
	driver.AdvanceTime(time.Second/60 + time.Nanosecond)
	if driver.State().Tick != 0 {
		t.Fatalf("tick while paused = %v, want 0", driver.State().Tick)
	}

	driver.Control(ControlCommand{Kind: ControlStep})
	driver.AdvanceTime(0)
	if driver.State().Tick != 1 {
		t.Fatalf("tick after manual step = %v, want 1", driver.State().Tick)
	}
}

func TestDriverCharacterizationResetConsumesAdvance(t *testing.T) {
	game := &characterizationGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step()
	if driver.State().Tick != 1 || game.updates != 1 {
		t.Fatalf("before reset tick=%v updates=%v, want tick=1 updates=1", driver.State().Tick, game.updates)
	}

	driver.Control(ControlCommand{Kind: ControlReset})
	driver.Step()
	if driver.State().Tick != 0 {
		t.Fatalf("tick after reset advance = %v, want 0", driver.State().Tick)
	}
	if game.updates != 1 {
		t.Fatalf("updates after reset advance = %v, want 1", game.updates)
	}

	driver.Step()
	if driver.State().Tick != 1 || game.updates != 2 {
		t.Fatalf("after post-reset advance tick=%v updates=%v, want tick=1 updates=2", driver.State().Tick, game.updates)
	}
}

func TestDriverCharacterizationBackRestoresTickAndChecksum(t *testing.T) {
	game := &inputBackGame{}
	driver := NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	})
	driver.Step()

	wantTick := driver.State().Tick
	wantChecksum := driver.world.Checksum()

	driver.Step()
	if driver.State().Tick != 2 {
		t.Fatalf("tick before back = %v, want 2", driver.State().Tick)
	}
	if checksum := driver.world.Checksum(); checksum == wantChecksum {
		t.Fatalf("checksum should change after second advance, still %v", checksum)
	}

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)

	if driver.State().Tick != wantTick {
		t.Fatalf("tick after back = %v, want %v", driver.State().Tick, wantTick)
	}
	if checksum := driver.world.Checksum(); checksum != wantChecksum {
		t.Fatalf("checksum after back = %v, want %v", checksum, wantChecksum)
	}
}
