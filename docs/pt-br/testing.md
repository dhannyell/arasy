# Estrutura de testes

Este documento registra a convenção de organização dos testes da Arasy.
O objetivo e manter o padrão idiomático de Go, com testes perto do código, sem
deixar os pacotes virarem arquivos gigantes por acumulação.

## Principios

- Testes ficam no mesmo diretório do pacote que exercitam.
- Arquivos de teste são nomeados por comportamento ou responsabilidade, não
  necessariamente por espelhar um arquivo de produção.
- Preferir vários arquivos pequenos e focados a um unico `*_test.go` grande.
- Testes devem documentar comportamento observável e decisões de simulação.
- Helpers de teste devem ser pequenos, explícitos e locais ao pacote.
- Benchmarks ficam separados em `*_benchmark_test.go`.

## Pacote do teste

Use `package foo_test` por padrão quando o teste puder validar a API publica do
pacote. Isso mantém o acoplamento baixo e simula como usuários reais usam o
código.

Exemplos atuais:

```text
scene      -> package scene_test
render     -> package render_test
input      -> package input_test
math2d     -> package math2d_test
platform/raster -> package raster_test
```

Use `package foo` quando o contrato importante depende de estado interno do
pacote ou quando o teste e uma caracterização deliberada de uma implementação
interna.

Exemplos atuais:

```text
runtime -> package runtime
ui      -> package ui
platform/wasm -> package wasm
```

Essa escolha deve ser intencional. Ao adicionar um novo arquivo de teste num
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

Esses nomes ainda são aceitáveis para pacotes pequenos, mas quando o arquivo
começa a misturar famílias de comportamento, divida.

## Organizacao recomendada por pacote

### `scene`

`scene` concentra simulação, ECS, colisão, debug e checksum. Os testes devem
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
- `Checksum`, quando o componente fizer parte do estado de simulação.

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

Use nomes explícitos quando um teste for regressão de bug ou caracterização de
semantica. Comentários são bem-vindos quando explicam a história do bug ou a
razão determinística da expectativa.

### `platform/raster`

O rasterizador tem testes pixel-level. Separe por primitiva ou regra de
composição:

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
visível no teste. Isso torna falhas visuais mais fáceis de entender.

### `platform/wasm`

O pipeline WebGPU separa a tessellate pura em `internal/render/gpu/tess`; mantenha
ali testes semânticos de vertices, clipping, atlas e painter order. Canvas2D e
WebGPU também possuem goldens visuais executados no Chromium por Playwright:

```powershell
npm ci
npx playwright install chromium # uma vez por ambiente
npm run test:wasm-golden
npm run update:wasm-golden # somente apos mudanca visual intencional
```

Os projetos `canvas2d` e `webgpu` usam baselines separados. O teste WebGPU deve
falhar se o host cair silenciosamente para Canvas2D. Artefatos de diferença
ficam em `test-results/` e nao devem ser commitados.

### `render`

Separe transformação de camera, aspect ratio, cores e comandos:

```text
camera_transform_test.go
camera_aspect_test.go
color_test.go
command_test.go
```

### `internal/svggen`

Compilador SVG-para-Go (`arasy svg`), tool-time only. Cobertura por
características, nao por arquivo espelhado:

```text
parse_test.go      -> subconjunto aceito, rejeicoes, FuzzParseSVG
path_test.go        -> comandos de path, conversao de arco, FuzzParsePathData
generate_test.go    -> TestGenerateIsDeterministicAndFormatted (gera duas vezes, compara bytes)
```

`FuzzParseSVG`/`FuzzParsePathData` rodam como testes de regressao normais via
`go test ./...` (corpus commitado) e como fuzzer real sob demanda
(`go test ./internal/svggen -fuzz FuzzParseSVG -fuzztime 60s`), no mesmo
espirito do fuzzing de simulacao descrito abaixo. Veja
`docs/pt-br/vector-graphics.md` para o que o compilador aceita/rejeita.

### Pacotes pequenos

Pacotes como `math2d`, `loop`, `input` e `ui` podem continuar com poucos
arquivos enquanto cada arquivo contar uma unica historia clara.

