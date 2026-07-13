//go:build js && wasm

package main

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/cmd/demos/wasmgolden"
	"github.com/dhannyell/frameforge/platform/wasm"
)

func main() {
	game, err := wasmgolden.New()
	if err != nil {
		panic(err)
	}

	params := js.Global().Get("URLSearchParams").New(js.Global().Get("location").Get("search"))
	forceCanvas2D := params.Call("get", "renderer").String() == "canvas2d"
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{ForceCanvas2D: forceCanvas2D, Log: log}); err != nil {
		log.Error("wasm golden host failed", "err", err)
	}
}
