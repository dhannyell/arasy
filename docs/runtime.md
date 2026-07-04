# Runtime

Este guia explica o papel do pacote `runtime` na FrameForge.

O `runtime` e a camada que liga um jogo a uma plataforma sem fazer o jogo conhecer Canvas, HTTP, PNG, janela nativa ou qualquer outro detalhe de borda.

## Ideia principal

```text
game code  ->  runtime  ->  scene/render/input
platform   ->  runtime
```

O jogo implementa uma interface pequena.

A plataforma cria um `runtime.Driver` e chama metodos explicitos:

```text
ApplyInput  ->  Advance  ->  Frame
Control     ->  Advance
```

O `runtime` coordena ciclo de vida, input, controles de desenvolvimento, historico opcional e producao de frames.

Ele nao desenha pixels e nao abre sockets.

## Onde ele fica na arquitetura

Regra de dependencia:

```text
cmd/* -> platform/* -> runtime -> { scene, render, input } -> math2d
```

Isso significa:

- `runtime` pode importar `scene`, `render` e `input`.
- `platform/web/canvas` pode importar `runtime`.
- `platform/raster` pode importar `runtime`.
- `runtime` nao importa nenhuma plataforma.
- `scene`, `render`, `math2d` e `input` nao importam `runtime`.

Essa separacao permite que o mesmo jogo rode em mais de uma saida:

```text
cmd/demos/football/webCanvas  -> platform/web/canvas -> runtime
cmd/demos/football/raster     -> platform/raster     -> runtime
futuro native                 -> platform/native     -> runtime
futuro wasm/webgpu            -> platform/...        -> runtime
```

## O contrato Game

Um jogo implementa `runtime.Game`:

```go
type Game interface {
    Setup() Config
    Init(ctx *UpdateContext)
    Update(ctx *UpdateContext, dt float64)
    Render(ctx *RenderContext) []render.Command
}
```

### Setup

`Setup` retorna configuracao portatil:

- camera base;
- cor de fundo;
- bounds da simulacao;
- configuracao de bounce;
- configuracao de fisica;
- frequencia de simulacao;
- FPS alvo;
- bindings de input;
- tamanho do historico de rewind.

Configuracao especifica de plataforma fica fora daqui.

Exemplos de configuracao que nao pertencem a `Setup`:

- endereco HTTP;
- titulo de janela;
- caminho do PNG;
- tamanho do arquivo de saida;
- opcoes de WebSocket.

Essas coisas pertencem ao `Options` de cada plataforma.

### Init

`Init` popula um `scene.World` novo.

Ele e chamado:

- no inicio;
- depois de reset.

O jogo recebe `ctx.World` diretamente. O `runtime` nao esconde o mundo atras de uma camada maior.

### Update

`Update` roda um tick fixo de simulacao.

Aqui o jogo normalmente:

1. Le `ctx.Input`.
2. Atualiza componentes no `ctx.World`.
3. Chama `ctx.HoldSimulation()` se este tick nao deve avancar fisica.

Depois do `Update`, o `runtime` chama `World.Step(dt, ctx.StepConfig)`
automaticamente. Jogos que precisam reagir a contatos no mesmo tick implementam
`runtime.PostStepper` e leem `ctx.World.Contacts()` em `PostStep`.

Essa decisao e importante: a semantica de tick fica centralizada no Driver,
enquanto o jogo ainda pode segurar a simulacao por tick quando necessario.

### Render

`Render` retorna comandos de desenho:

```go
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    return ctx.World.RenderCommands()
}
```

Esses comandos sao backend-agnostic.

O `runtime` nao sabe se eles vao virar:

- pixels em `image.RGBA`;
- JSON para Canvas;
- draw calls nativas;
- comandos de GPU no futuro.

## Contextos

O `runtime` separa o contexto de update do contexto de render.

```go
type UpdateContext struct {
    World      *scene.World
    Input      input.State
    Pointer    input.PointerFrame
    UI         ui.Interactor
    Log        *slog.Logger
    Tick       uint64
    StepConfig scene.StepConfig
}

type RenderContext struct {
    World *scene.World
    UI    ui.Drawer
    Debug scene.DebugLayers
    Time  float64
    Dt    float64
    Log   *slog.Logger
    Tick  uint64
}
```

Os dois contextos tambem expoem `Camera()`. O contexto de render permite ajustar
a camera do frame com `SetCamera(cam)`.

### World

`World` e o estado ECS simples da cena.

O jogo tem controle direto sobre ele.

### Input

`Input` e o estado semantico produzido pelo `input.Mapper`.

O jogo le acoes:

```go
ctx.Input.Value(input.ActionMoveX)
ctx.Input.Pressed(input.ActionJump)
```

Ele nao precisa saber se a acao veio de teclado, gamepad, mouse ou outra fonte futura.

### Log

`Log` e um logger ja escopado para o jogo.

O nucleo da engine continua sem depender de logger global.

### Tick

`Tick` e o numero do tick de simulacao atual.

Ele e util para debug, determinismo, replay e logs.

### Debug

`Debug` e uma mascara de camadas visuais.

A plataforma pode ligar/desligar overlays, mas quem decide o que desenhar e o jogo:

```go
if ctx.Debug.Has(scene.DebugColliders) {
    // adiciona comandos de debug
}
```

### Camera

`Camera()` comeca com a camera base do `Config`.

