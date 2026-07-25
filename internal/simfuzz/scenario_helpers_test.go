package simfuzz

import (
	"github.com/dhannyell/arasy/input"
)

func appendDrivenEntity(script Script) Script {
	mask := CompTransform | CompVelocity
	entity := EntityDef{
		ComponentMask: mask,
		Data: concatBytes(
			packF64(-150), packF64(-150), packF64(0), packF64(1), packF64(1),
			packF64(0), packF64(0),
		),
	}
	if len(script.Entities) < MaxEntities {
		script.Entities = append(script.Entities, entity)
	} else {
		script.Entities[len(script.Entities)-1] = entity
	}
	return script
}

func withoutSpaceEvents(events []input.Event) []input.Event {
	return filterEventsBySource(events, input.Keyboard(input.KeySpace))
}

func filterEventsBySource(events []input.Event, source input.Source) []input.Event {
	filtered := make([]input.Event, 0, len(events))
	for _, event := range events {
		if event.Source != source {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
