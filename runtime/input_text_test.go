package runtime

import (
	"testing"

	"github.com/dhannyell/arasy/input"
)

func typed(t *testing.T, r rune) input.Event {
	t.Helper()
	event, ok := input.TextEvent(r)
	if !ok {
		t.Fatalf("TextEvent(%q) reported invalid", r)
	}
	return event
}

func TestDriverInputCollectsTypedText(t *testing.T) {
	di := newDriverInput(nil)
	di.Apply([]input.Event{typed(t, 'h'), typed(t, 'i')}, 0)

	if got := string(di.KeyFrame().Text); got != "hi" {
		t.Fatalf("KeyFrame().Text = %q, want \"hi\"", got)
	}
}

func TestDriverInputClearsTextEachTick(t *testing.T) {
	di := newDriverInput(nil)
	di.Apply([]input.Event{typed(t, 'a')}, 0)
	di.EndTick(1)

	if got := di.KeyFrame().Text; len(got) != 0 {
		t.Fatalf("KeyFrame().Text = %q, want empty on the next tick", string(got))
	}
}

// Held keys outlive the tick they were pressed in — that is what lets a text
// field repeat backspace on its own clock — while the down edge does not.
func TestDriverInputKeepsKeysHeldAcrossTicks(t *testing.T) {
	di := newDriverInput(nil)
	di.Apply([]input.Event{{Source: input.Keyboard(input.KeyBackspace), Kind: input.EventButtonDown}}, 0)
	if !di.KeyFrame().JustPressed(input.KeyBackspace) {
		t.Fatal("press edge missing on the tick it arrived")
	}

	di.EndTick(1)
	frame := di.KeyFrame()
	if !frame.Down(input.KeyBackspace) {
		t.Fatal("key not held on the following tick")
	}
	if frame.JustPressed(input.KeyBackspace) {
		t.Fatal("press edge repeated on the following tick")
	}

	di.Apply([]input.Event{{Source: input.Keyboard(input.KeyBackspace), Kind: input.EventButtonUp}}, 1)
	di.EndTick(2)
	if di.KeyFrame().Down(input.KeyBackspace) {
		t.Fatal("key still held after release")
	}
}

func TestDriverInputSnapshotRestoresHeldKeys(t *testing.T) {
	di := newDriverInput(nil)
	di.Apply([]input.Event{{Source: input.Keyboard(input.KeyDelete), Kind: input.EventButtonDown}}, 0)
	di.EndTick(1)
	snapshot := di.Snapshot()

	di.Apply([]input.Event{{Source: input.Keyboard(input.KeyDelete), Kind: input.EventButtonUp}}, 1)
	di.EndTick(2)
	if di.KeyFrame().Down(input.KeyDelete) {
		t.Fatal("release did not land before the restore")
	}

	di.Restore(snapshot)
	if !di.KeyFrame().Down(input.KeyDelete) {
		t.Fatal("rewind lost the held key")
	}
}
