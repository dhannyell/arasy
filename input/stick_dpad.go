package input

import "math"

// Engage takes a firmer push than staying engaged: without the gap between
// the two thresholds, a stick resting near either one would flicker a grid
// cursor on and off, and a wobbling diagonal would alternate axes.
const (
	StickEngage  = 0.5
	StickRelease = 0.35

	// stickSwitchRatio is how dominant the other axis must become before an
	// engaged direction hands over to it -- the diagonal-wobble guard.
	stickSwitchRatio = 1.25
)

// StickDPad reads an analog stick as a four-way pad, which is what a grid
// cursor or a menu wants from it. The current direction is hysteresis state:
// exported so it can live inside snapshotted game state, zero value neutral.
type StickDPad struct {
	DX, DY int
}

// Update folds one tick's stick position into the pad and returns the held
// direction: one of the four axis directions, or zero when the stick is
// neutral. Digital input composes for free -- a dpad bound to the same axes
// arrives as ±1 and passes both thresholds.
func (s *StickDPad) Update(x, y float64) (dx, dy int) {
	absX, absY := math.Abs(x), math.Abs(y)

	if s.DX != 0 || s.DY != 0 {
		// Engaged: hold the direction while its own axis stays firm, unless
		// the other axis has clearly taken over.
		along, across := absX, absY
		if s.DY != 0 {
			along, across = absY, absX
		}
		if along >= StickRelease && across < along*stickSwitchRatio {
			s.alignSigns(x, y)
			return s.DX, s.DY
		}
		s.DX, s.DY = 0, 0
	}

	if absX < StickEngage && absY < StickEngage {
		return 0, 0
	}
	if absX >= absY {
		s.DX = sign(x)
	} else {
		s.DY = sign(y)
	}
	return s.DX, s.DY
}

// alignSigns lets a held axis reverse without passing through neutral: a flick
// from full left to full right is still that axis, just the other way.
func (s *StickDPad) alignSigns(x, y float64) {
	if s.DX != 0 {
		s.DX = sign(x)
	}
	if s.DY != 0 {
		s.DY = sign(y)
	}
}

func sign(v float64) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}
