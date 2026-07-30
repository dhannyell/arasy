package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// tabsRect is 300 wide, so three equal headers are 100 wide each and land on
// round numbers the geometry assertions can state literally.
func tabsRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 300, Y: 200})
}

func tabsLabels() []string {
	return []string{"Scene", "Assets", "Log"}
}

func tabsUpdate(state *State, events ...input.PointerEvent) *UI {
	cam := testInteractCamera()
	return NewUpdate(state, pointerFrame(events...), cam, DefaultTheme(), cam.Viewport)
}

func clickTab(pos math2d.Vec2) []input.PointerEvent {
	return []input.PointerEvent{
		{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
		{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: pos},
	}
}

func TestTabsInputSelectsClickedHeader(t *testing.T) {
	state := &State{}
	header := tabHeaderRect(tabsRect(), 2, 3, DefaultTheme().Tabs.BarHeight)
	u := tabsUpdate(state, clickTab(header.Center())...)

	active, changed := u.tabsInputIn("inspector", tabsRect(), 3, SpaceRef)

	if active != 2 || !changed {
		t.Fatalf("tabsInputIn() = (%d, %v), want (2, true)", active, changed)
	}
	if state.tabs[HashID("inspector")] != 2 {
		t.Fatalf("persisted tab = %d, want 2", state.tabs[HashID("inspector")])
	}
}

func TestTabsInputWithoutClickKeepsActiveTab(t *testing.T) {
	state := &State{tabs: map[ID]int{HashID("inspector"): 1}}
	u := tabsUpdate(state)

	active, changed := u.tabsInputIn("inspector", tabsRect(), 3, SpaceRef)

	if active != 1 || changed {
		t.Fatalf("tabsInputIn() = (%d, %v), want preserved (1, false)", active, changed)
	}
}

func TestTabsInputClickOnActiveHeaderReportsNoChange(t *testing.T) {
	state := &State{tabs: map[ID]int{HashID("inspector"): 1}}
	header := tabHeaderRect(tabsRect(), 1, 3, DefaultTheme().Tabs.BarHeight)
	u := tabsUpdate(state, clickTab(header.Center())...)

	active, changed := u.tabsInputIn("inspector", tabsRect(), 3, SpaceRef)

	if active != 1 || changed {
		t.Fatalf("tabsInputIn() = (%d, %v), want (1, false)", active, changed)
	}
}

func TestTabsInputIgnoresClickBelowTheHeaderBand(t *testing.T) {
	state := &State{}
	body := tabsBodyRect(tabsRect(), DefaultTheme().Tabs.BarHeight)
	u := tabsUpdate(state, clickTab(body.Center())...)

	if active, changed := u.tabsInputIn("inspector", tabsRect(), 3, SpaceRef); active != 0 || changed {
		t.Fatalf("tabsInputIn() = (%d, %v), want body click ignored (0, false)", active, changed)
	}
}

// Two tab widgets on one screen must not share hit targets: the derived
// header ids keep a click on one bar out of the other's state.
func TestTabsInputKeepsWidgetsIndependent(t *testing.T) {
	state := &State{}
	header := tabHeaderRect(tabsRect(), 2, 3, DefaultTheme().Tabs.BarHeight)
	u := tabsUpdate(state, clickTab(header.Center())...)

	if _, changed := u.tabsInputIn("inspector", tabsRect(), 3, SpaceRef); !changed {
		t.Fatal("first widget did not observe the click")
	}
	if active := state.tabs[HashID("console")]; active != 0 {
		t.Fatalf("other widget tab = %d, want untouched 0", active)
	}
}

func TestTabsDrawReadsTabSelectedInUpdate(t *testing.T) {
	state := &State{}
	header := tabHeaderRect(tabsRect(), 2, 3, DefaultTheme().Tabs.BarHeight)
	update := tabsUpdate(state, clickTab(header.Center())...)
	if _, changed := update.tabsInputIn("inspector", tabsRect(), 3, SpaceRef); !changed {
		t.Fatal("setup click did not select a tab")
	}

	cam := testCamera()
	store := AnimStore{}
	draw := New(state, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)

	if active := draw.tabsIn("inspector", tabsRect(), tabsLabels(), SpaceRef); active != 2 {
		t.Fatalf("tabsIn() = %d, want the tab selected in Update (2)", active)
	}
}

// Drawing is read-only with respect to selection: a cursor parked over another
// header must not move the active tab.
func TestTabsDrawDoesNotSelect(t *testing.T) {
	state := &State{tabs: map[ID]int{HashID("inspector"): 1}}
	header := tabHeaderRect(tabsRect(), 0, 3, DefaultTheme().Tabs.BarHeight)
	cam := testCamera()
	store := AnimStore{}
	draw := New(state, header.Center(), true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)

	if active := draw.tabsIn("inspector", tabsRect(), tabsLabels(), SpaceRef); active != 1 {
		t.Fatalf("tabsIn() = %d, want hover to leave the active tab at 1", active)
	}
}

func TestTabsClampsActiveToTabCount(t *testing.T) {
	state := &State{tabs: map[ID]int{HashID("inspector"): 5}}
	cam := testCamera()
	store := AnimStore{}
	draw := New(state, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)

	if active := draw.tabsIn("inspector", tabsRect(), tabsLabels(), SpaceRef); active != 2 {
		t.Fatalf("tabsIn() = %d, want clamped to last tab 2", active)
	}

	update := tabsUpdate(&State{tabs: map[ID]int{HashID("inspector"): 5}})
	if active, _ := update.tabsInputIn("inspector", tabsRect(), 2, SpaceRef); active != 1 {
		t.Fatalf("tabsInputIn() = %d, want clamped to last tab 1", active)
	}
}

func TestTabsDrawEmitsBarAndOneHeaderPerLabel(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	theme := DefaultTheme()
	draw := New(&State{}, math2d.Vec2{}, true, cam, theme, cam.Viewport, &store, 0, 0)

	draw.tabsIn("inspector", tabsRect(), tabsLabels(), SpaceRef)

	commands := draw.Commands()
	if len(commands) != 1+2*len(tabsLabels()) {
		t.Fatalf("len(commands) = %d, want a bar plus a rect and a label per tab", len(commands))
	}
	activeRect, ok := commands[1].(render.DrawRect)
	if !ok {
		t.Fatalf("first header type = %T, want render.DrawRect", commands[1])
	}
	if activeRect.Fill != theme.Tabs.TabActive {
		t.Fatalf("first header fill = %v, want active color %v", activeRect.Fill, theme.Tabs.TabActive)
	}
	inactiveRect := commands[3].(render.DrawRect)
	if inactiveRect.Fill != theme.Tabs.TabInactive {
		t.Fatalf("second header fill = %v, want inactive color %v", inactiveRect.Fill, theme.Tabs.TabInactive)
	}
	label, ok := commands[2].(render.DrawText)
	if !ok {
		t.Fatalf("first label type = %T, want render.DrawText", commands[2])
	}
	if label.Text != tabsLabels()[0] {
		t.Fatalf("first label = %q, want %q", label.Text, tabsLabels()[0])
	}
}

func TestTabsGeometrySplitsBarIntoEqualHeaders(t *testing.T) {
	rect := tabsRect()

	bar := tabsBarRect(rect, 32)
	if bar.Min != (math2d.Vec2{X: 100, Y: 100}) || bar.Width() != 300 || bar.Height() != 32 {
		t.Fatalf("tabsBarRect() = %+v, want the top 32px band", bar)
	}

	body := tabsBodyRect(rect, 32)
	if body.Min != (math2d.Vec2{X: 100, Y: 132}) || body.Width() != 300 || body.Height() != 168 {
		t.Fatalf("tabsBodyRect() = %+v, want the area below the band", body)
	}

	second := tabHeaderRect(rect, 1, 3, 32)
	if second.Min != (math2d.Vec2{X: 200, Y: 100}) || second.Width() != 100 || second.Height() != 32 {
		t.Fatalf("tabHeaderRect(1) = %+v, want the middle third of the band", second)
	}
}

// A bar taller than the widget must not produce a negative body, and a widget
// with no tabs must not divide by zero.
func TestTabsGeometryDegeneratesSafely(t *testing.T) {
	short := math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 100, Y: 10})

	if body := tabsBodyRect(short, 32); body.Height() != 0 {
		t.Fatalf("tabsBodyRect() height = %v, want 0 for a widget shorter than the bar", body.Height())
	}
	if bar := tabsBarRect(short, 32); bar.Height() != 10 {
		t.Fatalf("tabsBarRect() height = %v, want the widget height 10", bar.Height())
	}
	if header := tabHeaderRect(tabsRect(), 0, 0, 32); header.Width() != 0 {
		t.Fatalf("tabHeaderRect(count=0) = %+v, want an empty rect", header)
	}
}

