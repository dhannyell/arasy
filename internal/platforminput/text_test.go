package platforminput_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
)

func TestTextEventAcceptsResolvedCharacters(t *testing.T) {
	for _, key := range []string{"a", "A", "ç", " ", "7", "€"} {
		event, ok := platforminput.TextEvent(key)
		if !ok {
			t.Fatalf("TextEvent(%q) reported not text", key)
		}
		if event.Kind != input.EventText {
			t.Fatalf("TextEvent(%q).Kind = %v, want EventText", key, event.Kind)
		}
		if got, _ := event.Rune(); string(got) != key {
			t.Fatalf("TextEvent(%q) carried %q", key, got)
		}
	}
}

// Named keys, dead keys and the placeholders an IME or a mobile virtual
// keyboard sends are not characters — they belong to the physical-key path,
// which is what keeps a held ArrowLeft out of a text field.
func TestTextEventRejectsNamedKeys(t *testing.T) {
	for _, key := range []string{"Enter", "ArrowLeft", "Backspace", "Dead", "Process", "Unidentified", "Shift", ""} {
		if _, ok := platforminput.TextEvent(key); ok {
			t.Fatalf("TextEvent(%q) reported text", key)
		}
	}
}

func TestKeyboardCodeCoversEditingKeys(t *testing.T) {
	want := map[string]input.SourceCode{
		"Backspace": input.KeyBackspace,
		"Delete":    input.KeyDelete,
		"Home":      input.KeyHome,
		"End":       input.KeyEnd,
		"PageUp":    input.KeyPageUp,
		"PageDown":  input.KeyPageDown,
	}
	for code, expected := range want {
		got, ok := platforminput.KeyboardCode(code)
		if !ok || got != expected {
			t.Fatalf("KeyboardCode(%q) = %v, %v", code, got, ok)
		}
	}
}
