//go:build js && wasm

package main

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/cmd/wasmgolden"
	"github.com/dhannyell/frameforge/platform/wasm"
	"github.com/dhannyell/frameforge/runtime"
)

func main() {
	params := js.Global().Get("URLSearchParams").New(js.Global().Get("location").Get("search"))

	var game runtime.Game
	var err error
	switch params.Call("get", "fixture").String() {
	case "particles":
		game, err = wasmgolden.NewParticleGolden()
	case "sprite-array":
		game, err = wasmgolden.NewSpriteArrayGolden()
	default:
		game, err = wasmgolden.New()
	}
	if err != nil {
		panic(err)
	}

	forceCanvas2D := params.Call("get", "renderer").String() == "canvas2d"
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{ForceCanvas2D: forceCanvas2D, Log: log}); err != nil {
		log.Error("wasm golden host failed", "err", err)
	}
}
