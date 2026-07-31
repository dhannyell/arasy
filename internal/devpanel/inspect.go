package devpanel

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

const (
	idInspectModeGame  = "devpanel#inspect-mode-game"
	idInspectModeWorld = "devpanel#inspect-mode-world"
	idInspectPick      = "devpanel#inspect-pick"
	idInspectEntities  = "devpanel#inspect-entities"
	idInspectValues    = "devpanel#inspect-values"
	idInspectEdit      = "devpanel#inspect-edit"
)

// inspectRowHeight is tighter than the panel's button rows: the value lists
// are read like a terminal, so they pack caption text.
const inspectRowHeight = 18

const (
	inspectModeGame = iota
	inspectModeWorld
)

// inspectRowKind marks what a value-list row can do. Only rows of the
// entities source are editable; the game-state tree stays read-only (the
// snapshot the inspector reads is a copy, so there is nothing to write to).
type inspectRowKind int

const (
	rowInfo inspectRowKind = iota // headers and read-only lines
	rowFloat
	rowInt
	rowBool
)

// inspectRow is one line of the value list. Editable kinds carry the
// component name and the field path the editor writes back through.
type inspectRow struct {
	text string
	kind inspectRowKind
	comp string
	path string
}

// inspectEdit is the inline editor's state: which field is armed, the draft
// being typed, and the feedback of the last apply attempt. The draft is
// caller-owned like every widget value in the panel.
type inspectEdit struct {
	comp   string
	path   string
	text   string
	status string
}

// armed reports whether the editor is bound to a field.
func (e inspectEdit) armed() bool { return e.comp != "" }

// active reports whether the editor row should exist at all: a field is
// armed, or a status line from the last action is still worth showing.
func (e inspectEdit) active() bool { return e.comp != "" || e.status != "" }

// inspectState is the inspector tab's caller-owned data: which source it
// shows, which entity is selected, and the formatted rows cached per tick
// so neither pass re-walks the reflection tree every frame.
type inspectState struct {
	mode      int
	entity    scene.EntityID
	hasEntity bool
	edit      inspectEdit

	tick       uint64
	entities   []scene.EntityID
	gameRows   []inspectRow
	compTick   uint64
	compEntity scene.EntityID
	compRows   []inspectRow
}

type inspectLayout struct {
	modes    []math2d.Rect
	editor   math2d.Rect
	entities math2d.Rect
	values   math2d.Rect
}

func inspectRects(rect math2d.Rect, mode int, editing bool) inspectLayout {
	r := newRows(rect)
	modes := ui.Columns(r.next(), 3, rowGap)
	lay := inspectLayout{modes: modes}
	if editing {
		lay.editor = r.next()
	}
	body := r.remaining()
	if mode == inspectModeGame {
		lay.values = body
		return lay
	}
	cols := ui.Columns(body, 2, rowGap)
	lay.entities = cols[0]
	lay.values = cols[1]
	return lay
}

func (p *Panel) updateInspect(in ui.InteractScope, rect math2d.Rect, state runtime.DriveState) {
	p.refreshInspect(state)
	lay := inspectRects(rect, p.inspect.mode, p.inspect.edit.active())

	if in.Clicked(idInspectModeGame, lay.modes[0]) {
		p.inspect.mode = inspectModeGame
	}
	if in.Clicked(idInspectModeWorld, lay.modes[1]) {
		p.inspect.mode = inspectModeWorld
	}
	if in.Clicked(idInspectPick, lay.modes[2]) {
		// Arming also reveals the target: the click that picks lands on the
		// world, the selection shows up in the entities view.
		p.pickArmed = !p.pickArmed
		if p.pickArmed {
			p.inspect.mode = inspectModeWorld
		}
	}

	if p.inspect.mode == inspectModeWorld {
		next, changed := in.ListInputValue(idInspectEntities, lay.entities,
			inspectRowHeight, len(p.inspect.entities), p.entityIndex())
		if changed && next >= 0 && next < len(p.inspect.entities) {
			p.inspect.entity = p.inspect.entities[next]
			p.inspect.hasEntity = true
			p.inspect.edit = inspectEdit{} // a new selection drops the editor
		}
		in.ScrollOffset(idInspectEntities, lay.entities,
			float64(len(p.inspect.entities))*inspectRowHeight)

		p.updateInspectValues(in, lay)
	}
	in.ScrollOffset(idInspectValues, lay.values,
		float64(len(p.inspectRows()))*inspectRowHeight)
}

