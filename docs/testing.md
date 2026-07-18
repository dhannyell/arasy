# Estrutura de testes

Este documento registra a convencao de organizacao dos testes da FrameForge.
O objetivo e manter o padrao idiomatico de Go, com testes perto do codigo, sem
deixar os pacotes virarem arquivos gigantes por acumulacao.

## Principios

- Testes ficam no mesmo diretorio do pacote que exercitam.
- Arquivos de teste sao nomeados por comportamento ou responsabilidade, nao
  necessariamente por espelhar um arquivo de producao.
- Preferir varios arquivos pequenos e focados a um unico `*_test.go` grande.
- Testes devem documentar comportamento observavel e decisoes de simulacao.
- Helpers de teste devem ser pequenos, explicitos e locais ao pacote.
- Benchmarks ficam separados em `*_benchmark_test.go`.

## Pacote do teste

Use `package foo_test` por padrao quando o teste puder validar a API publica do
pacote. Isso mantem o acoplamento baixo e simula como usuarios reais usam o
codigo.

Exemplos atuais:

```text
scene      -> package scene_test
render     -> package render_test
input      -> package input_test
math2d     -> package math2d_test
platform/raster -> package raster_test
```

Use `package foo` quando o contrato importante depende de estado interno do
pacote ou quando o teste e uma caracterizacao deliberada de uma implementacao
interna.

Exemplos atuais:

```text
runtime -> package runtime
ui      -> package ui
platform/wasm -> package wasm
```

Essa escolha deve ser intencional. Ao adicionar um novo arquivo de teste em um
pacote misto, siga o estilo existente do assunto que esta sendo testado.

## Nome dos arquivos

Nomeie arquivos por area de comportamento:

```text
collision_bounds_test.go
collision_detection_test.go
collision_grid_test.go
collision_resolution_test.go
driver_rewind_test.go
driver_ui_test.go
renderer_text_test.go
camera_aspect_test.go
```

Evite nomes vagos quando o arquivo ja passou de alguns poucos testes:

```text
collision_test.go
driver_test.go
renderer_test.go
misc_test.go
```

Esses nomes ainda sao aceitaveis para pacotes pequenos, mas quando o arquivo
comeca a misturar familias de comportamento, divida.

## Organizacao recomendada por pacote

### `scene`

`scene` concentra simulacao, ECS, colisao, debug e checksum. Os testes devem
ficar separados por sistema ou contrato:

```text
world_test.go
movement_test.go
step_test.go
checksum_test.go
collision_bounds_test.go
collision_detection_test.go
collision_grid_test.go
collision_resolution_test.go
collision_shapes_test.go
collision_helpers_test.go
render_test.go
debug_test.go
collision_benchmark_test.go
step_benchmark_test.go
```

Ao adicionar componente novo ao `World`, cubra pelo menos:

- set/get/remove;
- `Clone`;
- `Despawn`;
- `Checksum`, quando o componente fizer parte do estado de simulacao.

### `runtime`

`runtime` define a semantica de tick, input, rewind, replay e UI runtime. Os
testes devem ser agrupados pelo contrato do driver:

```text
driver_camera_test.go
driver_step_test.go
driver_rewind_test.go
driver_state_test.go
driver_ui_test.go
driver_public_test.go
driver_characterization_test.go
input_test.go
replay_test.go
scrub_test.go
debug_test.go
seek_benchmark_test.go
```

Use nomes explicitos quando um teste for regressao de bug ou caracterizacao de
semantica. Comentarios sao bem-vindos quando explicam a historia do bug ou a
razao deterministica da expectativa.

### `platform/raster`

O rasterizador tem testes pixel-level. Separe por primitiva ou regra de
composicao:

```text
renderer_core_test.go
renderer_rect_test.go
renderer_circle_test.go
renderer_line_test.go
renderer_arc_test.go
renderer_text_test.go
renderer_layer_test.go
renderer_helpers_test.go
run_test.go
renderer_benchmark_test.go
```

Mantenha assertions de pixel em helpers pequenos, mas deixe o comando render
visivel no teste. Isso torna falhas visuais mais faceis de entender.

### `platform/wasm`

