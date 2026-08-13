package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/cmd/wasmgolden"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	fixture := flag.String("fixture", "rtt", "fixture: retained-batches, flip, rtt, or rtt-array")
	flag.Parse()

	game, err := wasmgolden.NewFixture(*fixture)
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Arasy " + *fixture + " Conformance",
		Width:  320,
		Height: 240,
		Log:    log,
	}); err != nil {
		log.Error("native conformance fixture failed", "fixture", *fixture, "err", err)
	}
}
