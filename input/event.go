package input

type EventKind uint8

const (
	EventButtonDown EventKind = iota
	EventButtonUp
	EventAxisChanged
)

func (k EventKind) Valid() bool {
	switch k {
	case EventButtonDown, EventButtonUp, EventAxisChanged:
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
}

func (e Event) Valid() bool {
	return e.Source.Valid() && e.Kind.Valid()
}