## Helpers

Use helpers quando eles removem repetição sem esconder o caso de teste.

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

Testes de integração podem ficar no pacote que exercitam quando ainda são
rápidos e estáveis. Se um teste precisar de servidor, WebSocket, tempo real ou
jogo completo, deixe isso claro no nome:

```text
scrub_integration_test.go
```

Se testes, Go de integracao ficarem lentos ou instaveis, use build tag:

```go
//go:build integration
```

Nesse caso, o fluxo passa a ser:

```powershell
go test ./...
go test -tags=integration ./caminho/do/pacote
```

Não use build tag cedo demais. Enquanto a suíte completa continuar rápida e
estável, manter tudo em `go test ./...` e melhor para feedback.

Testes que precisam de DOM, Canvas2D ou WebGPU não usam `go_js_wasm_exec`, que
não fornece um browser. Mantenha-os no harness Playwright e execute-os pelo
comando `npm run test:wasm-golden`.

## Conformidade cross-backend

`cmd/wasmgolden/conformance_test.go` renderiza a cena canónica do
`wasmgolden` no raster (in-process, sem browser) e diffa contra os goldens
Playwright commitados de canvas2d e webgpu, com orçamento de pixels
divergentes calibrado por fixture. Roda em `go test ./cmd/wasmgolden` em
toda a perna nativa do CI. Regenerou golden do browser
(`npm run update:wasm-golden`)? Rode a conformidade em seguida.

## Benchmarks

Benchmarks ficam em arquivos dedicados:

```text
collision_benchmark_test.go
step_benchmark_test.go
seek_benchmark_test.go
renderer_benchmark_test.go
```

Quando o benchmark virar uma familia oficial, registre a suíte em
`cmd/benchsave/suites.go` para que `go run ./cmd/benchsave -list-suites` a
encontre.

## Quando dividir um arquivo

Divida um arquivo de teste quando uma destas condições aparecer:

- ele mistura duas ou mais famílias de comportamento;
- ele passa de algumas centenas de linhas e a navegação fica cansativa;
- novos testes estão sendo adicionados longe dos helpers ou fixtures que usam;
- o nome do arquivo já não descreve bem o conteúdo.

Ao dividir, prefira mover testes inteiros sem alterar comportamento. Depois, em
um segundo passo, considere extrair helpers se a repetição continuar clara.


## Fuzzing da simulacao

`internal/simfuzz` oferece doze alvos de fuzz que conduzem um `runtime.Game`
dedicado (o "fuzz game") por um `runtime.Driver` real, exercitando o pipeline
completo `input -> Driver -> World`. Cada alvo decodifica um `Script` dos bytes
do fuzzer (decodificador total e determinístico), spawna entidades com o
vocabulário completo de componentes do `scene` e replays um log monotónico de
eventos de input. P2 também conduz o caminho real de UI/jogo do Breakout, e P5
é semeado pelos três recordings golden commitados.

| Alvo | Propriedade | O que verifica |
|------|-------------|----------------|
| `FuzzDeterminism` | P1 | Dois drivers independentes, mesmo script -> checksum identico a cada tick |
| `FuzzReplayFidelity` | P2 | Gravar -> `VerifyRecording` -> checksum do replay == run viva |
| `FuzzRewindConvergence` | P3 | Avancar ate N, rebobinar para K<N, avancar de novo ate N -> mesmo checksum |
| `FuzzNumericSanity` | P4 | Nenhum NaN/Inf em qualquer componente de qualquer entidade viva apos qualquer tick |
| `FuzzRecordingCodec` | P5 | `Recording` -> bytes -> `Recording` faz round-trip estavel |
| `FuzzNoPanic` | P6 | Input arbitrario (possivelmente invalido) nunca causa panic no engine |
| `FuzzResetReproducibility` | P7 | `ControlReset` no meio da run reconstrui o mundo de forma deterministica; checksums por tick apos o reset sao identicos aos de uma run nova |
| `FuzzScrubBidirectional` | P8 | Saltos de scrub repetidos (N -> K1 -> K2 -> ... -> N) cada um cai no checksum original, exercitando o thinning do archive de keyframes |
| `FuzzSpeedIndependence` | P9 | `ControlSetSpeed` muda o ritmo de ticks, nunca o estado por tick; historicos de speed diferentes dao checksums identicos |
| `FuzzDespawnMidRun` | P10 | Um despawn disparado por input acontece exatamente uma vez, permanece finito/determinístico e converge após rewind |
| `FuzzSubstepTunneling` | P11 | Um cenário moldado de projétil rápido/parede fina exercita CCD com substeps, permanece finito/determinístico e nunca atravessa a parede |
| `FuzzForkAfterScrubDeterminism` | P12 | Scrub + input novo cria um fork que coincide com uma run limpa do novo futuro e difere do futuro abandonado |

