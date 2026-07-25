//go:build windows && amd64

package native

import (
	"fmt"
	"image"
	"log/slog"
	goruntime "runtime"
	"time"

	"github.com/dhannyell/arasy/runtime"
	"github.com/go-gl/glfw/v3.4/glfw"
)

// FrameMetrics describes native frame pacing and host/renderer timings.
type FrameMetrics struct {
	FPS           float64
	FrameTime     time.Duration
	MeanFrameTime time.Duration
	P99FrameTime  time.Duration

	InputToSubmit    time.Duration
	InputToSubmitSet bool

	// HeapInUse and HeapSys are Go process memory measurements.
	HeapInUse uint64
	HeapSys   uint64
	HeapSet   bool

	GPUBufferBytes uint64
	GPUBufferSet   bool

	// GPUFrameTime is asynchronous submit-to-completion latency, not GPU
	// execution time. Timestamp queries are unavailable in the current binding.
	GPUFrameTime    time.Duration
	GPUFrameTimeSet bool

	CPUTickTime   time.Duration
	CPURenderTime time.Duration
	CPUTimingSet  bool

	CPUTessTime     time.Duration
	CPUUploadTime   time.Duration
	CPUEncodeTime   time.Duration
	CPUBreakdownSet bool
}

const (
	defaultTitle  = "Arasy"
	defaultWidth  = 1280
	defaultHeight = 720
)

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
	// Title is the window title. The default is "Arasy".
	Title string
	// Width is the initial logical window width. The default is 1280.
	Width int
	// Height is the initial logical window height. The default is 720.
	Height int
	// Fullscreen starts in borderless fullscreen mode.
	Fullscreen bool
	// VSync selects the surface presentation mode. The zero value is VSyncOn.
	VSync VSyncMode
	// Icon is an optional window icon.
	Icon image.Image
	// Log receives host diagnostics. The default is slog.Default().
	Log *slog.Logger
	// Metrics emits periodic frame-pacing reports through Log.
	Metrics bool
	// GPUTiming enables the asynchronous GPU completion probe for metric reports.
	GPUTiming bool
	// FrameMetrics receives periodic frame-pacing reports. Nil disables delivery.
	FrameMetrics func(FrameMetrics)

	// SmoothPacing enables a ~1ms busy-spin tail on the frame limiter for tighter
	// pacing (video capture, latency benchmarks) at the cost of one busy core.
	// Off by default: the plain sleep cap keeps CPU low.
	SmoothPacing bool
}

func normalizedOptions(opts Options) Options {
	if opts.Title == "" {
		opts.Title = defaultTitle
	}
	if opts.Width <= 0 {
		opts.Width = defaultWidth
	}
	if opts.Height <= 0 {
		opts.Height = defaultHeight
	}
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	return opts
}

// Run builds a driver for game and starts the native host. It blocks on the
// calling goroutine until the window closes.
func Run(game runtime.Game, opts Options) error {
	opts = normalizedOptions(opts)
	driver := runtime.NewDriver(game, opts.Log)
	return RunDriver(driver, game, opts)
}

// RunDriver starts the native host for an existing driver. The game parameter
// mirrors the other interactive hosts so callers can use a consistent API when
// reconstructing or verifying game state.
func RunDriver(driver *runtime.Driver, game runtime.Game, opts Options) error {
	if driver == nil {
		return fmt.Errorf("native host requires a driver")
	}
	_ = game
	opts = normalizedOptions(opts)

	// GLFW and its callbacks must remain on the OS thread that initialized it.
	goruntime.LockOSThread()
	if err := glfw.Init(); err != nil {
		return fmt.Errorf("initialize GLFW: %w", err)
	}
	defer glfw.Terminate()

	window, err := createWindow(opts)
	if err != nil {
		return err
	}
	defer window.Destroy()

	cfg := driver.Config()
	renderer, err := NewRenderer(window, cfg.Clear, cfg.Assets, opts.VSync, opts.Log)
	if err != nil {
		return err
	}
	renderer.SetCPUTiming(opts.Metrics || opts.FrameMetrics != nil)
	renderer.SetGPUTiming(opts.GPUTiming)
	defer renderer.Close()

	h := newHost(driver, renderer, window, opts)
	h.registerCallbacks()

	var sink *audioSink
	if len(cfg.Assets.AudioClips()) > 0 {
		sink, err = newAudioSink(cfg.Assets.AudioClips(), cfg.Assets.FS(), cfg.SimulationHz, opts.Log)
		if err != nil {
			opts.Log.Warn("native audio disabled", "err", err)
			sink = nil
		}
	}
	defer sink.close()

	h.audio = sink
	return h.run()
}
