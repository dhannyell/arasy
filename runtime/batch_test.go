package runtime_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// batchGame lets tests control exactly which batch operations happen in
// Init, Update and Render.
type batchGame struct {
	historyLen int
	initFn     func(ctx *runtime.UpdateContext)
	updateFn   func(ctx *runtime.UpdateContext)
	renderFn   func(ctx *runtime.RenderContext)
}

func (g *batchGame) Setup() runtime.Config { return runtime.Config{HistoryLen: g.historyLen} }
func (g *batchGame) Init(ctx *runtime.UpdateContext) {
	if g.initFn != nil {
		g.initFn(ctx)
	}
}
func (g *batchGame) Update(ctx *runtime.UpdateContext, dt float64) {
	if g.updateFn != nil {
		g.updateFn(ctx)
	}
}
func (g *batchGame) Render(ctx *runtime.RenderContext) []render.Command {
	if g.renderFn != nil {
		g.renderFn(ctx)
	}
	return nil
}

func spriteInstance(x float64) render.SpriteInstance {
	return render.SpriteInstance{
		Texture: 1,
		Dest:    math2d.NewRect(math2d.Vec2{X: x, Y: 0}, math2d.Vec2{X: 8, Y: 8}),
	}
}

func TestBatchReplaceConsumeDrain(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(7, []render.SpriteInstance{spriteInstance(1), spriteInstance(2)})
		ctx.Batches.UpdateText(3, []render.TextLabel{{Position: math2d.Vec2{X: 4, Y: 5}, Text: "hi"}})
	}

	driver.Frame(0)

	updates := driver.ConsumeFrameUpdates()
	if len(updates.SpriteBatches) != 1 || len(updates.TextBatches) != 1 {
		t.Fatalf("updates = %d sprites / %d text, want 1/1", len(updates.SpriteBatches), len(updates.TextBatches))
	}
	sprites := updates.SpriteBatches[0]
	if sprites.ID != 7 || sprites.Op != runtime.BatchReplace || len(sprites.Instances) != 2 {
		t.Fatalf("sprite update = %+v, want id 7 replace with 2 instances", sprites)
	}
	text := updates.TextBatches[0]
	if text.ID != 3 || text.Op != runtime.BatchReplace || len(text.Labels) != 1 || text.Labels[0].Text != "hi" {
		t.Fatalf("text update = %+v, want id 3 replace with label %q", text, "hi")
	}

	if more := driver.ConsumeFrameUpdates(); len(more.SpriteBatches) != 0 || len(more.TextBatches) != 0 {
		t.Fatalf("second consume = %d sprites / %d text, want 0/0 (pending must be drained)",
			len(more.SpriteBatches), len(more.TextBatches))
	}
}

func TestBatchReplaceEmptyIsNotDelete(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(1, nil)
	}

	driver.Frame(0)

	updates := driver.ConsumeFrameUpdates()
	if len(updates.SpriteBatches) != 1 {
		t.Fatalf("updates = %d, want 1 (empty replace is still a replace)", len(updates.SpriteBatches))
	}
	if updates.SpriteBatches[0].Op != runtime.BatchReplace {
		t.Fatalf("op = %d, want BatchReplace; nil content never means delete", updates.SpriteBatches[0].Op)
	}
	if len(updates.SpriteBatches[0].Instances) != 0 {
		t.Fatalf("instances = %d, want 0", len(updates.SpriteBatches[0].Instances))
	}
}

func TestBatchDeleteCollapsesReplace(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(2, []render.SpriteInstance{spriteInstance(1)})
		ctx.Batches.DeleteSprites(2)
	}

	driver.Frame(0)

	updates := driver.ConsumeFrameUpdates()
	if len(updates.SpriteBatches) != 1 {
		t.Fatalf("updates = %d, want 1 (same-frame ops collapse)", len(updates.SpriteBatches))
	}
	if updates.SpriteBatches[0].Op != runtime.BatchDelete {
		t.Fatalf("op = %d, want BatchDelete (last operation wins)", updates.SpriteBatches[0].Op)
	}
	if updates.SpriteBatches[0].Instances != nil {
		t.Fatalf("delete carries content: %+v", updates.SpriteBatches[0].Instances)
	}
}

func TestBatchLastOperationWins(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.DeleteText(5)
		ctx.Batches.UpdateText(5, []render.TextLabel{{Text: "first"}})
		ctx.Batches.UpdateText(5, []render.TextLabel{{Text: "last"}})
	}

	driver.Frame(0)

	updates := driver.ConsumeFrameUpdates()
	if len(updates.TextBatches) != 1 {
		t.Fatalf("updates = %d, want 1", len(updates.TextBatches))
	}
	got := updates.TextBatches[0]
	if got.Op != runtime.BatchReplace || len(got.Labels) != 1 || got.Labels[0].Text != "last" {
		t.Fatalf("update = %+v, want replace with %q (last operation wins)", got, "last")
	}
}

