package input

type Buffer struct {
	events map[uint64][]Event
}

func NewBuffer() *Buffer {
	return &Buffer{
		events: make(map[uint64][]Event),
	}
}

func (b *Buffer) Push(event Event) {
	if b.events == nil {
		b.events = make(map[uint64][]Event)
	}

	tick := event.Tick
	b.events[tick] = append(b.events[tick], event)
}

func (b *Buffer) Events(tick uint64) []Event {
	return append([]Event(nil), b.events[tick]...)
}

func (b *Buffer) ClearBefore(tick uint64) {
	for eventTick := range b.events {
		if eventTick < tick {
			delete(b.events, eventTick)
		}
	}
}
