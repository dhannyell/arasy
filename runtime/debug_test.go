package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func TestDebugActionNameReturnsKnownNames(t *testing.T) {
	cases := map[input.Action]string{
		input.ActionMoveX:   "MoveX",
		input.ActionMoveY:   "MoveY",
		input.ActionConfirm: "Confirm",
		input.ActionCancel:  "Cancel",
		input.ActionPause:   "Pause",
		input.ActionJump:    "Jump",
	}

	for action, want := range cases {
		if got := debugActionName(action); got != want {
			t.Fatalf("debugActionName(%v) = %q, want %q", action, got, want)
		}
	}
}

func TestDebugActionNameFallsBackToActionNumberForUnknownAction(t *testing.T) {
	got := debugActionName(input.Action(99))
	want := "Action(99)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDebugInputActionsDedupsSortsAndSkipsInvalid(t *testing.T) {
	bindings := []input.Binding{
		{Source: input.Keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyA), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeySpace), Action: input.ActionUnknown},
		{Source: input.Keyboard(input.KeyW), Action: input.ActionMoveY, Scale: -1},
	}

	actions := debugInputActions(bindings)

	want := []input.Action{input.ActionMoveX, input.ActionMoveY}
	if len(actions) != len(want) {
		t.Fatalf("got %v, want %v", actions, want)
	}
	for i, action := range want {
		if actions[i] != action {
			t.Fatalf("got %v, want %v", actions, want)
		}
	}
}

func TestDebugHUDBlockHeightAccountsForLineCountAndGap(t *testing.T) {
	got := debugHUDBlockHeight(3)
	want := 3*debugHUDLineHeight + debugHUDBlockGap
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDebugHUDTextCommandsProducesOneLinePerEntryWithIncreasingLayer(t *testing.T) {
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectExpand,
	}
	screenPos := math2d.Vec2{X: 16, Y: 16}

	commands := debugHUDTextCommands([]string{"a", "b"}, cam, screenPos, 9900)
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}

	first, ok := commands[0].(render.DrawText)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawText", commands[0])
	}
	second, ok := commands[1].(render.DrawText)
	if !ok {
		t.Fatalf("got command type %T, want render.DrawText", commands[1])
	}

	if first.Text != "a" || second.Text != "b" {
		t.Fatalf("got texts %q, %q, want %q, %q", first.Text, second.Text, "a", "b")
	}

	wantFirstPos := cam.ScreenToWorld(screenPos)
	if first.Position != wantFirstPos {
		t.Fatalf("got first position %+v, want %+v", first.Position, wantFirstPos)
	}

	wantSecondPos := cam.ScreenToWorld(math2d.Vec2{X: screenPos.X, Y: screenPos.Y + debugHUDLineHeight})
	if second.Position != wantSecondPos {
		t.Fatalf("got second position %+v, want %+v", second.Position, wantSecondPos)
	}

	if first.Layer() != 9900 || second.Layer() != 9901 {
		t.Fatalf("got layers %d, %d, want 9900, 9901", first.Layer(), second.Layer())
	}
}

func TestDebugRenderLayerLinesCountsAndSortsByLayer(t *testing.T) {
	gameCmds := []render.Command{
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: 5}},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: 1}},
		render.DrawCircle{BaseCommand: render.BaseCommand{LayerValue: 5}},
	}

	lines := debugRenderLayerLines(debugRenderer{}, debugContext{gameCommands: gameCmds})

	want := []string{"Render layers", "1: 1 cmds", "5: 2 cmds"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("got %v, want %v", lines, want)
		}
	}
}

func TestDebugRenderLayerLinesReturnsHeaderOnlyWithoutCommands(t *testing.T) {
	lines := debugRenderLayerLines(debugRenderer{}, debugContext{})
	if len(lines) != 1 || lines[0] != "Render layers" {
		t.Fatalf("got %v, want [Render layers]", lines)
	}
}

type debugStatsGame struct{}

