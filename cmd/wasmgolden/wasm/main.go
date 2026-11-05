//go:build js && wasm

package main

import (
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/arasy/cmd/wasmgolden"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/render"
)

func main() {
	params := js.Global().Get("URLSearchParams").New(js.Global().Get("location").Get("search"))

	fixture := params.Call("get", "fixture")
	fixtureName := ""
	if fixture.Type() == js.TypeString {
		fixtureName = fixture.String()
	}
	game, err := wasmgolden.NewFixture(fixtureName)
	if err != nil {
		panic(err)
	}

	msaa := render.MSAADefault
	if params.Call("get", "msaa").String() == "off" {
		msaa = render.MSAAOff
	}
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		Log:  log,
		MSAA: msaa,
	}); err != nil {
		log.Error("wasm golden host failed", "err", err)
	}
}
