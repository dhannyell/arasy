//go:build js && wasm && arr_dev

package wasm

import (
	"log/slog"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/devpanel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// panelOverlay adapts the dev panel to the host's devOverlay seam.
type panelOverlay struct {
	panel *devpanel.Panel
}

func (o *panelOverlay) HandleInput(events []input.Event, viewport math2d.Rect) []input.Event {
	return o.panel.HandleInput(events, viewport)
}

func (o *panelOverlay) Render(cam render.Camera, elapsed time.Duration, stats overlayStats) []render.Command {
	return o.panel.Render(cam, elapsed, devpanel.Stats{
		Commands:        stats.Commands,
		CPUTick:         stats.CPUTick,
		CPURender:       stats.CPURender,
		CPUSet:          stats.CPUSet,
		InputLatency:    stats.InputLatency,
		InputLatencySet: stats.InputLatencySet,
		HeapInUse:       stats.HeapInUse,
		HeapTotal:       stats.HeapTotal,
		HeapSet:         stats.HeapSet,
	})
}

func (o *panelOverlay) Toggle()       { o.panel.Toggle() }
func (o *panelOverlay) Visible() bool { return o.panel.Visible() }

// mountDevPanel builds the tooling overlay. The whole browser-specific part
// of it is the file I/O and the key that opens it; everything else is the
// same panel the native host mounts.
func (h *host) mountDevPanel(devMode bool, newGame func() runtime.Game, log *slog.Logger) {
	opts := devpanel.Options{
		DevMode: devMode,
		NewGame: newGame,
		Log:     log,
		Adopt: func(driver *runtime.Driver) {
			h.driver = driver
			h.resize()
		},
	}
	var panel *devpanel.Panel
	if newGame != nil {
		opts.Recording = newRecordingPort(log, func(data []byte, name string) {
			panel.ApplyRecording(data, name)
		})
	}
	panel = devpanel.New(h.driver, opts)
	h.panel = &panelOverlay{panel: panel}
}
