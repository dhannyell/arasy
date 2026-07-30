package input

import "maps"

type Mapper struct {
	bindings        []Binding
	sourceValues    map[Source]float64
	gamepadDeadzone float64
	// latched holds button sources that reported a down event during the
	// current tick's input batch, even if a later event in that same batch
	// brought the raw value back to zero. Hosts collect OS/browser events
	// per render frame and apply them in one batch stamped with the current
	// tick, so a press+release that both land inside one batch would
	// otherwise collapse to "never pressed" and silently drop the tap.
	// The latch guarantees the tap is reported as pressed for exactly one
	// tick; EndFrame clears it so the release surfaces on the next tick.
	latched  map[Source]bool
	previous State
	current  State
}

func NewMapper(bindings []Binding) *Mapper {
	return NewMapperWithDeadzone(bindings, 0)
}

// NewMapperWithDeadzone constructs a mapper that transforms values from
// DeviceGamepad sources before applying Binding.Scale. Zero or an invalid
// deadzone selects DefaultGamepadDeadzone. abs(value) <= deadzone becomes
// zero; values above it are rescaled to the remaining range.
func NewMapperWithDeadzone(bindings []Binding, deadzone float64) *Mapper {
	validBindings := make([]Binding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Valid() {
			validBindings = append(validBindings, binding)
		}
	}

	return &Mapper{
		bindings:        validBindings,
		sourceValues:    make(map[Source]float64),
		gamepadDeadzone: EffectiveGamepadDeadzone(deadzone),
		latched:         make(map[Source]bool),
		previous:        NewState(),
		current:         NewState(),
	}
}

func (m *Mapper) Apply(event Event) {
	if !event.Valid() {
		return
	}

	if m.sourceValues == nil {
		m.sourceValues = make(map[Source]float64)
	}

	if m.latched == nil {
		m.latched = make(map[Source]bool)
	}

	switch event.Kind {
	case EventButtonDown:
		m.sourceValues[event.Source] = 1
		m.latched[event.Source] = true
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
		raw := m.sourceValues[binding.Source]
		if raw == 0 && m.latched[binding.Source] {
			raw = 1
		}

		if binding.Source.Device == DeviceGamepad {
			raw = applyEffectiveGamepadDeadzone(raw, m.gamepadDeadzone)
		}
		value := raw * binding.Scale
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
	clear(m.latched)
}

func (m *Mapper) Reset() {
	m.sourceValues = make(map[Source]float64)
	m.latched = make(map[Source]bool)
	m.previous = NewState()
	m.current = NewState()
}

func (m *Mapper) Clone() *Mapper {
	clone := &Mapper{
		bindings:        append([]Binding(nil), m.bindings...),
		sourceValues:    make(map[Source]float64, len(m.sourceValues)),
		gamepadDeadzone: m.gamepadDeadzone,
		latched:         make(map[Source]bool, len(m.latched)),
		previous:        m.previous.clone(),
		current:         m.current.clone(),
	}

	maps.Copy(clone.sourceValues, m.sourceValues)

	maps.Copy(clone.latched, m.latched)

	return clone
}

// SetGamepadDeadzone updates the deterministic gamepad transform, resolving
// zero or an invalid value to DefaultGamepadDeadzone.
func (m *Mapper) SetGamepadDeadzone(deadzone float64) {
	m.gamepadDeadzone = EffectiveGamepadDeadzone(deadzone)
}

func (m *Mapper) Bindings() []Binding {
	return append([]Binding(nil), m.bindings...)
}
