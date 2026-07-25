package runtime_test

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// tickPipelineSpy records what each Update/PostStep call observes through
// the public API, so these tests characterize the "Do not reorder" contract
// (AGENTS.md: input -> Update -> auto Step -> PostStep -> tick++) as
// externally observable behavior instead of asserting on driver internals.
type tickPipelineSpy struct {
	updateTick       []uint64
	updateStepTick   []uint64
	justPressedSeen  []bool
	pointerEventsLen []int
	postStepSeen     []bool
	postStepStepTick []uint64

	holdOnCall map[int]bool
}

func (g *tickPipelineSpy) Setup() runtime.Config {
	return runtime.Config{
		InputBindings: []input.Binding{
			{Source: input.Keyboard(input.KeySpace), Action: input.ActionJump, Scale: 1},
		},
	}
}

func (g *tickPipelineSpy) Init(ctx *runtime.UpdateContext) {}

func (g *tickPipelineSpy) Update(ctx *runtime.UpdateContext, dt float64) {
	call := len(g.updateTick)
	g.updateTick = append(g.updateTick, ctx.Tick)
	g.updateStepTick = append(g.updateStepTick, ctx.World.StepTick())
	g.justPressedSeen = append(g.justPressedSeen, ctx.Input.JustPressed(input.ActionJump))
	g.pointerEventsLen = append(g.pointerEventsLen, len(ctx.Pointer.Events))
	g.postStepSeen = append(g.postStepSeen, false)
	g.postStepStepTick = append(g.postStepStepTick, 0)

	if g.holdOnCall[call] {
		ctx.HoldSimulation()
	}
}

func (g *tickPipelineSpy) PostStep(ctx *runtime.UpdateContext, _ scene.StepStats) {
	call := len(g.updateTick) - 1
	g.postStepSeen[call] = true
	g.postStepStepTick[call] = ctx.World.StepTick()
}

func (g *tickPipelineSpy) Render(ctx *runtime.RenderContext) []render.Command { return nil }

var _ runtime.Game = (*tickPipelineSpy)(nil)
var _ runtime.PostStepper = (*tickPipelineSpy)(nil)

func keyboardEvent(code input.SourceCode, kind input.EventKind) input.Event {
	return input.Event{Source: input.Source{Device: input.DeviceKeyboard, Code: code}, Kind: kind}
}

func TestTickPipelineInputAppliedBeforeStepIsVisibleSameTick(t *testing.T) {
	game := &tickPipelineSpy{}
	driver := runtime.NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{keyboardEvent(input.KeySpace, input.EventButtonDown)})
	driver.Step()

	if !game.justPressedSeen[0] {
		t.Fatal("event applied via ApplyInput before Step was not visible to Update on the same tick")
	}
}

func TestTickPipelineUpdateRunsBeforeAutoStepAndPostStepAfter(t *testing.T) {
	game := &tickPipelineSpy{}
	driver := runtime.NewDriver(game, slog.Default())

	driver.Step()
	driver.Step()

	for call, want := range []uint64{0, 1} {
		if got := game.updateStepTick[call]; got != want {
			t.Fatalf("call %d: Update saw World.StepTick()=%d, want %d (Update must run before the auto Step)", call, got, want)
		}
	}
	for call, want := range []uint64{1, 2} {
		if !game.postStepSeen[call] {
			t.Fatalf("call %d: PostStep did not run", call)
		}
		if got := game.postStepStepTick[call]; got != want {
			t.Fatalf("call %d: PostStep saw World.StepTick()=%d, want %d (PostStep must run after the auto Step)", call, got, want)
		}
	}
}

func TestTickPipelinePostStepSkippedWhenSimulationHeld(t *testing.T) {
	game := &tickPipelineSpy{holdOnCall: map[int]bool{0: true}}
	driver := runtime.NewDriver(game, slog.Default())

	driver.Step()

	if game.postStepSeen[0] {
		t.Fatal("PostStep ran even though Update called ctx.HoldSimulation()")
	}
	if got := driver.State().Tick; got != 1 {
		t.Fatalf("tick after a held-simulation step = %d, want 1 (the tick still advances)", got)
	}
}

func TestTickPipelineCtxTickIsPreIncrementDriverTickIsPost(t *testing.T) {
	game := &tickPipelineSpy{}
	driver := runtime.NewDriver(game, slog.Default())

	driver.Step()
	driver.Step()

	for call, want := range []uint64{0, 1} {
		if got := game.updateTick[call]; got != want {
			t.Fatalf("call %d: ctx.Tick = %d, want %d (pre-increment)", call, got, want)
		}
	}
	if got := driver.State().Tick; got != 2 {
		t.Fatalf("driver tick after 2 steps = %d, want 2", got)
	}
}

func TestTickPipelinePointerFrameClearedBetweenTicks(t *testing.T) {
	game := &tickPipelineSpy{}
	driver := runtime.NewDriver(game, slog.Default())

	driver.ApplyInput([]input.Event{{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft},
		Kind:   input.EventButtonDown, X: 1, Y: 1,
	}})
	driver.Step()
	driver.Step()

	if game.pointerEventsLen[0] == 0 {
		t.Fatal("setup: pointer event was not visible on the tick it was applied")
	}
	if got := game.pointerEventsLen[1]; got != 0 {
		t.Fatalf("pointer event reappeared on the following tick: len = %d, want 0", got)
	}
}
