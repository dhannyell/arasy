//go:build !((windows || linux || darwin) && (amd64 || arm64))

// This file is the out-of-reach half of the package. It exists so a game
// built for a platform the native host does not cover fails with a sentence
// that names the platform, instead of "undefined: native.Run".
//
// The declarations below mirror the real ones in run.go. Keep them in step:
// the build-tag guard in buildtags_test.go watches the constraints, not the
// field lists, so a field added to Options there has to be added here too.

package native

import (
	"fmt"
	"image"
	"log/slog"
	goruntime "runtime"
	"time"

	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// FrameMetrics describes native frame pacing and host/renderer timings.
type FrameMetrics struct {
	FPS           float64
	FrameTime     time.Duration
	MeanFrameTime time.Duration
	P99FrameTime  time.Duration

	InputToSubmit    time.Duration
	InputToSubmitSet bool

	HeapInUse uint64
	HeapSys   uint64
	HeapSet   bool

	GPUBufferBytes uint64
	GPUBufferSet   bool

	GPUFrameTime    time.Duration
	GPUFrameTimeSet bool

	GPUPrepareTime  time.Duration
	GPUSimulateTime time.Duration
	GPURenderTime   time.Duration
	GPUTimingSet    bool

	CPUTickTime   time.Duration
	CPURenderTime time.Duration
	CPUTimingSet  bool

	CPUTessTime     time.Duration
	CPUUploadTime   time.Duration
	CPUEncodeTime   time.Duration
	CPUBreakdownSet bool
}

// VSyncMode selects how the native surface presents completed frames.
type VSyncMode uint8

const (
	// VSyncOn uses FIFO presentation, the default mode guaranteed by WebGPU.
	VSyncOn VSyncMode = iota
	// VSyncOff prefers an uncapped presentation mode when the surface supports one.
	VSyncOff
)

// Options configures the native desktop host.
type Options struct {
	Title        string
	Width        int
	Height       int
	Fullscreen   bool
	VSync        VSyncMode
	MSAA         render.MSAAMode
	Icon         image.Image
	Log          *slog.Logger
	Metrics      bool
	GPUTiming    bool
	FrameMetrics func(FrameMetrics)
	DevTools     bool
	RecordReplay bool
	GameFactory  func() runtime.Game
}

// Run reports that this platform is out of the native host's reach.
func Run(runtime.Game, Options) error { return errUnsupported() }

// RunDriver reports that this platform is out of the native host's reach.
func RunDriver(*runtime.Driver, runtime.Game, Options) error { return errUnsupported() }

func errUnsupported() error {
	return fmt.Errorf("native host is not supported on %s/%s", goruntime.GOOS, goruntime.GOARCH)
}
