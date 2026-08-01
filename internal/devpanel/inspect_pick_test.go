//go:build arr_dev

package devpanel

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

func TestPickEntityFindsTopmostCollider(t *testing.T) {
	world := scene.NewWorld()

	floor := world.Spawn()
	world.SetTransform(floor, scene.Transform{Position: math2d.Vec2{X: 10, Y: 10}})
	world.SetCircleCollider(floor, scene.CircleCollider{Radius: 5})

	top := world.Spawn()
	world.SetTransform(top, scene.Transform{Position: math2d.Vec2{X: 12, Y: 10}})
	world.SetCircleCollider(top, scene.CircleCollider{Radius: 5})
	world.SetCircleSprite(top, scene.CircleSprite{BaseSprite: scene.BaseSprite{Layer: 2}})

	entities := world.Entities()
	view := world.View()

	// The point sits inside both circles; the higher layer wins.
	if id, ok := pickEntity(view, entities, math2d.Vec2{X: 11, Y: 10}); !ok || id != top {
		t.Fatalf("pickEntity = (%d, %v), want the layer-2 entity %d", id, ok, top)
	}

	// Without the sprite the layer tie remains, and equal layers and Y break
	// toward the larger id, matching the render order.
	world.RemoveCircleSprite(top)
	if id, ok := pickEntity(view, entities, math2d.Vec2{X: 11, Y: 10}); !ok || id != top {
		t.Fatalf("pickEntity without layers = (%d, %v), want %d (tie goes to the larger id)", id, ok, top)
	}

	if _, ok := pickEntity(view, entities, math2d.Vec2{X: 100, Y: 100}); ok {
		t.Fatal("pickEntity hit something far away from every collider")
	}
}

func TestPickEntityRectColliderAndNoCollider(t *testing.T) {
	world := scene.NewWorld()

	box := world.Spawn()
	world.SetTransform(box, scene.Transform{Position: math2d.Vec2{X: 50, Y: 50}})
	world.SetRectCollider(box, scene.RectCollider{HalfExtends: math2d.Vec2{X: 10, Y: 5}})

	ghost := world.Spawn() // no collider: nothing to click on
	world.SetTransform(ghost, scene.Transform{Position: math2d.Vec2{X: 50, Y: 50}})

	entities := world.Entities()
	view := world.View()

	if id, ok := pickEntity(view, entities, math2d.Vec2{X: 55, Y: 52}); !ok || id != box {
		t.Fatalf("pickEntity inside the rect = (%d, %v), want %d", id, ok, box)
	}
	if _, ok := pickEntity(view, entities, math2d.Vec2{X: 61, Y: 50}); ok {
		t.Fatal("pickEntity hit past the half extents")
	}
}

// armPick opens the inspector on the entities source and clicks Pick.
func armPick(t *testing.T, p *Panel) {
	t.Helper()
	renderTab(p) // cache the frame camera and the entity list
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeGame, false)
	clickPanel(p, lay.modes[2].Center())
	if !p.pickArmed {
		t.Fatal("Pick did not arm")
	}
	if p.inspect.mode != inspectModeWorld {
		t.Fatal("arming Pick did not reveal the entities source")
	}
}

func TestPickGestureSelectsEntityAndConsumesTheClick(t *testing.T) {
	p := newInspectPanel(t)
	entities := p.driver.WorldView().Entities()
	armPick(t, p)

	// The ball sits at ~(3,4) with radius 5; the camera is the identity.
	click := []input.Event{pressAt(math2d.Vec2{X: 3, Y: 4})}
	if left := p.HandleInput(click, testViewport()); len(left) != 0 {
		t.Fatalf("pick click forwarded %d events, want none", len(left))
	}
	if p.pickArmed {
		t.Fatal("pick did not disarm after the click")
	}
	if !p.inspect.hasEntity || p.inspect.entity != entities[0] {
		t.Fatalf("pick selected %v (has=%v), want entity %d", p.inspect.entity, p.inspect.hasEntity, entities[0])
	}

	// The paired release is swallowed too: the game saw no press.
	if left := p.HandleInput([]input.Event{releaseAt(math2d.Vec2{X: 3, Y: 4})}, testViewport()); len(left) != 0 {
		t.Fatalf("paired release forwarded %d events, want none", len(left))
	}

	if cmds := renderTab(p); !containsText(cmds, "Transform") {
		t.Fatal("the picked entity's components did not show up")
	}
}

func TestPickMissStillConsumes(t *testing.T) {
	p := newInspectPanel(t)
	armPick(t, p)

	click := []input.Event{pressAt(math2d.Vec2{X: 400, Y: 590})}
	if left := p.HandleInput(click, testViewport()); len(left) != 0 {
		t.Fatalf("missed pick forwarded %d events, want none", len(left))
	}
	if p.inspect.hasEntity {
		t.Fatal("a miss selected something")
	}
}

func TestPickRightClickCancels(t *testing.T) {
	p := newInspectPanel(t)
	armPick(t, p)

	cancel := input.Event{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseRight}, Kind: input.EventButtonDown, X: 3, Y: 4}
	if left := p.HandleInput([]input.Event{cancel}, testViewport()); len(left) != 0 {
		t.Fatalf("cancel forwarded %d events, want none", len(left))
	}
	if p.pickArmed {
		t.Fatal("right click did not cancel the pick")
	}
	if p.inspect.hasEntity {
		t.Fatal("a cancelled pick selected something")
	}

	release := input.Event{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseRight}, Kind: input.EventButtonUp, X: 3, Y: 4}
	if left := p.HandleInput([]input.Event{release}, testViewport()); len(left) != 0 {
		t.Fatalf("paired right-button release forwarded %d events, want none", len(left))
	}
}

func TestPickEscapeDisarmsButPassesThrough(t *testing.T) {
	p := newInspectPanel(t)
	armPick(t, p)

	left := p.HandleInput([]input.Event{keyEvent(input.EventButtonDown, input.KeyEscape)}, testViewport())
	if p.pickArmed {
		t.Fatal("Escape did not disarm the pick")
	}
	if len(left) != 1 {
		t.Fatalf("Escape forwarded %d events, want it passed to the game", len(left))
	}
}

func TestPickDisarmsWhenThePanelHides(t *testing.T) {
	p := newInspectPanel(t)
	armPick(t, p)

	p.SetVisible(false)
	if p.pickArmed {
		t.Fatal("hiding the panel left the pick armed")
	}
}
