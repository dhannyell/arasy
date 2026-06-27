package input

type Mapper struct {
	bindings     []Binding
	sourceValues map[Source]float64
	previous     State
	current      State
}

func NewMapper(bindings []Binding) *Mapper {
	validBindings := make([]Binding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Valid() {
			validBindings = append(validBindings, binding)
		}
	}

	return &Mapper{
		bindings:     validBindings,
		sourceValues: make(map[Source]float64),
		previous:     NewState(),
		current:      NewState(),
	}
}

func (m *Mapper) Apply(event Event) {
	if !event.Valid() {
		return
	}

	if m.sourceValues == nil {
		m.sourceValues = make(map[Source]float64)
	}

	switch event.Kind {
	case EventButtonDown:
		m.sourceValues[event.Source] = 1
	case EventButtonUp:
		m.sourceValues[event.Source] = 0
	case EventAxisChanged:
		m.sourceValues[event.Source] = event.Value
	}
}

func (m *Mapper) State() State {
	if m.sourceValues == nil {
		m.sourceValues = make(map[Source]float64)
	}

	next := NewState()
	for _, binding := range m.bindings {
		value := m.sourceValues[binding.Source] * binding.Scale
		if value == 0 {
			continue
		}

		next.values[binding.Action] += value
	}

	for action, value := range next.values {
		if value > 1 {
			value = 1
		}

		if value < -1 {
			value = -1
		}

		next.values[action] = value
		next.pressed[action] = value != 0

		wasPressed := m.previous.pressed[action]
		next.justPressed[action] = !wasPressed && next.pressed[action]
		next.justReleased[action] = wasPressed && !next.pressed[action]
	}

	for action, wasPressed := range m.previous.pressed {
		if wasPressed && !next.pressed[action] {
			next.justReleased[action] = true
		}
	}

	m.current = next.clone()

	return next
}

func (m *Mapper) EndFrame() {
	m.previous = m.current.clone()
	m.current = NewState()
}

func (m *Mapper) Bindings() []Binding {
	return append([]Binding(nil), m.bindings...)
}
