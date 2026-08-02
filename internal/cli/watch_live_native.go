//go:build cgo && (windows || linux || darwin) && (amd64 || arm64)

package cli

import (
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func watchLiveSupported() bool { return true }

// runWatchLive rewinds the reconstructed driver to the start and enters
// watch mode, then hands it to the native host: the window opens already
// playing and auto-pauses on the final tick (see runtime.Driver.playbackSteps),
// so this is a real-time, single-pass viewing of the exact recorded case.
func runWatchLive(driver *runtime.Driver, game runtime.Game, title string) error {
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	return native.RunDriver(driver, game, native.Options{
		Title:  title,
		Width:  900,
		Height: 900,
	})
}
