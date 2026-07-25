package ui_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/ui"
)

func contractCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
}

func TestDrawerInEmitsButtonCommands(t *testing.T) {
	cam := contractCamera()
	store := ui.AnimStore{}
	u := ui.New(&ui.State{}, math2d.Vec2{}, true, cam, ui.DefaultTheme(), cam.Viewport, &store, 0, 0)

	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 40, Y: 20})
	u.Draw().In(ui.SpaceScreen).Button("btn", "Go", rect)

	var hasRect, hasText bool
	for _, cmd := range u.Commands() {
		switch cmd.(type) {
		case render.DrawRect:
			hasRect = true
		case render.DrawText:
			hasText = true
		}
	}
	if !hasRect || !hasText {
		t.Fatalf("Commands() = %+v, want at least one DrawRect and one DrawText", u.Commands())
	}
}

func TestInteractorInReportsClickOnDownUpInsideSameTick(t *testing.T) {
	cam := contractCamera()
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 40, Y: 20})
	pos := rect.Center()

	u := ui.NewUpdate(&ui.State{}, input.PointerFrame{
		Inside: true,
		Events: []input.PointerEvent{
			{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
			{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: pos},
		},
	}, cam, ui.DefaultTheme(), cam.Viewport)

	if !u.Interact().In(ui.SpaceRef).Clicked("btn", rect) {
		t.Fatal("expected Clicked to report true when down+up land inside rect in the same tick")
	}
}
