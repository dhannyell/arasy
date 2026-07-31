package runtime

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/render"
)

type inputDeviceGame struct {
	devices          []input.DeviceKind
	gamepadConnected bool
}

func (*inputDeviceGame) Setup() Config       { return Config{} }
func (*inputDeviceGame) Init(*UpdateContext) {}
func (g *inputDeviceGame) Update(ctx *UpdateContext, _ float64) {
	g.devices = append(g.devices, ctx.Input.LastInputDevice())
}
func (g *inputDeviceGame) Render(ctx *RenderContext) []render.Command {
	g.gamepadConnected = ctx.GamepadConnected
	return nil
}

func TestLastInputDeviceReplaysDeterministically(t *testing.T) {
	live := &inputDeviceGame{}
	driver := NewDriver(live, slog.Default())
	driver.StartRecording()

	events := [][]input.Event{
		{{Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeySpace}, Kind: input.EventButtonDown}},
		{{Source: input.Gamepad(input.GamepadA), Kind: input.EventButtonUp}},
		{{Source: input.Gamepad(input.GamepadA), Kind: input.EventButtonDown}},
	}
	for _, tickEvents := range events {
		driver.ApplyInput(tickEvents)
		driver.Step()
	}

	recording := driver.StopRecording()
	replayed := &inputDeviceGame{}
	Replay(replayed, slog.Default(), recording)

	if !reflect.DeepEqual(replayed.devices, live.devices) {
		t.Fatalf("replayed devices = %v, want %v", replayed.devices, live.devices)
	}
	want := []input.DeviceKind{
		input.DeviceKeyboard,
		input.DeviceKeyboard,
		input.DeviceGamepad,
	}
	if !reflect.DeepEqual(live.devices, want) {
		t.Fatalf("live devices = %v, want %v", live.devices, want)
	}
}

func TestGamepadConnectedIsRenderOnlyAndNotRestoredFromSnapshot(t *testing.T) {
	game := &inputDeviceGame{}
	driver := NewDriver(game, slog.Default())

	driver.SetGamepadConnected(true)
	driver.Frame(0)
	if !game.gamepadConnected {
		t.Fatal("render did not receive connected gamepad hint")
	}

	snapshot := driver.captureSnapshot()
	driver.SetGamepadConnected(false)
	driver.restoreSnapshot(snapshot)
	driver.Frame(0)
	if game.gamepadConnected {
		t.Fatal("snapshot restore changed presentation-only gamepad presence")
	}
}
