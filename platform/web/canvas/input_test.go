package canvas

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
)

func TestBrowserKeyboardCodeMapsMovementKeys(t *testing.T) {
	tests := map[string]input.SourceCode{
		"KeyW":       input.KeyW,
		"KeyA":       input.KeyA,
		"KeyS":       input.KeyS,
		"KeyD":       input.KeyD,
		"ArrowUp":    input.KeyArrowUp,
		"ArrowDown":  input.KeyArrowDown,
		"ArrowLeft":  input.KeyArrowLeft,
		"ArrowRight": input.KeyArrowRight,
		"Tab":        input.KeyTab,
		"Enter":      input.KeyEnter,
		"Escape":     input.KeyEscape,
	}

	for browserCode, want := range tests {
		got, ok := BrowserKeyboardCode(browserCode)
		if !ok {
			t.Fatalf("BrowserKeyboardCode(%q) ok = false, want true", browserCode)
		}

		if got != want {
			t.Fatalf("BrowserKeyboardCode(%q) = %v, want %v", browserCode, got, want)
		}
	}
}

func TestBrowserKeyboardCodeRejectsUnknownCode(t *testing.T) {
	if got, ok := BrowserKeyboardCode("Digit1"); ok || got != input.KeyUnknown {
		t.Fatalf("BrowserKeyboardCode(Digit1) = %v, %v; want KeyUnknown, false", got, ok)
	}
}

func TestInputEventFromClientMessageMapsKeyboardDownAndUp(t *testing.T) {
	down, ok := inputEventFromClientMessage(clientMessage{
		Device: "keyboard",
		Code:   "KeyW",
		Kind:   "down",
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage(down) ok = false, want true")
	}

	if down.Source.Device != input.DeviceKeyboard || down.Source.Code != input.KeyW {
		t.Fatalf("down source = %#v, want keyboard KeyW", down.Source)
	}

	if down.Kind != input.EventButtonDown {
		t.Fatalf("down kind = %v, want EventButtonDown", down.Kind)
	}

	up, ok := inputEventFromClientMessage(clientMessage{
		Device: "keyboard",
		Code:   "KeyW",
		Kind:   "up",
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage(up) ok = false, want true")
	}

	if up.Kind != input.EventButtonUp {
		t.Fatalf("up kind = %v, want EventButtonUp", up.Kind)
	}
}

func TestInputEventFromClientMessageRejectsUnsupportedInput(t *testing.T) {
	tests := []clientMessage{
		{Device: "mouse", Code: "KeyW", Kind: "down"},
		{Device: "keyboard", Code: "Digit1", Kind: "down"},
		{Device: "keyboard", Code: "KeyW", Kind: "repeat"},
	}

	for _, message := range tests {
		if event, ok := inputEventFromClientMessage(message); ok {
			t.Fatalf("inputEventFromClientMessage(%#v) = %#v, true; want false", message, event)
		}
	}
}
