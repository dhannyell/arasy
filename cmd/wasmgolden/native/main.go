package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/cmd/wasmgolden"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	array := flag.Bool("array", false, "exercise RTT while sprite texture arrays are enabled")
	flag.Parse()

	var game runtime.Game
	var err error
	if *array {
		game, err = wasmgolden.NewRTTArrayGolden()
	} else {
		game, err = wasmgolden.NewRTTGolden()
	}
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Arasy RTT Conformance",
		Width:  320,
		Height: 240,
		Log:    log,
	}); err != nil {
		log.Error("native RTT fixture failed", "err", err)
	}
}
