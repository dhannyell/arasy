package input_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
)

func TestBufferStoresEventsByTick(t *testing.T) {
	buffer := input.NewBuffer()
	event := input.Event{
		Tick: 12,
		Source: input.Source{
			Device: input.DeviceKeyboard,
			Code:   input.KeyW,
		},
		Kind: input.EventButtonDown,
	}

	buffer.Push(event)

	events := buffer.Events(12)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0] != event {
		t.Fatalf("got event %+v, want %+v", events[0], event)
	}
}

func TestBufferEventsReturnsCopy(t *testing.T) {
	buffer := input.NewBuffer()
	event := input.Event{
		Tick: 12,
		Source: input.Source{
			Device: input.DeviceKeyboard,
			Code:   input.KeyW,
		},
		Kind: input.EventButtonDown,
	}

	buffer.Push(event)

	events := buffer.Events(12)
	events[0].Kind = input.EventButtonUp

	got := buffer.Events(12)
	if got[0] != event {
		t.Fatalf("buffer event was modified through returned slice: got %+v, want %+v", got[0], event)
	}
}

func TestBufferClearBeforeRemovesOldTicks(t *testing.T) {
	buffer := input.NewBuffer()

	buffer.Push(input.Event{Tick: 1})
	buffer.Push(input.Event{Tick: 2})
	buffer.Push(input.Event{Tick: 3})

	buffer.ClearBefore(3)

	if events := buffer.Events(1); len(events) != 0 {
		t.Fatalf("tick 1 events = %d, want 0", len(events))
	}

	if events := buffer.Events(2); len(events) != 0 {
		t.Fatalf("tick 2 events = %d, want 0", len(events))
	}

	if events := buffer.Events(3); len(events) != 1 {
		t.Fatalf("tick 3 events = %d, want 1", len(events))
	}
}