O jogo pode ajustar a camera em `Render`, por exemplo para shake, zoom ou pan visual:

```go
cam := ctx.Camera()
cam.World = shiftedWorld
ctx.SetCamera(cam)
```

O `Driver.Frame(elapsed)` devolve a camera resultante junto dos comandos.

### StepConfig

`StepConfig` junta configuracoes de simulacao vindas do `Config`, como bounds,
bounce e fisica.

O Driver passa isso para `World.Step` automaticamente depois de `Update`. O jogo
pode ajustar `ctx.StepConfig` durante `Update` para afetar apenas o step do tick
atual.

## Driver

`Driver` e a maquina de estado portatil da engine.

Ele guarda:

- instancia do jogo;
- configuracao portatil;
- mundo atual;
- mapper de input;
- buffer de input;
- controles de desenvolvimento;
- tick atual;
- logger;
- historico opcional para rewind.

Cada plataforma cria um `Driver`:

```go
driver := runtime.NewDriver(game, log)
cfg := driver.Config()
```

Depois dirige o loop.

## Fluxo de input

A plataforma traduz eventos nativos para `input.Event`.

Exemplo conceitual:

```text
browser keydown "KeyW"
  -> input.Event{Source: keyboard KeyW, Kind: ButtonDown}
  -> driver.ApplyInput(events)
  -> input.Mapper
  -> ctx.Input em Update
```

O jogo nao le evento bruto.

Ele le estado semantico:

```go
ctx.Input.Value(input.ActionMoveY)
```

Isso deixa o mesmo jogo reaproveitavel entre plataformas.

## Fluxo de simulacao

Um tick normal:

```text
platform drena input
platform chama driver.ApplyInput(events)
platform chama driver.Advance(dt)

driver:
  captura snapshot se HistoryLen > 0
  calcula input.State
  monta Context
  chama game.Update(ctx, dt)
  chama World.Step(dt, ctx.StepConfig), a menos que Update tenha segurado a simulacao
  chama game.PostStep(ctx, stats), se implementado
  incrementa Tick
  encerra frame de input
```

Se o jogo estiver pausado, `Advance` respeita os controles de desenvolvimento.

## Fluxo de render

Um frame de render:

```text
platform chama driver.Frame(elapsed)

driver:
  acumula renderTime += elapsed; renderDt = elapsed
  monta RenderContext (ctx.Time/Dt refletem render-time, nao sim-time)
  chama game.Render(ctx)
  retorna Frame{Camera, Commands}

platform:
  converte Commands para sua saida
```

`elapsed` e o tempo real decorrido desde o frame anterior, medido pelo host.
Com a simulacao pausada ou em speed diferente, `ctx.Time`/`ctx.Dt` continuam
avancando em render-rate — correto para animacoes de UI cosmеticas.
`Commands()` e um helper que chama `Frame(0)` sem avancar o relogio de render.

No canvas, a plataforma serializa comandos para JSON.

No raster, a plataforma desenha comandos em `image.RGBA`.

## Controles de desenvolvimento

O `runtime` tambem centraliza controles comuns:

- play;
- pause;
- step manual;
- reset;
- speed;
- voltar um snapshot;
- voltar para um tick;
- voltar alguns segundos;
- selecionar camadas de debug.

A plataforma so traduz a intencao do usuario para `ControlCommand`:

```go
driver.Control(runtime.ControlCommand{
    Kind: runtime.ControlPause,
})
```

Assim uma plataforma nativa futura pode reaproveitar os mesmos controles.

## Historico e rewind

Se `Config.HistoryLen > 0`, o `Driver` captura snapshots antes dos ticks.

Cada snapshot guarda:

- clone do `World`;
- clone do `input.Mapper`;
- ultimo `input.State`;
- tick.

Isso permite voltar no tempo sem a plataforma conhecer detalhes do ECS.

O historico e uma ferramenta de desenvolvimento/debug. Ele nao deve virar regra de jogo.

## Concorrencia

O `Driver` nao e thread-safe por conta propria.

Isso e deliberado.

A plataforma decide como serializar acesso.

No canvas atual, por exemplo:

- `Advance` roda com lock exclusivo;
- `Frame` roda com lock de leitura;
- input e comandos de controle entram por filas.

Manter o `Driver` sem locks internos deixa o pacote mais simples e testavel.

## O que nao pertence ao runtime

Nao colocar no `runtime`:

- `net/http`;
- WebSocket;
- DOM;
- Canvas;
- paths de PNG/GIF;
- APIs de janela nativa;
- detalhes de GPU;
- regras de futebol;
- renderizacao de pixels.

Essas coisas pertencem a:

- `platform/*`, quando sao detalhe de saida/entrada;
- `cmd/*`, quando sao configuracao de executavel;
- jogo/app, quando sao regra de produto.

## Resumo

```text
runtime.Game:
  contrato portatil do jogo

runtime.UpdateContext / runtime.RenderContext:
  dados entregues ao jogo por tick/frame

runtime.Driver:
  maquina de estado portatil que plataformas dirigem

ApplyInput:
  eventos brutos da plataforma -> input semantico

Advance:
  roda ticks de simulacao

Frame:
  produz camera + render commands

Control:
  play/pause/step/reset/rewind/debug
```

O `runtime` existe para que o jogo seja escrito uma vez e possa rodar em varias plataformas com o minimo de adaptacao.
