//go:build js && wasm && !arr_dev

package wasm

import (
	"log/slog"

	"github.com/dhannyell/arasy/runtime"
)

func devToolsWantGPUTiming(bool) bool { return false }

// mountDevPanel is the release-build stub: the dev panel compiles in only
// under the arr_dev tag, so a shipped game cannot open it — the code is not
// in the binary at all. Asking for it here means a release build kept the
// development options on, which is worth one log line.
func (h *host) mountDevPanel(devMode bool, newGame func() runtime.Game, log *slog.Logger) {
	if devMode || newGame != nil {
		log.Warn("dev panel requested but not compiled in; rebuild with -tags arr_dev")
	}
}
