package ui

// State persists across frames and is CAPTURED in the driverSnapshot (cloned),
// ensuring release semantics survive a rewind. It belongs to the Driver.
type State struct {
	activeID ID
}

// Clone is used by the runtime driver when capturing rewind snapshots.
func (s State) Clone() State {
	return s
}
