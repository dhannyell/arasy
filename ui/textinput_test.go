package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func textRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 200, Y: 32})
}

// typing builds the update-phase UI of a tick in which text and keys arrived.
func typing(state *State, text string, pressed ...input.SourceCode) *UI {
	cam := testInteractCamera()
	var keyboard input.KeyboardState
	frame := keyboard.BeginFrame()
	for _, r := range text {
		event, _ := input.TextEvent(r)
		frame = frame.AddEvent(event)
	}
	for _, code := range pressed {
		frame = frame.AddEvent(input.Event{Source: input.Keyboard(code), Kind: input.EventButtonDown})
	}
	return NewUpdateWithKeys(state, input.PointerFrame{}, frame, input.NewState(), cam, DefaultTheme(), cam.Viewport)
}

// holding builds a tick in which code stayed down without a new press edge.
func holding(state *State, code input.SourceCode) *UI {
	cam := testInteractCamera()
	var keyboard input.KeyboardState
	keyboard.SetDown(code, true)
	return NewUpdateWithKeys(state, input.PointerFrame{}, keyboard.BeginFrame(), input.NewState(), cam, DefaultTheme(), cam.Viewport)
}

func focusedState(id string) *State {
	return &State{focusID: HashID(id)}
}

func TestTextInputInsertsTypedRunesAtCaret(t *testing.T) {
	state := focusedState("search")
	u := typing(state, "ab")

	got := u.textInputIn("search", textRect(), "", TextInputOptions{}, SpaceRef)

	if got.Text != "ab" || !got.Changed {
		t.Fatalf("TextInput() = %+v, want text \"ab\" changed", got)
	}
	if state.caret[HashID("search")] != 2 {
		t.Fatalf("caret = %d, want 2", state.caret[HashID("search")])
	}
}

func TestTextInputInsertsInTheMiddle(t *testing.T) {
	state := focusedState("search")
	state.caret = map[ID]int{HashID("search"): 1}
	u := typing(state, "X")

	if got := u.textInputIn("search", textRect(), "ac", TextInputOptions{}, SpaceRef); got.Text != "aXc" {
		t.Fatalf("TextInput() = %q, want \"aXc\"", got.Text)
	}
}

func TestTextInputIgnoresKeysWithoutFocus(t *testing.T) {
	state := &State{}
	u := typing(state, "ab", input.KeyBackspace)

	if got := u.textInputIn("search", textRect(), "hi", TextInputOptions{}, SpaceRef); got.Text != "hi" || got.Changed {
		t.Fatalf("TextInput() = %+v, want the value untouched", got)
	}
}

func TestTextInputRespectsMaxRunes(t *testing.T) {
	state := focusedState("search")
	state.caret = map[ID]int{HashID("search"): 2}
	u := typing(state, "cd")

	if got := u.textInputIn("search", textRect(), "ab", TextInputOptions{MaxRunes: 3}, SpaceRef); got.Text != "abc" {
		t.Fatalf("TextInput() = %q, want \"abc\"", got.Text)
	}
}

func TestTextInputEditingKeysMoveAndDelete(t *testing.T) {
	cases := []struct {
		name   string
		caret  int
		keys   []input.SourceCode
		text   string
		want   string
		caret2 int
	}{
		{name: "backspace", caret: 3, keys: []input.SourceCode{input.KeyBackspace}, text: "abc", want: "ab", caret2: 2},
		{name: "backspace at start", caret: 0, keys: []input.SourceCode{input.KeyBackspace}, text: "abc", want: "abc", caret2: 0},
		{name: "delete", caret: 0, keys: []input.SourceCode{input.KeyDelete}, text: "abc", want: "bc", caret2: 0},
		{name: "delete at end", caret: 3, keys: []input.SourceCode{input.KeyDelete}, text: "abc", want: "abc", caret2: 3},
		{name: "left", caret: 2, keys: []input.SourceCode{input.KeyArrowLeft}, text: "abc", want: "abc", caret2: 1},
		{name: "right", caret: 0, keys: []input.SourceCode{input.KeyArrowRight}, text: "abc", want: "abc", caret2: 1},
		{name: "home", caret: 3, keys: []input.SourceCode{input.KeyHome}, text: "abc", want: "abc", caret2: 0},
		{name: "end", caret: 0, keys: []input.SourceCode{input.KeyEnd}, text: "abc", want: "abc", caret2: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := focusedState("search")
			state.caret = map[ID]int{HashID("search"): tc.caret}
			u := typing(state, "", tc.keys...)

			got := u.textInputIn("search", textRect(), tc.text, TextInputOptions{}, SpaceRef)
			if got.Text != tc.want {
				t.Fatalf("text = %q, want %q", got.Text, tc.want)
			}
			if caret := state.caret[HashID("search")]; caret != tc.caret2 {
				t.Fatalf("caret = %d, want %d", caret, tc.caret2)
			}
		})
	}
}

