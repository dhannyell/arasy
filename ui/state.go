package ui

// State persists across frames and is CAPTURED in the driverSnapshot (cloned),
// ensuring release semantics survive a rewind. It belongs to the Driver.
type State struct {
	activeID ID
}

func (s State) Clone() State {
	return s
}