// updateInspectValues runs the value list and the inline editor: clicking a
// scalar row arms the editor for it, clicking a bool row flips it in place.
func (p *Panel) updateInspectValues(in ui.InteractScope, lay inspectLayout) {
	rows := p.inspectRows()
	row, changed := in.ListInputValue(idInspectValues, lay.values,
		inspectRowHeight, len(rows), -1)
	if changed && row >= 0 && row < len(rows) {
		p.armInspectEdit(rows[row])
	}

	if !p.inspect.edit.armed() {
		return
	}
	result := in.TextInput(idInspectEdit, lay.editor, p.inspect.edit.text,
		ui.TextInputOptions{MaxRunes: 32})
	p.inspect.edit.text = result.Text
	if result.Submitted {
		p.applyInspectEdit()
	}
}

// armInspectEdit reacts to a clicked value row: scalars open the editor
// with the live value as the draft (never the display string, which is
// rounded), bools toggle without one.
func (p *Panel) armInspectEdit(row inspectRow) {
	switch row.kind {
	case rowFloat, rowInt:
		text, ok := p.currentFieldText(row.comp, row.path)
		if !ok {
			p.inspect.edit = inspectEdit{status: row.comp + "." + row.path + " is gone"}
			return
		}
		p.inspect.edit = inspectEdit{comp: row.comp, path: row.path, text: text}
	case rowBool:
		p.toggleInspectBool(row)
	}
}

// currentFieldText reads the armed field's live value as editable text.
func (p *Panel) currentFieldText(comp, path string) (string, bool) {
	current, ok := findComponentValue(p.driver.WorldView(), p.inspect.entity, comp)
	if !ok {
		return "", false
	}
	field := current
	for _, name := range strings.Split(path, ".") {
		for field.Kind() == reflect.Pointer || field.Kind() == reflect.Interface {
			if field.IsNil() {
				return "", false
			}
			field = field.Elem()
		}
		if field.Kind() != reflect.Struct {
			return "", false
		}
		field = field.FieldByName(name)
		if !field.IsValid() {
			return "", false
		}
	}
	switch field.Kind() {
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(field.Float(), 'g', -1, 64), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%v", field.Interface()), true
	}
	return "", false
}

func (p *Panel) drawInspect(draw ui.DrawScope, rect math2d.Rect, state runtime.DriveState) {
	lay := inspectRects(rect, p.inspect.mode, p.inspect.edit.active())

	gameLabel, worldLabel, pickLabel := "Game state", "Entities", "Pick"
	if p.inspect.mode == inspectModeGame {
		gameLabel = "[" + gameLabel + "]"
	} else {
		worldLabel = "[" + worldLabel + "]"
	}
	if p.pickArmed {
		pickLabel = "[" + pickLabel + "]"
	}
	draw.Button(idInspectModeGame, gameLabel, lay.modes[0])
	draw.Button(idInspectModeWorld, worldLabel, lay.modes[1])
	draw.Button(idInspectPick, pickLabel, lay.modes[2])

	if p.inspect.mode == inspectModeWorld {
		draw.List(idInspectEntities, lay.entities, inspectRowHeight,
			len(p.inspect.entities), p.entityIndex(),
			func(row int, rowRect math2d.Rect) {
				p.text(draw, "#"+formatUint(uint64(p.inspect.entities[row])),
					math2d.Vec2{X: rowRect.Min.X + 4, Y: rowRect.Center().Y},
					p.theme.Text.Color, p.theme.Text.Caption)
			})

		if p.inspect.edit.armed() {
			draw.TextInput(idInspectEdit, lay.editor, p.inspect.edit.text,
				ui.TextInputOptions{Placeholder: p.inspect.edit.comp + "." + p.inspect.edit.path})
		} else if p.inspect.edit.status != "" {
			p.text(draw, p.inspect.edit.status, leftOf(lay.editor),
				p.theme.Text.Muted, p.theme.Text.Caption)
		}
	}

	rows := p.inspectRows()
	draw.List(idInspectValues, lay.values, inspectRowHeight, len(rows), -1,
		func(row int, rowRect math2d.Rect) {
			color := p.theme.Text.Color
			if rows[row].kind != rowInfo {
				color = colorAction
			}
			p.text(draw, truncateToWidth(rows[row].text, rowRect.Width()-4, p.theme.Text.Caption),
				math2d.Vec2{X: rowRect.Min.X + 2, Y: rowRect.Center().Y},
				color, p.theme.Text.Caption)
		})
}

