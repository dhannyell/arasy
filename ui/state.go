package ui

import "maps"

// State persists across frames and is CAPTURED in the driverSnapshot (cloned),
// ensuring release semantics survive a rewind. It belongs to the Driver.
type State struct {
	activeID ID
	focusID  ID
	scroll   map[ID]float64
}

// Clone is used by the runtime driver when capturing rewind snapshots.
func (s State) Clone() State {
	return State{
		activeID: s.activeID,
		focusID:  s.focusID,
		scroll:   maps.Clone(s.scroll),
	}
}
