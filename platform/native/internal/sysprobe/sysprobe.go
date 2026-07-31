// Package sysprobe samples OS-level process and system metrics — RAM, VRAM
// and GPU utilization — for the dev panel's Perf tab. The point of every
// reading is the split it carries: the process number is the game, the
// system number is everything else on the machine, so "the frame spiked
// because the GPU was busy" stops meaning "...with my game" by default.
//
// Each OS implements NewProber in a sibling file (windows.go today; a Linux
// or macOS port adds linux.go/darwin.go and nothing else changes). An OS
// without an implementation gets the zero prober from unsupported.go, whose
// samples report nothing — and the panel simply hides those rows, so the
// absence of a probe is never an error worth logging.
package sysprobe

import "time"

// Sample is one reading of every probe the OS could provide. Each group is
// valid only when its OK flag is set; the rest stay zero.
type Sample struct {
	// MemProcessBytes is this process's working set. MemSystemUsedBytes and
	// MemSystemTotalBytes are physical memory across the whole machine.
	MemProcessBytes     uint64
	MemSystemUsedBytes  uint64
	MemSystemTotalBytes uint64
	MemOK               bool

	// VRAMProcessBytes is this process's video memory on the render adapter;
	// VRAMBudgetBytes is the budget the OS grants the process there.
	VRAMProcessBytes uint64
	VRAMBudgetBytes  uint64
	VRAMOK           bool

	// GPU utilization in percent (0-100): the engines this process runs
	// versus every engine on the system.
	GPUProcessPercent float64
	GPUSystemPercent  float64
	GPUOK             bool
}

// proberInterval is the minimum time between two real samples. OS probes are
// called once per frame by the host, and none of these numbers moves fast
// enough to justify more than one reading a second.
const proberInterval = time.Second
