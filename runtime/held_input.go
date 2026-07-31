package runtime

import (
	"sort"

	"github.com/dhannyell/arasy/input"
)

// A press recorded in the journal belongs to the recording, not to the
// player. While the driver is consuming that journal the two are the same
// thing — reconstructing a tick where a key was down should leave that key
// down. The moment the driver stops consuming it, they part ways: the release
// that would have ended the press is still in the journal, at a tick the
// driver will now never reach, so the key stays down forever.
//
// heldInput is what makes that endable. It is the set of sources the input
// pipeline currently considers active, maintained from the same events the
// journal replays, so the driver can end exactly those presses and nothing
// else when it takes control.

// heldInput is one active source: a button that is down, or a gamepad axis
// resting away from centre.
type heldInput struct {
	source input.Source
	value  float64
	axis   bool
}

// heldBefore orders by device then code, so the release events derived from
// this set are emitted in a fixed order. They go into the journal, and a
// journal that replays in a different order is not a recording.
func heldBefore(a, b input.Source) bool {
	if a.Device != b.Device {
		return a.Device < b.Device
	}
	return a.Code < b.Code
}

// track folds one event into the held set.
func (i *driverInput) track(event input.Event) {
	switch event.Kind {
	case input.EventButtonDown:
		i.setHeld(heldInput{source: event.Source, value: 1})
	case input.EventButtonUp:
		i.clearHeld(event.Source)
	case input.EventAxisChanged:
		// Only sticks and triggers rest anywhere: a mouse wheel reports
		// impulses, and an impulse held forever is exactly the bug this file
		// exists to prevent.
		if event.Source.Device != input.DeviceGamepad {
			return
		}
		if event.Value == 0 {
			i.clearHeld(event.Source)
			return
		}
		i.setHeld(heldInput{source: event.Source, value: event.Value, axis: true})
	case input.EventPointerLeave:
		// The pointer leaving drops its buttons, mirroring what the pointer
		// frame already does with them.
		i.clearHeldDevice(input.DeviceMouse)
	}
}

func (i *driverInput) setHeld(held heldInput) {
	at := sort.Search(len(i.held), func(index int) bool {
		return !heldBefore(i.held[index].source, held.source)
	})
	if at < len(i.held) && i.held[at].source == held.source {
		i.held[at] = held
		return
	}
	i.held = append(i.held, heldInput{})
	copy(i.held[at+1:], i.held[at:])
	i.held[at] = held
}

func (i *driverInput) clearHeld(source input.Source) {
	at := sort.Search(len(i.held), func(index int) bool {
		return !heldBefore(i.held[index].source, source)
	})
	if at >= len(i.held) || i.held[at].source != source {
		return
	}
	i.held = append(i.held[:at], i.held[at+1:]...)
}

func (i *driverInput) clearHeldDevice(device input.DeviceKind) {
	kept := i.held[:0]
	for _, held := range i.held {
		if held.source.Device != device || held.axis {
			kept = append(kept, held)
		}
	}
	i.held = kept
}

// releaseEvents ends every active press. They are ordinary input events on
// purpose: fed through ApplyInput they enter the journal like any other, so a
// later scrub back across this point reproduces the release instead of
// reconstructing a world where the key was never let go.
func (i *driverInput) releaseEvents() []input.Event {
	if len(i.held) == 0 {
		return nil
	}

	pos := i.pointer.Pos()
	events := make([]input.Event, 0, len(i.held))
	for _, held := range i.held {
		event := input.Event{Source: held.source, Kind: input.EventButtonUp}
		if held.axis {
			// An axis is not released, it is re-centred.
			event.Kind = input.EventAxisChanged
			event.Value = 0
		}
		if held.source.Device == input.DeviceMouse {
			event.X, event.Y = pos.X, pos.Y
		}
		events = append(events, event)
	}
	return events
}
