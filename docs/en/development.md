# Development Commands And Gates

## Tests

Run the full suite before handoff. Use focused commands while developing.

```powershell
go test ./...
go test ./scene -run TestStep
go test ./scene -run="^$" -bench=. -benchmem
golangci-lint run ./...
```

## Benchmarks

The benchmark suite registry lives in `cmd/benchsave/suites.go`.

```powershell
go run ./cmd/benchsave -list-suites
go run ./cmd/benchsave -suite <name>
go run ./cmd/benchsave -suite all
go run ./cmd/benchsave -suite <name> -compare baseline
go run ./cmd/benchsave -suite <name> -compare auto
go run ./cmd/benchsave -suite <name> -update-baseline
go run ./cmd/benchsave -diff <old> <new>
go run ./cmd/benchsave -compare-all
```

## Demos and hosts

```powershell
go run ./cmd/demos/football/raster
go run -tags arr_dev ./examples/breakout/native
go run ./cmd/demos/assets/raster
go run ./cmd/demos/assets/canvas
go run ./cmd/demos/football/canvas
go run ./cmd/demos/breakout/canvas
go run ./cmd/wasmserve -dir output/wasm/breakout -addr 127.0.0.1:8090
go run ./examples/rhythm/raster
go run ./cmd/arasy dev ./examples/rhythm/wasm -a 127.0.0.1:8090
go run ./cmd/arasy build ./examples/breakout/wasm -o dist/breakout
go run ./cmd/arasy run ./examples/breakout/native
go run ./cmd/arasy dev -p native ./examples/breakout/native
go run ./cmd/arasy doctor -p native ./examples/breakout/native
go run ./cmd/arasy watch <corpus> <replay>.arrp [--browser]
go run -tags arr_dev ./cmd/arasy watch <corpus> <replay>.arrp --live
```

Native demos open the development panel only in `arr_dev` builds. The default
CLI build does not include the native host, so it can install without GLFW or
ALSA headers. Offline watch mode creates a GIF. Live watch mode opens a native
window. Browser watch mode serves the WASM dev panel.

## Golden files

Regenerate a golden only after an intentional visual or simulation change. The
pull request must explain the change.

```powershell
go test ./platform/raster -run TestGolden -update
npm run update:wasm-golden
go test ./examples/breakout ./examples/2048 ./examples/rhythm -run TestGoldenReplay -update
```

Golden replay fixtures contain a committed `.arrp` and `.state.txt` under each
demo test directory. A visual or simulation golden diff without an explanation
is a regression signal.

## WebAssembly build

Use the JS/WASM build when a change affects the browser host or shared engine
packages.

```powershell
$env:GOOS="js"; $env:GOARCH="wasm"; go build ./...; Remove-Item Env:GOOS,Env:GOARCH
```

## CI gates

| Job | Checks |
|---|---|
| `quality` | actionlint, `golangci-lint`, and `govulncheck` |
| `test` | gofmt, `go vet`, and `go test ./...` on x86-64 |
| `no-default-font` | tests and JS/WASM build with `arr_no_default_font` |
| `race` | `go test -race ./...` |
| `windows` | `go test ./...` on Windows |
| `wasm` | JS/WASM build, vet, simulation tests, replay tests, and Canvas2D golden |
| `arm64` | `go test ./...` on native arm64 |

The WASM and arm64 jobs provide the cross-architecture determinism check. The
checksum golden must match x86-64, arm64, and WASM. WebGPU visual goldens remain
a local browser gate until a GPU-backed runner exists.

## Default font

The engine embeds Go Mono for `FontID(0)`. Text layout remains deterministic.
`arasy build --no-default-font` removes the TTF and uses the built-in 7x13
bitmap font.

Raster goldens have embedded-font and bitmap variants. Regenerate the bitmap
variant with:

```powershell
go test -tags arr_no_default_font ./platform/raster -run TestGolden -update
```