// inspectRows is the value list for the current mode and selection.
func (p *Panel) inspectRows() []inspectRow {
	if p.inspect.mode == inspectModeGame {
		return p.inspect.gameRows
	}
	switch {
	case len(p.inspect.entities) == 0:
		return []inspectRow{{text: "(the world has no entities)"}}
	case !p.inspect.hasEntity:
		return []inspectRow{{text: "(select an entity)"}}
	default:
		return p.inspect.compRows
	}
}

// entityIndex maps the selected entity back to its row, or -1 when the
// selection is empty or the entity left the world.
func (p *Panel) entityIndex() int {
	if !p.inspect.hasEntity {
		return -1
	}
	if idx, found := slices.BinarySearch(p.inspect.entities, p.inspect.entity); found {
		return idx
	}
	return -1
}

// refreshInspect re-reads the world and the game state once per tick. The
// inspector is read-only tooling over live data: formatting is cheap enough
// per tick, and caching per tick keeps the interaction and draw passes
// looking at the same answer.
func (p *Panel) refreshInspect(state runtime.DriveState) {
	if p.inspect.tick != state.Tick {
		p.inspect.tick = state.Tick
		p.inspect.entities = p.driver.WorldView().Entities()
		if value, ok := p.driver.DebugGameState(); ok {
			p.inspect.gameRows = infoRows(value)
		} else {
			p.inspect.gameRows = []inspectRow{{text: "(the game does not implement StatefulGame)"}}
		}
		p.inspect.compTick = ^uint64(0) // force the component refresh below
	}

	if !p.inspect.hasEntity {
		p.inspect.compRows = nil
		return
	}
	if p.inspect.compTick == state.Tick && p.inspect.compEntity == p.inspect.entity {
		return
	}
	p.inspect.compTick = state.Tick
	p.inspect.compEntity = p.inspect.entity
	if _, found := slices.BinarySearch(p.inspect.entities, p.inspect.entity); !found {
		p.inspect.compRows = []inspectRow{{text: "#" + formatUint(uint64(p.inspect.entity)) + " left the world"}}
		return
	}
	p.inspect.compRows = componentRows(p.driver.WorldView(), p.inspect.entity)
}

// infoRows wraps the read-only game-state tree as rows.
func infoRows(value any) []inspectRow {
	lines := formatTree(value)
	rows := make([]inspectRow, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, inspectRow{text: line})
	}
	return rows
}

// componentProp is one component reader on scene.WorldView, discovered by
// reflection: one EntityID in, a component and a presence bool out. New
// components following the Set/getter convention show up in the inspector
// with no inspector change.
type componentProp struct {
	name  string
	index int
}

// worldViewProps lists every component reader on WorldView, sorted by name
// for a stable display order. Non-component methods (RenderCommands,
// Contacts, Entities, StepTick) fail the signature filter.
var worldViewProps = collectComponentProps()

