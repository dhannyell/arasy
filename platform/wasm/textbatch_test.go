package wasm

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestCurrentTextBatchProjKeyChangesWithCameraOrCanvas(t *testing.T) {
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
	base := currentTextBatchProjKey(cam, 800, 600)
	if again := currentTextBatchProjKey(cam, 800, 600); again != base {
		t.Fatal("identical camera and canvas must produce an identical projection key")
	}
	if resized := currentTextBatchProjKey(cam, 801, 600); resized == base {
		t.Fatal("canvas resize must change the projection key")
	}
	zoomed := cam
	zoomed.World = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 50, Y: 50})
	if changed := currentTextBatchProjKey(zoomed, 800, 600); changed == base {
		t.Fatal("camera world change must change the projection key")
	}
}

func TestTextBatchScreenViewNoClipIsCanvasRect(t *testing.T) {
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
	view, ok := textBatchScreenView(cam, 320, 240, math2d.Rect{})
	if !ok {
		t.Fatal("ok = false, want true for a positive-area canvas")
	}
	want := math2d.Rect{Max: math2d.Vec2{X: 320, Y: 240}}
	if view != want {
		t.Fatalf("view = %+v, want %+v", view, want)
	}
}

func TestTextBatchScreenViewClipTightensCanvasRect(t *testing.T) {
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 200}), // 2x world->screen scale
	}
	clip := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 20, Y: 20}) // world (10,10)-(30,30)
	view, ok := textBatchScreenView(cam, 400, 400, clip)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	want := math2d.Rect{Min: math2d.Vec2{X: 20, Y: 20}, Max: math2d.Vec2{X: 60, Y: 60}}
	if view != want {
		t.Fatalf("view = %+v, want %+v (clip projected to screen and clamped to the canvas)", view, want)
	}
}

func TestTextBatchScreenViewDegenerateCanvas(t *testing.T) {
	if _, ok := textBatchScreenView(render.Camera{}, 0, 0, math2d.Rect{}); ok {
		t.Fatal("ok = true, want false for a zero-size canvas")
	}
}

func drawTextBatchCmd(id render.TextBatchID) render.DrawTextBatch {
	return render.DrawTextBatch{Batch: id}
}

func TestCollectTextBatchIDsDedupesInFirstAppearanceOrder(t *testing.T) {
	commands := []render.Command{
		drawTextBatchCmd(3),
		render.DrawRect{},
		drawTextBatchCmd(1),
		drawTextBatchCmd(3),
		drawTextBatchCmd(2),
	}
	got := collectTextBatchIDs(commands)
	want := []render.TextBatchID{3, 1, 2}
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v", got, want)
		}
	}
}

func TestCollectTextBatchIDsEmptyWhenNoneReferenced(t *testing.T) {
	if ids := collectTextBatchIDs([]render.Command{render.DrawRect{}}); ids != nil {
		t.Fatalf("ids = %v, want nil", ids)
	}
}