func TestTabsBodyRectIsExposedThroughBothFaces(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	want := tabsBodyRect(tabsRect(), DefaultTheme().Tabs.BarHeight)

	draw := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	if got := draw.Draw().TabsBodyRect(tabsRect()); got != want {
		t.Fatalf("Drawer.TabsBodyRect() = %+v, want %+v", got, want)
	}

	update := tabsUpdate(&State{})
	if got := update.Interact().TabsBodyRect(tabsRect()); got != want {
		t.Fatalf("Interactor.TabsBodyRect() = %+v, want %+v", got, want)
	}
}

func TestTitledPanelDrawsPanelBarAndTitle(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	theme := DefaultTheme()
	draw := New(&State{}, math2d.Vec2{}, true, cam, theme, cam.Viewport, &store, 0, 0)

	draw.titledPanelIn(tabsRect(), "Inspector", SpaceRef)

	commands := draw.Commands()
	if len(commands) != 3 {
		t.Fatalf("len(commands) = %d, want panel, title bar and title", len(commands))
	}
	bar, ok := commands[1].(render.DrawRect)
	if !ok {
		t.Fatalf("title bar type = %T, want render.DrawRect", commands[1])
	}
	if bar.Rect.Height() != theme.Header.Height {
		t.Fatalf("title bar height = %v, want theme header height %v", bar.Rect.Height(), theme.Header.Height)
	}
	title, ok := commands[2].(render.DrawText)
	if !ok {
		t.Fatalf("title type = %T, want render.DrawText", commands[2])
	}
	if title.Text != "Inspector" {
		t.Fatalf("title = %q, want %q", title.Text, "Inspector")
	}
}