func collectComponentProps() []componentProp {
	viewType := reflect.TypeFor[scene.WorldView]()
	entityType := reflect.TypeFor[scene.EntityID]()
	var props []componentProp
	for i := 0; i < viewType.NumMethod(); i++ {
		method := viewType.Method(i)
		if method.Type.NumIn() != 2 || method.Type.In(1) != entityType {
			continue
		}
		if method.Type.NumOut() != 2 || method.Type.Out(1).Kind() != reflect.Bool {
			continue
		}
		props = append(props, componentProp{name: method.Name, index: i})
	}
	slices.SortFunc(props, func(a, b componentProp) int { return strings.Compare(a.name, b.name) })
	return props
}

// componentValue is one component an entity holds, with its live value.
type componentValue struct {
	name  string
	value reflect.Value
}

// componentsOf reads every component id holds through the view's readers.
func componentsOf(view scene.WorldView, id scene.EntityID) []componentValue {
	viewValue := reflect.ValueOf(view)
	entity := reflect.ValueOf(id)

	var out []componentValue
	for _, prop := range worldViewProps {
		results := viewValue.Method(prop.index).Call([]reflect.Value{entity})
		if !results[1].Bool() {
			continue
		}
		out = append(out, componentValue{name: prop.name, value: results[0]})
	}
	return out
}

// componentRows formats every component id holds: a header per component,
// then one row per scalar field, which is what makes the field editable.
func componentRows(view scene.WorldView, id scene.EntityID) []inspectRow {
	components := componentsOf(view, id)
	if len(components) == 0 {
		return []inspectRow{{text: "#" + formatUint(uint64(id)) + " holds no components"}}
	}

	var rows []inspectRow
	for _, component := range components {
		rows = append(rows, inspectRow{text: component.name})
		walkComponentRows(&rows, component.value, component.name, "", 1)
	}
	return rows
}

// walkComponentRows appends one row per leaf field, keeping the field path
// (Position.X) so the editor can write the row back. Structs always expand:
// the inline compactness of the game-state tree would cost every field its
// own editable row.
func walkComponentRows(rows *[]inspectRow, v reflect.Value, comp, prefix string, depth int) {
	if len(*rows) >= inspectMaxLines {
		return
	}
	if depth > inspectMaxDepth {
		*rows = append(*rows, inspectRow{text: indent(depth) + "…"})
		return
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			*rows = append(*rows, inspectRow{text: indent(depth) + prefix + ": nil"})
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		if prefix != "" {
			*rows = append(*rows, inspectRow{text: indent(depth) + prefix})
		}
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			walkComponentRows(rows, v.Field(i), comp, path, depth+1)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		// Collections stay read-only lines; editing an element through a
		// reflection path buys little next to the churn it costs.
		*rows = append(*rows, inspectRow{
			text: indent(depth) + fmt.Sprintf("%s: %s [len %d]", prefix, v.Type(), v.Len()),
		})
	case reflect.Bool:
		*rows = append(*rows, inspectRow{
			text: indent(depth) + fmt.Sprintf("%s: %v", prefix, v.Bool()),
			kind: rowBool, comp: comp, path: prefix,
		})
	case reflect.Float32, reflect.Float64:
		*rows = append(*rows, inspectRow{
			text: indent(depth) + fmt.Sprintf("%s: %v", prefix, v.Float()),
			kind: rowFloat, comp: comp, path: prefix,
		})
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*rows = append(*rows, inspectRow{
			text: indent(depth) + fmt.Sprintf("%s: %v", prefix, v.Interface()),
			kind: rowInt, comp: comp, path: prefix,
		})
	default:
		*rows = append(*rows, inspectRow{
			text: indent(depth) + fmt.Sprintf("%s: %v", prefix, v),
		})
	}
}

func indent(depth int) string { return strings.Repeat("  ", depth) }

// --- write-back ---

