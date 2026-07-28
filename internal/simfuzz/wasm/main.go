//go:build js && wasm

// Command wasm is the browser viewer for a recorded fuzz case: it fetches
// the corpus and the paired .arrp recording from the server that serves it
// (see internal/cli's `arasy watch --browser`, which stages them as
// watch-corpus and watch-replay.arrp next to this binary), reconstructs them
// with runtime.ReplayForViewing, and plays the recording once on load. The
// existing dev panel (Backquote/~) exposes Play/Pause/Step/Scrub so the
// recording can be explored interactively instead of only watched once.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"syscall/js"

	"github.com/dhannyell/arasy/internal/simfuzz"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)

	corpus, err := fetchBytes("watch-corpus")
	if err != nil {
		log.Error("fetch corpus", "err", err)
		return
	}
	replayBytes, err := fetchBytes("watch-replay.arrp")
	if err != nil {
		log.Error("fetch replay", "err", err)
		return
	}
	rec, err := runtime.ReadRecordingFromBytes(replayBytes)
	if err != nil {
		log.Error("decode replay", "err", err)
		return
	}

	game := simfuzz.NewViewerGame(simfuzz.DecodeScript(corpus))
	probe := runtime.ReplayForViewing(game, log, rec)
	game.FitContentToRecording(probe)
	driver := runtime.ReplayForViewing(game, log, rec)

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: 0})
	driver.AdvanceTime(0)
	driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})

	if err := wasm.RunDriver(driver, game, wasm.Options{
		DevTools: true,
		Log:      log,
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}

// fetchBytes reads a same-origin static file served alongside this WASM
// binary. net/http's client needs an absolute URL even for a same-origin
// request under GOOS=js, hence the explicit origin.
func fetchBytes(path string) ([]byte, error) {
	origin := js.Global().Get("location").Get("origin").String()
	resp, err := http.Get(origin + "/" + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %s", path, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
