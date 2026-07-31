package runtime

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/input"
)

func textRecording(events ...input.Event) Recording {
	return Recording{Ticks: 2, Seed: 7, Events: events}
}

func TestRecordingRoundTripsTextEvents(t *testing.T) {
	typed, ok := input.TextEvent('ç')
	if !ok {
		t.Fatal("TextEvent reported invalid")
	}
	typed.Tick = 1
	rec := textRecording(typed)

	var buf bytes.Buffer
	if _, err := rec.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	got, err := ReadRecording(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("ReadRecording returned error: %v", err)
	}
	if !reflect.DeepEqual(got.Events, rec.Events) {
		t.Fatalf("events = %#v, want %#v", got.Events, rec.Events)
	}
	if r, ok := got.Events[0].Rune(); !ok || r != 'ç' {
		t.Fatalf("decoded rune = %q, %v, want 'ç', true", r, ok)
	}
}

// A text event must cost exactly what every other event costs: the codepoint
// rides in the Value field the format already carries, so adding text to the
// vocabulary leaves the byte layout — and every recording written before it —
// untouched.
func TestTextEventDoesNotChangeRecordingLayout(t *testing.T) {
	typed, _ := input.TextEvent('a')
	button := input.Event{Source: input.Keyboard(input.KeyA), Kind: input.EventButtonDown}

	var withText, withButton bytes.Buffer
	if _, err := textRecording(typed).WriteTo(&withText); err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	if _, err := textRecording(button).WriteTo(&withButton); err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	if withText.Len() != withButton.Len() {
		t.Fatalf("text recording = %d bytes, button recording = %d", withText.Len(), withButton.Len())
	}
}

func TestReadRecordingRejectsInvalidCodepoint(t *testing.T) {
	broken := input.Event{Source: input.Keyboard(input.KeyUnknown), Kind: input.EventText, Value: 0.5}
	var buf bytes.Buffer
	if _, err := textRecording(broken).WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo returned error: %v", err)
	}
	if _, err := ReadRecording(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("ReadRecording accepted a text event with a fractional codepoint")
	}
}
