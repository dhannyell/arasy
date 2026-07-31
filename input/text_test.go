package input

import (
	"math"
	"testing"
)

func TestTextEventCarriesCodepoint(t *testing.T) {
	event, ok := TextEvent('á')
	if !ok {
		t.Fatal("TextEvent('á') reported invalid")
	}
	if event.Kind != EventText {
		t.Fatalf("kind = %v, want EventText", event.Kind)
	}
	if event.Source.Device != DeviceKeyboard || event.Source.Code != KeyUnknown {
		t.Fatalf("source = %+v, want keyboard/KeyUnknown", event.Source)
	}
	if got, ok := event.Rune(); !ok || got != 'á' {
		t.Fatalf("Rune() = %q, %v, want 'á', true", got, ok)
	}
}

func TestTextEventRejectsInvalidRunes(t *testing.T) {
	for _, r := range []rune{-1, 0xD800, 0x110000} {
		if _, ok := TextEvent(r); ok {
			t.Fatalf("TextEvent(%d) reported valid", r)
		}
	}
}

func TestTextEventValidRejectsGarbageValues(t *testing.T) {
	for name, value := range map[string]float64{
		"nan":        math.NaN(),
		"inf":        math.Inf(1),
		"fractional": 65.5,
		"negative":   -1,
		"surrogate":  0xDC00,
	} {
		event := Event{Source: Keyboard(KeyUnknown), Kind: EventText, Value: value}
		if event.Valid() {
			t.Fatalf("%s: Valid() = true, want false", name)
		}
	}
}

func TestTextEventRequiresKeyboardDevice(t *testing.T) {
	event := Event{Source: Source{Device: DeviceMouse, Code: MouseLeft}, Kind: EventText, Value: 'a'}
	if event.Valid() {
		t.Fatal("Valid() = true for a mouse-sourced text event")
	}
}

func TestKeyUnknownStaysInvalidForButtons(t *testing.T) {
	event := Event{Source: Keyboard(KeyUnknown), Kind: EventButtonDown}
	if event.Valid() {
		t.Fatal("Valid() = true for a button event on KeyUnknown")
	}
}

func TestEditingKeysAreValidSources(t *testing.T) {
	for _, code := range []SourceCode{KeyBackspace, KeyDelete, KeyHome, KeyEnd, KeyPageUp, KeyPageDown} {
		if !Keyboard(code).Valid() {
			t.Fatalf("Keyboard(%d).Valid() = false", code)
		}
	}
}

func TestMapperIgnoresTextEvents(t *testing.T) {
	mapper := NewMapper([]Binding{{Source: Keyboard(KeyA), Action: ActionConfirm, Scale: 1}})
	event, _ := TextEvent('a')
	mapper.Apply(event)

	state := mapper.State()
	if state.Pressed(ActionConfirm) {
		t.Fatal("text event mapped to an action")
	}
	if state.LastInputDevice() != DeviceUnknown {
		t.Fatalf("LastInputDevice() = %v, want unchanged", state.LastInputDevice())
	}
}

func TestKeyFrameCollectsTextAndEdges(t *testing.T) {
	var keyboard KeyboardState
	frame := keyboard.BeginFrame()
	for _, r := range "ab" {
		event, _ := TextEvent(r)
		frame = frame.AddEvent(event)
	}
	frame = frame.AddEvent(Event{Source: Keyboard(KeyBackspace), Kind: EventButtonDown})

	if string(frame.Text) != "ab" {
		t.Fatalf("Text = %q, want \"ab\"", string(frame.Text))
	}
	if !frame.JustPressed(KeyBackspace) || !frame.Down(KeyBackspace) {
		t.Fatal("backspace press missing from the frame")
	}
	if frame.Down(KeyDelete) {
		t.Fatal("delete reported down")
	}
}

func TestKeyFrameKeepsHeldKeysAcrossFrames(t *testing.T) {
	var keyboard KeyboardState
	keyboard.SetDown(KeyBackspace, true)

	frame := keyboard.BeginFrame()
	if !frame.Down(KeyBackspace) {
		t.Fatal("held key missing from the next frame")
	}
	if frame.JustPressed(KeyBackspace) {
		t.Fatal("a key held from an earlier tick reported a new down edge")
	}

	frame = frame.AddEvent(Event{Source: Keyboard(KeyBackspace), Kind: EventButtonUp})
	if frame.Down(KeyBackspace) {
		t.Fatal("release left the key down")
	}
}