func TestBatchConsumeSortedByID(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(3, nil)
		ctx.Batches.UpdateSprites(1, nil)
		ctx.Batches.UpdateSprites(2, nil)
		ctx.Batches.UpdateText(9, nil)
		ctx.Batches.UpdateText(4, nil)
	}

	driver.Frame(0)

	updates := driver.ConsumeFrameUpdates()
	wantSprites := []render.SpriteBatchID{1, 2, 3}
	if len(updates.SpriteBatches) != len(wantSprites) {
		t.Fatalf("sprite updates = %d, want %d", len(updates.SpriteBatches), len(wantSprites))
	}
	for i, want := range wantSprites {
		if updates.SpriteBatches[i].ID != want {
			t.Fatalf("sprite order[%d] = %d, want %d", i, updates.SpriteBatches[i].ID, want)
		}
	}
	wantText := []render.TextBatchID{4, 9}
	for i, want := range wantText {
		if updates.TextBatches[i].ID != want {
			t.Fatalf("text order[%d] = %d, want %d", i, updates.TextBatches[i].ID, want)
		}
	}
}

func TestBatchPortCopiesInput(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	instances := []render.SpriteInstance{spriteInstance(1)}
	labels := []render.TextLabel{{Text: "before"}}
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(1, instances)
		ctx.Batches.UpdateText(1, labels)
	}

	driver.Frame(0)
	instances[0].Texture = 99
	labels[0].Text = "after"

	updates := driver.ConsumeFrameUpdates()
	if got := updates.SpriteBatches[0].Instances[0].Texture; got != 1 {
		t.Fatalf("stored texture = %d, want 1 (game slices are copied on publish)", got)
	}
	if got := updates.TextBatches[0].Labels[0].Text; got != "before" {
		t.Fatalf("stored label = %q, want %q (game slices are copied on publish)", got, "before")
	}
}

func TestBatchPublishFromInit(t *testing.T) {
	game := &batchGame{
		initFn: func(ctx *runtime.UpdateContext) {
			ctx.Batches.UpdateSprites(11, []render.SpriteInstance{spriteInstance(3)})
		},
	}
	driver := runtime.NewDriver(game, testLogger())

	updates := driver.ConsumeFrameUpdates()
	if len(updates.SpriteBatches) != 1 || updates.SpriteBatches[0].ID != 11 {
		t.Fatalf("updates = %+v, want the batch published by Init", updates.SpriteBatches)
	}
}

func TestBatchPublishFromUpdate(t *testing.T) {
	game := &batchGame{
		updateFn: func(ctx *runtime.UpdateContext) {
			ctx.Batches.UpdateText(12, []render.TextLabel{{Text: "tick"}})
		},
	}
	driver := runtime.NewDriver(game, testLogger())

	driver.Step()

	updates := driver.ConsumeFrameUpdates()
	if len(updates.TextBatches) != 1 || updates.TextBatches[0].ID != 12 {
		t.Fatalf("updates = %+v, want the batch published by Update", updates.TextBatches)
	}
}

// captureRenderer records the order and content of FrameRenderer calls.
type captureRenderer struct {
	calls   []string
	updates runtime.FrameUpdates
}

func (r *captureRenderer) UpdateResources(u runtime.FrameUpdates) {
	r.calls = append(r.calls, "updates")
	r.updates = u
}
func (r *captureRenderer) Render(f runtime.Frame) { r.calls = append(r.calls, "render") }

func TestBatchPublishFromRenderArrivesSameFrame(t *testing.T) {
	game := &batchGame{
		renderFn: func(ctx *runtime.RenderContext) {
			ctx.Batches.UpdateSprites(21, []render.SpriteInstance{spriteInstance(9)})
		},
	}
	driver := runtime.NewDriver(game, testLogger())
	r := &captureRenderer{}

	driver.RenderFrame(0, r)

	if len(r.calls) != 2 || r.calls[0] != "updates" || r.calls[1] != "render" {
		t.Fatalf("calls = %v, want [updates render]", r.calls)
	}
	if len(r.updates.SpriteBatches) != 1 || r.updates.SpriteBatches[0].ID != 21 {
		t.Fatalf("updates = %+v, want the batch published in Render to arrive in the same frame", r.updates.SpriteBatches)
	}
}

// TestBatchPendingSurvivesRewind guards the render-only boundary: batch
// publications must never be part of the rewind snapshot, so a Back between
// publish and consume must neither drop nor duplicate them.
func TestBatchPendingSurvivesRewind(t *testing.T) {
	game := &batchGame{historyLen: 4}
	driver := runtime.NewDriver(game, testLogger())
	driver.Step()
	driver.Step()

	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(31, []render.SpriteInstance{spriteInstance(1)})
	}
	driver.Frame(0)

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlBack})
	driver.AdvanceTime(0)

	updates := driver.ConsumeFrameUpdates()
	if len(updates.SpriteBatches) != 1 {
		t.Fatalf("pending after rewind = %d, want 1 (render-only state must survive rewind untouched)",
			len(updates.SpriteBatches))
	}
}

func TestBatchResetClearsPending(t *testing.T) {
	game := &batchGame{}
	driver := runtime.NewDriver(game, testLogger())
	game.renderFn = func(ctx *runtime.RenderContext) {
		ctx.Batches.UpdateSprites(41, []render.SpriteInstance{spriteInstance(1)})
		ctx.Batches.UpdateText(42, []render.TextLabel{{Text: "x"}})
	}
	driver.Frame(0)

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReset})
	driver.AdvanceTime(0)

	updates := driver.ConsumeFrameUpdates()
	if len(updates.SpriteBatches) != 0 || len(updates.TextBatches) != 0 {
		t.Fatalf("pending after reset = %d sprites / %d text, want 0/0 (reset drops pre-reset publications)",
			len(updates.SpriteBatches), len(updates.TextBatches))
	}
}
