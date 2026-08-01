//go:build !(windows && amd64)

package sysprobe

import "time"

// Prober is the fallback for operating systems without a probe
// implementation: every Sample reports nothing, and the dev panel hides the
// system rows. Porting to a new OS means adding a sibling file with a real
// Prober — callers never change.
type Prober struct{}

// NewProber returns the zero prober.
func NewProber(vendorID, deviceID uint32) *Prober { return &Prober{} }

// Sample reports an empty reading: no group OK.
func (p *Prober) Sample(now time.Time) Sample { return Sample{} }

// Close releases nothing; the zero prober holds no handles.
func (p *Prober) Close() {}
