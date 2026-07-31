package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// A game that draws its own button has no other way to see focus: ui.State is
// private, so without this query the keyboard position is invisible to it.
func TestFocusedReportsTheFocusedWidget(t *testing.T) {
	state := &State{focusID: HashID("btn_sim")}
	cam := testInteractCamera()

	update := NewUpdate(state, pointerFrame(), cam, DefaultTheme(), cam.Viewport)
	if !update.focusedIn("btn_sim") {
		t.Fatal("Focused reported false for the focused widget in Update")
	}
	if update.focusedIn("btn_other") {
		t.Fatal("Focused reported true for an unfocused widget")
	}

	draw := New(state, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &AnimStore{}, 0, 0)
	if !draw.focusedIn("btn_sim") {
		t.Fatal("Focused reported false in Render; both phases must agree")
	}
}

func TestFocusedIsFalseWithoutFocus(t *testing.T) {
	cam := testInteractCamera()
	u := NewUpdate(&State{}, pointerFrame(), cam, DefaultTheme(), cam.Viewport)

	if u.focusedIn("btn_sim") {
		t.Fatal("Focused reported true with nothing focused")
	}
}

// The ring a game draws must be the ring the engine draws: same theme, same
// inset, so hand-drawn chrome cannot drift into a second focus language.
func TestFocusRingUsesTheThemeAndInset(t *testing.T) {
	cam := testCamera()
	theme := DefaultTheme()
	rect := math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 100, Y: 40})

	u := New(&State{}, math2d.Vec2{}, false, cam, theme, cam.Viewport, &AnimStore{}, 0, 0)
	u.focusRingIn(rect, u.nextLayer(), SpaceRef)

	cmds := u.Commands()
	if len(cmds) != 1 {
		t.Fatalf("FocusRing emitted %d commands, want 1", len(cmds))
	}
	ring, ok := cmds[0].(render.DrawRect)
	if !ok {
		t.Fatalf("command type = %T, want render.DrawRect", cmds[0])
	}
	if ring.Stroke != theme.Focus.Color || ring.StrokeWidth != theme.Focus.Width {
		t.Fatalf("ring stroke = %+v/%v, want the theme's", ring.Stroke, ring.StrokeWidth)
	}
	if ring.Fill.A != 0 {
		t.Fatal("the ring is an outline; it must not fill over the widget")
	}
	if want := rect.Min.X - theme.Focus.Inset; ring.Rect.Min.X != want {
		t.Fatalf("ring left edge = %v, want %v (inset outward)", ring.Rect.Min.X, want)
	}
}

func rowsRect() math2d.Rect {
	// Four rows of the default 32px row height fit in the view.
	return math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 200, Y: 128})
}

// navigating builds a tick in which navigation keys were pressed, with the
// given widget focused.
func navigating(state *State, focused string, pressed ...input.SourceCode) *UI {
	state.focusID = HashID(focused)
	return typing(state, "", pressed...)
}

func TestListSelectionFollowsArrowKeysWhenFocused(t *testing.T) {
	state := &State{}
	u := navigating(state, "roster", input.KeyArrowDown)

	next, changed := u.listInputValueIn("roster", rowsRect(), 32, 10, 2, SpaceRef)
	if next != 3 || !changed {
		t.Fatalf("ListInputValue() = (%d, %v), want (3, true)", next, changed)
	}
}

func TestListSelectionClampsAtTheEnds(t *testing.T) {
	state := &State{}
	if next, _ := navigating(state, "roster", input.KeyArrowUp).
		listInputValueIn("roster", rowsRect(), 32, 10, 0, SpaceRef); next != 0 {
		t.Fatalf("up from the first row = %d, want 0", next)
	}
	if next, _ := navigating(state, "roster", input.KeyArrowDown).
		listInputValueIn("roster", rowsRect(), 32, 10, 9, SpaceRef); next != 9 {
		t.Fatalf("down from the last row = %d, want 9", next)
	}
}

func TestListPageAndHomeEndKeys(t *testing.T) {
	cases := []struct {
		key      input.SourceCode
		selected int
		want     int
	}{
		{key: input.KeyPageDown, selected: 0, want: 4},
		{key: input.KeyPageUp, selected: 9, want: 5},
		{key: input.KeyHome, selected: 7, want: 0},
		{key: input.KeyEnd, selected: 0, want: 9},
	}
	for _, tc := range cases {
		state := &State{}
		next, _ := navigating(state, "roster", tc.key).
			listInputValueIn("roster", rowsRect(), 32, 10, tc.selected, SpaceRef)
		if next != tc.want {
			t.Fatalf("key %v from %d = %d, want %d", tc.key, tc.selected, next, tc.want)
		}
	}
}

