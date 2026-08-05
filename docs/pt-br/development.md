# Comandos E Gates De Desenvolvimento

## Testes

Rode a suíte completa antes do handoff. Use comandos focados durante o
desenvolvimento.

```powershell
go test ./...
go test ./scene -run TestStep
go test ./scene -run="^$" -bench=. -benchmem
golangci-lint run ./...
```

## Benchmarks

O registro das suítes fica em `cmd/benchsave/suites.go`.

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

## Demos E Hosts

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

Demos native abrem o painel de desenvolvimento somente em builds `arr_dev`. O
build padrão da CLI não inclui o host native, então pode ser instalado sem
headers de GLFW ou ALSA. Watch offline cria um GIF. Watch live abre uma janela
native. Watch browser serve o painel WASM.

## Arquivos golden

Regenere um golden somente depois de uma mudança visual ou de simulação
intencional. O pull request deve explicar a mudança.

```powershell
go test ./platform/raster -run TestGolden -update
npm run update:wasm-golden
go test ./examples/breakout ./examples/2048 ./examples/rhythm -run TestGoldenReplay -update
```

Fixtures de golden replay contêm um `.arrp` e um `.state.txt` commitados no
diretório de testes de cada demo. Um diff de golden visual ou de simulação sem
explicação é um sinal de regressão.

## Build WebAssembly

Use o build JS/WASM quando uma mudança afetar o host browser ou pacotes
compartilhados da engine.

```powershell
$env:GOOS="js"; $env:GOARCH="wasm"; go build ./...; Remove-Item Env:GOOS,Env:GOARCH
```

## Gates de CI

| Job | Checks |
|---|---|
| `quality` | actionlint, `golangci-lint` e `govulncheck` |
| `test` | gofmt, `go vet` e `go test ./...` em x86-64 |
| `no-default-font` | testes e build JS/WASM com `arr_no_default_font` |
| `race` | `go test -race ./...` |
| `windows` | `go test ./...` em Windows |
| `wasm` | build JS/WASM, vet, testes de simulação, replay e golden Canvas2D |
| `arm64` | `go test ./...` em arm64 native |

Os jobs WASM e arm64 formam a verificação de determinismo entre arquiteturas.
O golden de checksum deve coincidir em x86-64, arm64 e WASM. Goldens visuais
WebGPU continuam sendo um gate local de browser até existir um runner com GPU.

## Fonte padrão

A engine embute Go Mono para `FontID(0)`. O layout de texto continua
determinístico. `arasy build --no-default-font` remove o TTF e usa a fonte
bitmap interna 7x13.

Goldens raster têm variantes com fonte embutida e bitmap. Regenere a variante
bitmap com:

```powershell
go test -tags arr_no_default_font ./platform/raster -run TestGolden -update
```
