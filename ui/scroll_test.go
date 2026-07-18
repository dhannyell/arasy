package ui

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

func TestScrollOffsetConsumesWheelInsideViewAndClamps(t *testing.T) {
	state := &State{}
	view := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 100, Y: 50})
	cam := testInteractCamera()
	u := NewUpdate(state, input.PointerFrame{
		Pos:    math2d.Vec2{X: 20, Y: 20},
		Inside: true,
		Wheel:  math2d.Vec2{Y: 2},
	}, cam, DefaultTheme(), cam.Viewport)

	offset := u.scrollOffsetIn("list", view, 90, SpaceRef)
	if offset != 40 {
		t.Fatalf("offset = %v, want max clamp 40", offset)
	}

	u = NewUpdate(state, input.PointerFrame{
		Pos:    math2d.Vec2{X: 20, Y: 20},
		Inside: true,
		Wheel:  math2d.Vec2{Y: -10},
	}, cam, DefaultTheme(), cam.Viewport)
	offset = u.scrollOffsetIn("list", view, 90, SpaceRef)
	if offset != 0 {
		t.Fatalf("offset after negative wheel = %v, want 0", offset)
	}
}

func TestScrollOffsetIgnoresWheelOutsideView(t *testing.T) {
	state := &State{}
	view := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 100, Y: 50})
	cam := testInteractCamera()
	u := NewUpdate(state, input.PointerFrame{
		Pos:    math2d.Vec2{X: 200, Y: 200},
		Inside: true,
		Wheel:  math2d.Vec2{Y: 1},
	}, cam, DefaultTheme(), cam.Viewport)

	if got := u.scrollOffsetIn("list", view, 120, SpaceRef); got != 0 {
		t.Fatalf("offset = %v, want 0 outside view", got)
	}
}

func TestScrollAreaReturnsWorldClipAndScrollbarCommands(t *testing.T) {
	state := &State{scroll: map[ID]float64{HashID("list"): 20}}
	cam := testCamera()
	store := AnimStore{}
	u := New(state, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	view := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 100, Y: 50})

	offset, clip := u.scrollAreaIn("list", view, 120, SpaceRef)
	if offset != 20 {
		t.Fatalf("offset = %v, want 20", offset)
	}
	if !rectNear(clip, view) {
		t.Fatalf("clip = %+v, want view %+v", clip, view)
	}
	if len(u.Commands()) != 2 {
		t.Fatalf("commands = %d, want scrollbar track and thumb", len(u.Commands()))
	}
}

func rectNear(a, b math2d.Rect) bool {
	const eps = 1e-9
	return abs(a.Min.X-b.Min.X) <= eps &&
		abs(a.Min.Y-b.Min.Y) <= eps &&
		abs(a.Max.X-b.Max.X) <= eps &&
		abs(a.Max.Y-b.Max.Y) <= eps
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
