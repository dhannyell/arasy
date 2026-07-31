package devpanel

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func wheelEvent(delta float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelY},
		Kind:   input.EventAxisChanged,
		Value:  delta,
	}
}

// scrollValuesToBottom selects the keeper (its tween makes the component
// list long) and wheels the values list down until the drawn rows change.
func scrollValuesDown(t *testing.T, p *Panel, ticks int) {
	t.Helper()
	renderTab(p)
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeGame, false)
	clickPanel(p, lay.modes[1].Center())
	renderTab(p)
	worldLay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, false)
	clickPanel(p, math2d.Vec2{
		X: worldLay.entities.Min.X + 8,
		Y: worldLay.entities.Min.Y + 1.5*inspectRowHeight,
	})
	renderTab(p)

	center := worldLay.values.Center()
	for i := 0; i < ticks; i++ {
		p.HandleInput([]input.Event{moveTo(center), wheelEvent(3)}, testViewport())
	}
}

// firstValueText returns the topmost text drawn inside the values rect,
// which is the first visible row of the scrolled list.
func firstValueText(t *testing.T, cmds []render.Command, values math2d.Rect) string {
	t.Helper()
	best := ""
	bestY := 0.0
	for _, cmd := range cmds {
		text, ok := cmd.(render.DrawText)
		if !ok || !values.Contains(text.Position) {
			continue
		}
		if best == "" || text.Position.Y < bestY {
			best = text.Text
			bestY = text.Position.Y
		}
	}
	if best == "" {
		t.Fatal("no text drawn inside the values list")
	}
	return best
}

// TestEditKeepsScrollWhenArming reproduces the reported bug: scrolling the
// values list and clicking a row that was below the initial fold must arm
// the editor without the list jumping back to the top.
func TestEditKeepsScrollWhenArming(t *testing.T) {
	p := newInspectPanel(t)
	scrollValuesDown(t, p, 6)

	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, false)

	before := renderTab(p)
	topRowBefore := firstValueText(t, before, lay.values)
	if topRowBefore == "Transform" {
		t.Fatalf("list did not scroll: first row is still %q", topRowBefore)
	}
	t.Logf("scrolled: first visible row %q", topRowBefore)

	// Click the row in the middle of the values view.
	clickPanel(p, math2d.Vec2{X: lay.values.Min.X + 8, Y: lay.values.Center().Y})

	if !p.inspect.edit.armed() {
		t.Fatal("clicking a scrolled row did not arm the editor")
	}

	after := renderTab(p)
	topRowAfter := firstValueText(t, after, lay.values)
	if topRowAfter != topRowBefore {
		t.Fatalf("list jumped on arm: first row %q -> %q, want it kept", topRowBefore, topRowAfter)
	}
}
