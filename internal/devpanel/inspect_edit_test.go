//go:build arr_dev

package devpanel

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

// TestComponentSettersMatchReaders is the write side's coverage guard: every
// component reader on WorldView needs a matching Set<Component> on World, or
// the editor would offer a field it cannot write.
func TestComponentSettersMatchReaders(t *testing.T) {
	viewType := reflect.TypeFor[scene.WorldView]()
	for _, prop := range worldViewProps {
		setter, ok := worldSetters[prop.name]
		if !ok {
			t.Errorf("component %s has a reader but no Set%s on World", prop.name, prop.name)
			continue
		}
		getterType := viewType.Method(prop.index).Type.Out(0)
		if getterType != setter.typ {
			t.Errorf("%s: reader returns %v but setter takes %v", prop.name, getterType, setter.typ)
		}
	}
}

// selectFirstEntity switches to the entities source and clicks the first row.
func selectFirstEntity(t *testing.T, p *Panel) {
	t.Helper()
	renderTab(p)
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeGame, false)
	clickPanel(p, lay.modes[1].Center())
	renderTab(p)
	worldLay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, false)
	clickPanel(p, math2d.Vec2{
		X: worldLay.entities.Min.X + 8,
		Y: worldLay.entities.Min.Y + inspectRowHeight/2,
	})
	renderTab(p)
	if !p.inspect.hasEntity {
		t.Fatal("no entity selected")
	}
}

// clickValueRow clicks the first values-list row whose text contains sub and
// returns the row.
func clickValueRow(t *testing.T, p *Panel, sub string) inspectRow {
	t.Helper()
	rows := p.inspect.compRows
	index := -1
	for i, row := range rows {
		if len(row.text) >= len(sub) && contains(row.text, sub) {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("no value row containing %q in %v", sub, rows)
	}
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, p.inspect.edit.active())
	clickPanel(p, math2d.Vec2{
		X: lay.values.Min.X + 8,
		Y: lay.values.Min.Y + (float64(index)+0.5)*inspectRowHeight,
	})
	return rows[index]
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// submitEditor focuses the editor field, replaces the draft and presses
// Enter — the exact sequence a user performs.
func submitEditor(t *testing.T, p *Panel, draft string) {
	t.Helper()
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, true)
	clickPanel(p, lay.editor.Center()) // focus the field
	p.inspect.edit.text = draft
	p.HandleInput([]input.Event{keyEvent(input.EventButtonDown, input.KeyEnter)}, testViewport())
}

func TestArmEditDraftsTheLiveValue(t *testing.T) {
	p := newInspectPanel(t)
	selectFirstEntity(t, p)

	row := clickValueRow(t, p, "Position.X")
	if !p.inspect.edit.armed() {
		t.Fatal("clicking a scalar row did not arm the editor")
	}
	ball, _ := p.driver.WorldView().Transform(p.inspect.entity)
	want := strconv.FormatFloat(ball.Position.X, 'g', -1, 64)
	if p.inspect.edit.text != want {
		t.Fatalf("draft = %q, want the live value %q", p.inspect.edit.text, want)
	}
	if p.inspect.edit.comp != row.comp || p.inspect.edit.path != "Position.X" {
		t.Fatalf("editor armed for %s.%s, want Transform.Position.X",
			p.inspect.edit.comp, p.inspect.edit.path)
	}
}

func TestApplyEditWritesThroughTheWorldSetter(t *testing.T) {
	p := newInspectPanel(t)
	selectFirstEntity(t, p)
	clickValueRow(t, p, "Position.X")

	submitEditor(t, p, "9")

	ball, _ := p.driver.WorldView().Transform(p.inspect.entity)
	if ball.Position.X != 9 {
		t.Fatalf("Position.X = %v, want 9 after the edit", ball.Position.X)
	}
	if cmds := renderTab(p); !containsText(cmds, "Transform.Position.X applied") {
		t.Fatal("no applied status after the edit")
	}
}

func TestApplyEditRejectsGarbage(t *testing.T) {
	p := newInspectPanel(t)
	selectFirstEntity(t, p)
	clickValueRow(t, p, "Position.X")
	before, _ := p.driver.WorldView().Transform(p.inspect.entity)

	submitEditor(t, p, "abc")

	after, _ := p.driver.WorldView().Transform(p.inspect.entity)
	if after != before {
		t.Fatalf("an invalid draft changed the transform: %+v -> %+v", before, after)
	}
	if cmds := renderTab(p); !containsText(cmds, "\"abc\" is not a number") {
		t.Fatal("no parse-error status")
	}
}

func TestBoolRowTogglesInPlace(t *testing.T) {
	p := newInspectPanel(t)
	renderTab(p)
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeGame, false)
	clickPanel(p, lay.modes[1].Center())
	renderTab(p)

	// The keeper is the second row: it carries the tween with the bools.
	worldLay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, false)
	clickPanel(p, math2d.Vec2{
		X: worldLay.entities.Min.X + 8,
		Y: worldLay.entities.Min.Y + 1.5*inspectRowHeight,
	})
	renderTab(p)

	// The bool rows sit below the fold of this long component, so arm
	// through the same entry point the click uses instead of scrolling.
	var boolRow inspectRow
	for _, row := range p.inspect.compRows {
		if row.kind == rowBool && contains(row.text, "RemoveOnComplete") {
			boolRow = row
			break
		}
	}
	if boolRow.kind != rowBool {
		t.Fatal("no RemoveOnComplete row in the keeper's components")
	}
	p.armInspectEdit(boolRow)

	tween, ok := p.driver.WorldView().TransformTween(p.inspect.entity)
	if !ok {
		t.Fatal("the keeper lost its tween")
	}
	if !tween.RemoveOnComplete {
		t.Fatal("arming a bool row did not toggle it")
	}
	requireTextContains(t, renderTab(p), "RemoveOnComplete toggled")
}

func TestEditorDropsOnSelectionChange(t *testing.T) {
	p := newInspectPanel(t)
	selectFirstEntity(t, p)
	clickValueRow(t, p, "Position.X")
	if !p.inspect.edit.armed() {
		t.Fatal("editor did not arm")
	}

	// Select the keeper: the editor must not carry a field of another entity.
	worldLay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, p.inspect.edit.active())
	clickPanel(p, math2d.Vec2{
		X: worldLay.entities.Min.X + 8,
		Y: worldLay.entities.Min.Y + 1.5*inspectRowHeight,
	})
	if p.inspect.edit.armed() {
		t.Fatal("editor kept a field of the previous entity")
	}
}

// TestGameStateRowsAreNotEditable pins the boundary: the game-state tree is
// a snapshot copy, so its rows never arm an editor.
func TestGameStateRowsAreNotEditable(t *testing.T) {
	p := newInspectPanel(t)
	renderTab(p) // game source is the default

	for _, row := range p.inspect.gameRows {
		if row.kind != rowInfo {
			t.Fatalf("game-state row %q is editable (%v); only world rows may be", row.text, row.kind)
		}
	}
}
