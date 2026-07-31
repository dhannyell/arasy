package input

import (
	"math"
	"unicode/utf8"
)

type EventKind uint8

const (
	EventButtonDown EventKind = iota
	EventButtonUp
	EventAxisChanged
	EventPointerMove
	EventPointerLeave
	// EventText carries one typed character, resolved by the platform, in
	// Value as a Unicode codepoint. It is the logical counterpart of the
	// physical key events above: hosts report the key position through
	// EventButtonDown/Up (so WASD stays WASD on an AZERTY layout) and the
	// character the layout produced through EventText. Every codepoint fits
	// exactly in float64, so text travels in the existing Value field and
	// needs no room of its own in the recording format.
	EventText
)

func (k EventKind) Valid() bool {
	switch k {
	case EventButtonDown,
		EventButtonUp,
		EventAxisChanged,
		EventPointerMove,
		EventPointerLeave,
		EventText:
		return true
	default:
		return false
	}
}

type Event struct {
	Tick   uint64
	Source Source
	Kind   EventKind
	Value  float64
	X, Y   float64
}

func (e Event) Valid() bool {
	// A text event has no key behind it — the character is already resolved,
	// so which physical key produced it is not part of the event. It carries
	// KeyUnknown, which Source.Valid rejects for buttons and must keep
	// rejecting: an unknown key is a translation failure, an unknown key on a
	// text event is the normal case.
	if e.Kind == EventText {
		return e.Source.Device == DeviceKeyboard && validCodepoint(e.Value)
	}
	return e.Source.Valid() && e.Kind.Valid()
}

// TextEvent builds the event for one typed character. It reports false for a
// rune no platform should have produced (surrogate halves, out of range), so
// hosts can forward whatever the OS handed them without pre-filtering.
func TextEvent(r rune) (Event, bool) {
	event := Event{
		Source: Keyboard(KeyUnknown),
		Kind:   EventText,
		Value:  float64(r),
	}
	return event, event.Valid()
}

// Rune returns the character of a text event. It reports false for any other
// kind of event, so callers can range over a batch without a type switch.
func (e Event) Rune() (rune, bool) {
	if e.Kind != EventText || !validCodepoint(e.Value) {
		return 0, false
	}
	return rune(e.Value), true
}

func validCodepoint(value float64) bool {
	if math.IsNaN(value) || value < 0 || value > utf8.MaxRune || value != math.Trunc(value) {
		return false
	}
	return utf8.ValidRune(rune(value))
}