func TestListIgnoresArrowKeysWithoutFocus(t *testing.T) {
	state := &State{}
	u := typing(state, "", input.KeyArrowDown)

	if next, changed := u.listInputValueIn("roster", rowsRect(), 32, 10, 2, SpaceRef); next != 2 || changed {
		t.Fatalf("ListInputValue() = (%d, %v), want the selection untouched", next, changed)
	}
}

// Walking the selection past the visible window has to bring the window along,
// or the player is moving something they cannot see.
func TestKeyboardSelectionScrollsTheRowIntoView(t *testing.T) {
	state := &State{}
	rect := rowsRect()
	u := navigating(state, "roster", input.KeyEnd)

	next, _ := u.listInputValueIn("roster", rect, 32, 10, 0, SpaceRef)
	if next != 9 {
		t.Fatalf("End selected %d, want 9", next)
	}
	want := float64(10)*32 - rect.Height()
	if got := state.scroll[HashID("roster")]; got != want {
		t.Fatalf("scroll = %v, want %v", got, want)
	}
}

func TestTableSelectionFollowsArrowKeysWhenFocused(t *testing.T) {
	state := &State{}
	rect := math2d.NewRect(math2d.Vec2{X: 10, Y: 10}, math2d.Vec2{X: 200, Y: 200})
	u := navigating(state, "players", input.KeyArrowDown)

	next, changed := u.tableInputValueIn("players", rect, []TableColumn{{Title: "Name"}}, 20, 4, SpaceRef)
	if next != 5 || !changed {
		t.Fatalf("TableInputValue() = (%d, %v), want (5, true)", next, changed)
	}
}

func TestRowSelectionRegistersAsFocusable(t *testing.T) {
	state := &State{}
	u := typing(state, "")
	u.listInputValueIn("roster", rowsRect(), 32, 10, 0, SpaceRef)

	if len(u.focusable) != 1 || u.focusable[0] != HashID("roster") {
		t.Fatalf("focusable = %v, want the list", u.focusable)
	}
}

// Without a focusable widget in the frame, Tab belongs to the game.
func TestTabIsLeftToTheGameWhenNothingIsFocusable(t *testing.T) {
	state := &State{focusID: HashID("stale")}
	cam := testInteractCamera()
	u := NewUpdateWithInput(state, pointerFrame(), inputStateJustPressed(input.ActionFocusNext), cam, DefaultTheme(), cam.Viewport)
	u.EndFrame()

	if state.focusID != HashID("stale") {
		t.Fatalf("focusID = %v, want it left alone", state.focusID)
	}
}

// A held arrow and a held backspace share one clock: two widgets reading the
// same key must not make it repeat twice as fast.
func TestHeldKeyAdvancesOneRepeatClockPerFrame(t *testing.T) {
	state := &State{focusID: HashID("roster"), repeatCode: input.KeyArrowDown}
	u := holding(state, input.KeyArrowDown)

	u.listInputValueIn("roster", rowsRect(), 32, 10, 0, SpaceRef)
	u.listInputValueIn("other", rowsRect(), 32, 10, 0, SpaceRef)

	if state.repeatTicks != 1 {
		t.Fatalf("repeatTicks = %d, want 1 after one frame", state.repeatTicks)
	}
}

func TestFocusedRowSelectionRepeatsOnTheTickClock(t *testing.T) {
	state := &State{focusID: HashID("roster")}
	selected := 0

	next, _ := navigating(state, "roster", input.KeyArrowDown).
		listInputValueIn("roster", rowsRect(), 32, 100, selected, SpaceRef)
	selected = next

	moves := 0
	for tick := 0; tick < textRepeatDelay+3*textRepeatRate; tick++ {
		u := holding(state, input.KeyArrowDown)
		state.focusID = HashID("roster")
		next, changed := u.listInputValueIn("roster", rowsRect(), 32, 100, selected, SpaceRef)
		if changed {
			moves++
		}
		selected = next
	}
	if moves != 4 {
		t.Fatalf("repeats = %d, want 4 (one at the delay, then one per rate)", moves)
	}
}