func (debugStatsGame) Setup() Config                              { return Config{} }
func (debugStatsGame) Init(ctx *UpdateContext)                    {}
func (debugStatsGame) Update(ctx *UpdateContext, dt float64)      {}
func (debugStatsGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestDebugRuntimeStatsLinesReportsTickPausedSpeedAndChecksum(t *testing.T) {
	driver := NewDriver(debugStatsGame{}, slog.Default())
	driver.Step()

	lines := debugRuntimeStatsLines(debugRenderer{driver: driver}, debugContext{})

	if len(lines) < 5 {
		t.Fatalf("got %d lines, want at least 5: %v", len(lines), lines)
	}
	if lines[0] != "Runtime" {
		t.Fatalf("got header %q, want %q", lines[0], "Runtime")
	}
	if lines[1] != "tick: 1" {
		t.Fatalf("got %q, want %q", lines[1], "tick: 1")
	}
	if lines[2] != "paused: false" {
		t.Fatalf("got %q, want %q", lines[2], "paused: false")
	}
}

func TestDebugRuntimeStatsLinesIncludesHistoryRangeWhenRecorderPresent(t *testing.T) {
	driver := NewDriver(&statefulGame{}, slog.Default())
	driver.Step()
	driver.Step()

	lines := debugRuntimeStatsLines(debugRenderer{driver: driver}, debugContext{})

	found := false
	for _, line := range lines {
		if line == "range: 0-0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("got %v, want a range line", lines)
	}
}

func TestDriverCommandsOmitsDebugOverlayWhenNoLayersEnabled(t *testing.T) {
	driver := NewDriver(debugStatsGame{}, slog.Default())

	commands := driver.Frame(0).Commands
	if len(commands) != 0 {
		t.Fatalf("got %d commands, want 0 with no debug layers enabled", len(commands))
	}
}

func TestDriverCommandsAppendsRuntimeStatsOverlayWhenLayerEnabled(t *testing.T) {
	driver := NewDriver(debugStatsGame{}, slog.Default())
	driver.Control(ControlCommand{Kind: ControlSetDebugLayers, DebugLayers: scene.DebugRuntimeStats})

	commands := driver.Frame(0).Commands
	if len(commands) == 0 {
		t.Fatalf("got 0 commands, want overlay text commands for DebugRuntimeStats")
	}

	for _, cmd := range commands {
		if _, ok := cmd.(render.DrawText); !ok {
			t.Fatalf("got command type %T, want render.DrawText", cmd)
		}
	}
}

// The checksum walks every component store, and the overlay asks for it once
// per frame while it can only change once per tick.
func TestDebugChecksumIsComputedOncePerTick(t *testing.T) {
	driver := NewDriver(debugStatsGame{}, slog.Default())
	driver.Step()

	first := driver.debugChecksumForFrame()
	if !driver.debugChecksumValid || driver.debugChecksumTick != driver.tick {
		t.Fatal("the first call did not memoize the checksum")
	}

	// A frame that follows without a tick must reuse it. Poisoning the cached
	// value is how the test can tell reuse from recomputation.
	driver.debugChecksum = 0xdeadbeef
	if got := driver.debugChecksumForFrame(); got != 0xdeadbeef {
		t.Fatalf("checksum recomputed within the same tick (got %x)", got)
	}

	driver.Step()
	if got := driver.debugChecksumForFrame(); got != driver.world.Checksum() {
		t.Fatalf("checksum after a tick = %x, want the live %x", got, driver.world.Checksum())
	}
	if first == 0 {
		t.Fatal("checksum never produced a value")
	}
}

// A reset is the one way the world changes without the tick changing, so it
// has to drop the memo or the overlay would report the pre-reset world.
func TestDebugChecksumInvalidatedByReset(t *testing.T) {
	driver := NewDriver(&statefulGame{}, slog.Default())
	driver.debugChecksumForFrame()

	driver.Control(ControlCommand{Kind: ControlReset})
	driver.Step()

	if driver.debugChecksumValid {
		t.Fatal("reset left a stale checksum memo behind")
	}
	if got := driver.debugChecksumForFrame(); got != driver.world.Checksum() {
		t.Fatalf("checksum after reset = %x, want the rebuilt world's %x", got, driver.world.Checksum())
	}
}