P4 e o destaque: floats de entrada são forçadamente finitos pelo decodificador,
então qualquer NaN ou Inf que ele encontra foi produzido pela propria simulação
(p.ex. `Body.InvMass` ou `Ballistics.DragK` extremos, mas finitos), não injetado.

Um registro de evento em cada três decodifica para um `EventAxisChanged` num
eixo de gamepad ligado, em vez de uma borda de botão. Esse é o único caminho em
que o `Mapper` consome `Event.Value`, então também é o único jeito de os bytes
de valor de um registro afetarem a simulação. `TestAxisEventDrivesSimulation`
prova que o caminho está vivo de ponta a ponta.

### Como rodam

1. **Como teste de regressão, de graça**, em todo `go test ./...` -- o framework
   de testes do Go executa o ‘corpus’ semente de toda funcao `FuzzXxx` mais os
   arquivos commitados em `testdata/fuzz/<Target>/`. Custo: milissegundos.

2. **Como fuzzer de verdade**, sob demanda com `-fuzz`:

   ```powershell
   go test ./internal/simfuzz -fuzz FuzzDeterminism -fuzztime 300s
   go test ./internal/simfuzz -fuzz FuzzRewindConvergence -fuzztime 120s
   # etc.
   ```

3. **No CI**, pelo workflow `.github/workflows/fuzz.yml`, diariamente e sob
   demanda (`workflow_dispatch`) -- **não** em pull request. Fuzzing com
   orçamento fixo não é um bom portão de PR: ou trava merges num timeout
   instável, ou é marcado como não bloqueante e aí ninguém olha. O sinal de PR
   já existe e é de graça: o corpus semente roda em `go test ./...`.

### Fluxo de triagem

1. Um alvo de fuzz falha -> o helper de checagem chama `t.Fatalf`.
2. `dumpRepro` grava uma entrada válida de corpus do Go (`.txt`) e um replay
   `.arrp` em `testdata/repro/<Target>/`; a mensagem de falha informa ambos.
   O próprio fuzz runner também pode persistir o input minimizado diretamente
   em `testdata/fuzz/<Target>/`.
3. Mova o `.txt` para o diretório de corpus do alvo
   (`testdata/fuzz/<Target>/`) e commite-o com o replay correspondente em
   `testdata/replays/<Target>/` **antes** de qualquer fix.
4. Corrija o bug, depois verifique que o arquivo de repro passa.
5. Commite o repro e o fix em commits separados para a regressão ser revisavel
   de forma independente.

`TestReproRoundTrip` prova que um artefato do `dumpRepro` recarrega e
redecodifica para o mesmo cenário.

### Adicionando um novo alvo

1. Defina o helper de checagem em um arquivo `*_test.go` específico do
   comportamento.
2. Adicione um wrapper `Fuzz*` em `fuzz_properties_test.go`.
3. Semeie-o com `addSimulationSeeds`, `addRecordingSeeds` ou um seed específico
   deliberadamente moldado.
4. Confirme a descoberta com `go test ./internal/simfuzz -list '^Fuzz'`.

Novos alvos nunca devem causar panic; o decodificador e total.

## Comandos de verificacao

Antes de finalizar mudanças de teste:

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
