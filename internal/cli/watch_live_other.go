//go:build !(windows && amd64)

package cli

import "github.com/dhannyell/arasy/runtime"

func watchLiveSupported() bool { return false }

func runWatchLive(*runtime.Driver, runtime.Game, string) error {
	panic("unreachable: watchLiveSupported() guards every call to runWatchLive")
}
