//go:build !(arr_dev && cgo && (windows || linux || darwin) && (amd64 || arm64))

package cli

import "github.com/dhannyell/arasy/runtime"

func watchLiveSupported() bool { return false }

func runWatchLive(*runtime.Driver, runtime.Game, string) error {
	panic("unreachable: watchLiveSupported() guards every call to runWatchLive")
}