O pipeline WebGPU separa a tessellacao pura em `platform/wasm/tess`; mantenha
ali testes semanticos de vertices, clipping, atlas e painter order. Canvas2D e
WebGPU tambem possuem goldens visuais executados no Chromium por Playwright:

```powershell
npm ci
npx playwright install chromium # uma vez por ambiente
npm run test:wasm-golden
npm run update:wasm-golden # somente apos mudanca visual intencional
```

Os projetos `canvas2d` e `webgpu` usam baselines separados. O teste WebGPU deve
falhar se o host cair silenciosamente para Canvas2D. Artefatos de diferenca
ficam em `test-results/` e nao devem ser commitados.

### `render`

Separe transformacao de camera, aspect ratio, cores e comandos:

```text
camera_transform_test.go
camera_aspect_test.go
color_test.go
command_test.go
```

### Pacotes pequenos

Pacotes como `math2d`, `loop`, `input` e `ui` podem continuar com poucos
arquivos enquanto cada arquivo contar uma unica historia clara.

## Helpers

Use helpers quando eles removem repeticao sem esconder o caso de teste.

Bons exemplos:

```go
func assertVec2Near(t *testing.T, got, want math2d.Vec2)
func assertPixel(t *testing.T, got color.Color, want color.RGBA)
func spawnCollidableCircle(w *scene.World, pos math2d.Vec2, radius float64) scene.EntityID
```

Regras para helpers:

- chamar `t.Helper()`;
- ficar em `*_helpers_test.go` quando usados por varios arquivos;
- ficar no proprio arquivo quando usados por uma unica familia de testes;
- evitar DSLs grandes que escondem o estado montado.

## Integracao e testes pesados

Testes de integracao podem ficar no pacote que exercitam quando ainda sao
rapidos e estaveis. Se um teste precisar de servidor, WebSocket, tempo real ou
jogo completo, deixe isso claro no nome:

```text
scrub_integration_test.go
```

Se testes Go de integracao ficarem lentos ou instaveis, use build tag:

```go
//go:build integration
```

Nesse caso, o fluxo passa a ser:

```powershell
go test ./...
go test -tags=integration ./caminho/do/pacote
```

Nao use build tag cedo demais. Enquanto a suite completa continuar rapida e
estavel, manter tudo em `go test ./...` e melhor para feedback.

Testes que precisam de DOM, Canvas2D ou WebGPU nao usam `go_js_wasm_exec`, que
nao fornece um browser. Mantenha-os no harness Playwright e execute-os pelo
comando `npm run test:wasm-golden`.

## Conformidade cross-backend

`cmd/wasmgolden/conformance_test.go` renderiza a cena canonica do
`wasmgolden` no raster (in-process, sem browser) e diffa contra os goldens
Playwright commitados de canvas2d e webgpu, com orcamento de pixels
divergentes calibrado por fixture. Roda em `go test ./cmd/wasmgolden` em
toda perna nativa do CI. Regenerou golden do browser
(`npm run update:wasm-golden`)? Rode a conformidade em seguida.

## Benchmarks

Benchmarks ficam em arquivos dedicados:

```text
collision_benchmark_test.go
step_benchmark_test.go
seek_benchmark_test.go
renderer_benchmark_test.go
```

Quando o benchmark virar uma familia oficial, registre a suite em
`cmd/benchsave/suites.go` para que `go run ./cmd/benchsave -list-suites` a
encontre.

## Quando dividir um arquivo

Divida um arquivo de teste quando uma destas condicoes aparecer:

- ele mistura duas ou mais familias de comportamento;
- ele passa de algumas centenas de linhas e a navegacao fica cansativa;
- novos testes estao sendo adicionados longe dos helpers ou fixtures que usam;
- o nome do arquivo ja nao descreve bem o conteudo.

Ao dividir, prefira mover testes inteiros sem alterar comportamento. Depois, em
um segundo passo, considere extrair helpers se a repeticao continuar clara.

## Comandos de verificacao

Antes de finalizar mudancas de teste:

```powershell
gofmt -w <arquivos alterados>
go test ./...
npm run test:wasm-golden
```

Para checar cobertura de um pacote especifico:

```powershell
go test ./render -cover
go test ./scene -cover
go test ./runtime -cover
```