// componentSetter is one Set<Component> writer on scene.World, matched to a
// reader by name: Set takes an EntityID and the component, nothing back.
type componentSetter struct {
	index int
	typ   reflect.Type
}

var worldSetters = collectComponentSetters()

func collectComponentSetters() map[string]componentSetter {
	worldType := reflect.TypeFor[*scene.World]()
	entityType := reflect.TypeFor[scene.EntityID]()
	setters := map[string]componentSetter{}
	for i := 0; i < worldType.NumMethod(); i++ {
		method := worldType.Method(i)
		if !strings.HasPrefix(method.Name, "Set") {
			continue
		}
		if method.Type.NumIn() != 3 || method.Type.In(1) != entityType || method.Type.NumOut() != 0 {
			continue
		}
		setters[method.Name[len("Set"):]] = componentSetter{index: i, typ: method.Type.In(2)}
	}
	return setters
}

// applyInspectEdit parses the draft and writes it into the armed field of
// the selected entity's component. The write goes through the world's own
// setter, exactly the path a game's Update would take. Either way the
// editor closes into a status line: on a typo the row re-reads the live
// value on the next arm, so no stale draft is worth keeping.
func (p *Panel) applyInspectEdit() {
	edit := p.inspect.edit
	if err := p.writeComponentField(edit.comp, edit.path, edit.text); err != nil {
		p.inspect.edit = inspectEdit{status: err.Error()}
		return
	}
	p.inspect.edit = inspectEdit{status: edit.comp + "." + edit.path + " applied"}
	// The value changed outside the tick: force the component cache to
	// re-read on the next pass instead of showing the stale copy.
	p.inspect.compTick = ^uint64(0)
}

// toggleInspectBool flips a bool field in place, no editor needed.
func (p *Panel) toggleInspectBool(row inspectRow) {
	if err := p.writeComponentField(row.comp, row.path, "toggle"); err != nil {
		p.inspect.edit = inspectEdit{status: err.Error()}
		return
	}
	p.inspect.edit = inspectEdit{status: row.comp + "." + row.path + " toggled"}
	p.inspect.compTick = ^uint64(0)
}

// writeComponentField sets one field, addressed by its display path, on the
// selected entity's component. text is the draft for numeric kinds and the
// literal "toggle" for bools.
func (p *Panel) writeComponentField(comp, path, text string) error {
	setter, ok := worldSetters[comp]
	if !ok {
		return fmt.Errorf("%s has no world setter", comp)
	}

	current, ok := findComponentValue(p.driver.WorldView(), p.inspect.entity, comp)
	if !ok {
		return fmt.Errorf("#%d no longer holds %s", p.inspect.entity, comp)
	}

	next := reflect.New(setter.typ).Elem()
	next.Set(current)
	field := next
	for _, name := range strings.Split(path, ".") {
		if field.Kind() != reflect.Struct {
			return fmt.Errorf("%s is not a struct along %s", field.Type(), path)
		}
		field = field.FieldByName(name)
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%s has no settable field %s", comp, path)
		}
	}

	switch field.Kind() {
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return fmt.Errorf("%q is not a number", text)
		}
		field.SetFloat(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return fmt.Errorf("%q is not an integer", text)
		}
		field.SetInt(value)
	case reflect.Bool:
		if text != "toggle" {
			return fmt.Errorf("bool fields toggle, they do not parse")
		}
		field.SetBool(!field.Bool())
	default:
		return fmt.Errorf("%s is not editable", field.Kind())
	}

	reflect.ValueOf(p.driver.DebugWorld()).Method(setter.index).Call([]reflect.Value{
		reflect.ValueOf(p.inspect.entity), next,
	})
	return nil
}

// findComponentValue reads one component of one entity by reader name.
func findComponentValue(view scene.WorldView, id scene.EntityID, comp string) (reflect.Value, bool) {
	for _, candidate := range componentsOf(view, id) {
		if candidate.name == comp {
			return candidate.value, true
		}
	}
	return reflect.Value{}, false
}
