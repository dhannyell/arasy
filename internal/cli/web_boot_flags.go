package cli

import (
	"github.com/dhannyell/arasy/internal/wasmserve"
	"github.com/spf13/cobra"
)

type webBootFlags struct {
	noLoading        bool
	noSplash         bool
	noEngineSplash   bool
	streamingCompile bool
	bootBackground   string
}

func (f *webBootFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.noLoading, "no-loading", false, "omit the web download poster and progress indicator")
	cmd.Flags().BoolVar(&f.noSplash, "no-splash", false, "skip runtime splash screens without disabling download progress")
	cmd.Flags().BoolVar(&f.noEngineSplash, "no-engine-splash", false, "skip only the animated engine brand, preserving game splash screens")
	cmd.Flags().BoolVar(&f.streamingCompile, "streaming-compile", false, "overlap WASM download and compilation (progress bar becomes indeterminate)")
	cmd.Flags().StringVar(&f.bootBackground, "boot-background", "", "web boot color (#RRGGBB; default matches the Arasy poster)")
}

func (f webBootFlags) options() (wasmserve.BootOptions, error) {
	options := wasmserve.BootOptions{
		DisableLoadingIndicator: f.noLoading,
		DisableRuntimeSplash:    f.noSplash,
		DisableEngineSplash:     f.noEngineSplash,
		StreamingCompile:        f.streamingCompile,
		Background:              f.bootBackground,
	}
	if err := options.Validate(); err != nil {
		return wasmserve.BootOptions{}, err
	}
	return options, nil
}