func TestTextInputEnterSubmits(t *testing.T) {
	state := focusedState("search")
	u := typing(state, "", input.KeyEnter)

	if got := u.textInputIn("search", textRect(), "hi", TextInputOptions{}, SpaceRef); !got.Submitted {
		t.Fatal("Enter did not submit")
	}
}

// A held backspace must wait out the delay and then repeat on the engine's own
// clock, so a replay deletes exactly as many characters as the live run did.
func TestTextInputRepeatsHeldBackspaceOnTickClock(t *testing.T) {
	state := focusedState("search")
	text := "abcdefghij"
	state.caret = map[ID]int{HashID("search"): len([]rune(text))}

	// The press edge deletes once and starts the clock.
	text = typing(state, "", input.KeyBackspace).
		textInputIn("search", textRect(), text, TextInputOptions{}, SpaceRef).Text
	if text != "abcdefghi" {
		t.Fatalf("after press: %q", text)
	}

	deletions := 0
	for tick := 0; tick < textRepeatDelay+3*textRepeatRate; tick++ {
		next := holding(state, input.KeyBackspace).
			textInputIn("search", textRect(), text, TextInputOptions{}, SpaceRef)
		if next.Text != text {
			deletions++
		}
		text = next.Text
	}
	if deletions != 4 {
		t.Fatalf("repeats = %d, want 4 (one at the delay, then one per rate)", deletions)
	}
}

func TestTextInputStopsRepeatingAfterRelease(t *testing.T) {
	state := focusedState("search")
	state.caret = map[ID]int{HashID("search"): 3}
	text := "abc"

	typing(state, "", input.KeyBackspace).textInputIn("search", textRect(), text, TextInputOptions{}, SpaceRef)
	for tick := 0; tick < textRepeatDelay*2; tick++ {
		// No key held: the clock must not advance.
		u := typing(state, "")
		u.textInputIn("search", textRect(), "ab", TextInputOptions{}, SpaceRef)
	}
	if state.repeatCode != input.KeyUnknown || state.repeatTicks != 0 {
		t.Fatalf("repeat clock = (%v, %d), want cleared", state.repeatCode, state.repeatTicks)
	}
}

func TestTextInputClickFocusesAndPlacesCaret(t *testing.T) {
	state := &State{}
	rect := textRect()
	cam := testInteractCamera()
	advance := render.MeasureText("nnn", DefaultTheme().TextInput.TextSize).X
	pos := math2d.Vec2{X: rect.Min.X + DefaultTheme().TextInput.PaddingX + advance, Y: rect.Center().Y}
	u := NewUpdateWithKeys(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), input.KeyFrame{}, input.NewState(), cam, DefaultTheme(), cam.Viewport)

	u.textInputIn("search", rect, "abcdef", TextInputOptions{}, SpaceRef)

	if state.focusID != HashID("search") {
		t.Fatal("click did not focus the field")
	}
	if got := state.caret[HashID("search")]; got != 3 {
		t.Fatalf("caret = %d, want 3", got)
	}
}

func TestTextInputClickOutsideBlurs(t *testing.T) {
	state := focusedState("search")
	cam := testInteractCamera()
	u := NewUpdateWithKeys(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 380, Y: 380}},
	), input.KeyFrame{}, input.NewState(), cam, DefaultTheme(), cam.Viewport)

	u.textInputIn("search", textRect(), "abc", TextInputOptions{}, SpaceRef)

	if state.focusID != noID {
		t.Fatal("a click outside the field kept focus")
	}
}

func TestTextInputRegistersAsFocusable(t *testing.T) {
	state := &State{}
	u := typing(state, "")
	u.textInputIn("search", textRect(), "", TextInputOptions{}, SpaceRef)

	if len(u.focusable) != 1 || u.focusable[0] != HashID("search") {
		t.Fatalf("focusable = %v, want the field", u.focusable)
	}
}

func TestTextInputDrawsCaretOnlyWhenFocused(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	caretCount := func(state *State) int {
		u := New(state, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
		u.textInputDrawIn("search", textRect(), "abc", TextInputOptions{}, SpaceRef)
		return len(u.Commands())
	}

	blurred := caretCount(&State{})
	focused := caretCount(focusedState("search"))
	if focused != blurred+1 {
		t.Fatalf("focused emitted %d commands, blurred %d — want exactly one more (the caret)", focused, blurred)
	}
}

func TestTextInputDrawsPlaceholderWhenEmpty(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	u.textInputDrawIn("search", textRect(), "", TextInputOptions{Placeholder: "Search"}, SpaceRef)

	for _, cmd := range u.Commands() {
		if text, ok := cmd.(render.DrawText); ok {
			if text.Text != "Search" {
				t.Fatalf("drew %q, want the placeholder", text.Text)
			}
			return
		}
	}
	t.Fatal("no text drawn")
}

func TestCaretBlinksOffTheRenderClock(t *testing.T) {
	if !caretVisible(0, 1) || caretVisible(0.75, 1) {
		t.Fatal("caret blink does not follow the render clock")
	}
	if !caretVisible(3, 0) {
		t.Fatal("a zero period should keep the caret solid")
	}
}
