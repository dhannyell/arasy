# UI

Este guia explica o pacote `ui` da FrameForge.

O pacote implementa uma UI imediata pequena, pensada para HUDs, menus simples,
debug overlays e ferramentas leves dentro da engine.

## Ideia principal

```text
Update:
  pointer frame -> ctx.UI.Clicked(...) -> muda estado deterministico

Render:
  world + cursor atual -> ctx.UI.Button(...) -> render.Command
```

A UI nao fala com Canvas, DOM, janela nativa ou mouse do sistema diretamente.
Ela recebe dados ja traduzidos pelo `runtime` e produz comandos do pacote
`render`.

## Onde ele fica na arquitetura

Regra de dependencia:

```text
cmd/* -> platform/* -> runtime -> ui -> { render, math2d, input }
```

Isso significa:

- jogos usam UI via `ctx.UI`;
- plataformas nao desenham UI diretamente;
- `ui` nao importa `runtime`;
- `ui` nao conhece nenhuma plataforma;
- widgets geram `render.Command`, como o restante da engine.

## Screen-space

Widgets recebem retangulos e posicoes em pixels de viewport:

```go
reset := math2d.NewRect(
    math2d.Vec2{X: ctx.Camera.Viewport.Width() - 100, Y: 8},
    math2d.Vec2{X: 92, Y: 28},
)
```

O segundo argumento de `math2d.NewRect` e tamanho, nao canto maximo.

Internamente, o pacote converte esses valores para world-space usando a camera
do contexto:

```text
screen rect -> camera.ScreenToWorld -> render.DrawRect
screen text size -> camera.ScreenScalarToWorld -> render.DrawText
```

Essa escolha permite que o pipeline de render atual continue unico: tanto jogo
quanto UI produzem comandos backend-agnostic.

## Update vs Render

Interacao que muda simulacao deve acontecer em `Update`.

```go
func (g *Game) Update(ctx *runtime.Context, dt float64) {
    _, reset := hudLayout(ctx.Camera.Viewport)
    if ctx.UI.Clicked("btn_reset", reset) {
        g.resetMatch(ctx.World)
    }
}
```

Desenho deve acontecer em `Render`.

```go
func (g *Game) Render(ctx *runtime.Context) []render.Command {
    score, reset := hudLayout(ctx.Camera.Viewport)

    ctx.UI.Panel(score)
    ctx.UI.Label("0 - 0", score.Center())
    ctx.UI.Button("btn_reset", "Reset", reset)

    return ctx.World.RenderCommands()
}
```

Essa separacao e deliberada:

- `Update` consome input carimbado por tick;
- `Render` pode usar hover/cursor como estado visual transitorio;
- gameplay continua deterministico;
- rewind consegue restaurar `activeID` e estado de pointer.

## Pointer

O runtime entrega um `input.PointerFrame` para a UI no Update.

O frame contem:

- posicao final do pointer no tick;
- se o cursor esta dentro da superficie;
- estado final dos botoes;
- transicoes de botao com posicao.

As transicoes preservam casos importantes:

```text
down fora  -> up dentro  = sem clique
down dentro -> up fora   = sem clique
down dentro -> up dentro = clique
leave com botao apertado = release sintetico
```

O estado persistente de UI, como `activeID`, vive no `runtime.Driver` e entra no
snapshot do recorder.

## Widgets

Widgets disponiveis:

- `Button(id, label, rect)`
- `Label(text, pos)`
- `Panel(rect)`
- `ProgressBar(rect, value)`

`Button`, `Panel` e `ProgressBar` desenham retangulos. `Label` e o texto de
`Button` usam `render.DrawText` com alinhamento central e baseline middle.

`ProgressBar` faz clamp de `value` para `[0, 1]`.

## IDs

Widgets interativos usam IDs estaveis:

```go
ctx.UI.Button("btn_reset", "Reset", reset)
if ctx.UI.Clicked("btn_reset", reset) {
    // ...
}
```

O ID precisa ser o mesmo no Update e no Render. IDs duplicados compartilham
estado de interacao, entao devem ser evitados.

## Layers

`ui.LayerBase` reserva a faixa de layers da UI:

```go
const LayerBase = 10_000
```

Widgets incrementam layers a partir desse valor para aparecerem acima dos
comandos comuns do jogo.

## Tema

O `runtime.Config` pode receber um `UITheme`.

Se o tema for zero value, o driver usa `ui.DefaultTheme()`.

```go
func (g *Game) Setup() runtime.Config {
    return runtime.Config{
        UITheme: ui.DefaultTheme(),
    }
}
```

O tema define estilos para:

- botoes normais, hovered e active;
- labels;
- panels;
- progress bars.

## Plataforma web/canvas

No canvas web, o browser envia eventos de mouse em backing pixels:

```text
mousemove  -> EventPointerMove
mousedown  -> EventButtonDown
mouseup    -> EventButtonUp
mouseleave -> EventPointerLeave
```

O servidor traduz esses eventos para `input.Event`, o `runtime.Driver` acumula
um `PointerFrame`, e o jogo consome `ctx.UI` sem conhecer WebSocket ou DOM.

## Exemplo completo

```go
func hudLayout(vp math2d.Rect) (score, reset math2d.Rect) {
    score = math2d.NewRect(
        math2d.Vec2{X: vp.Width()/2 - 60, Y: 8},
        math2d.Vec2{X: 120, Y: 28},
    )
    reset = math2d.NewRect(
        math2d.Vec2{X: vp.Width() - 100, Y: 8},
        math2d.Vec2{X: 92, Y: 28},
    )
    return
}

func (g *Game) Update(ctx *runtime.Context, dt float64) {
    _, reset := hudLayout(ctx.Camera.Viewport)
    if ctx.UI.Clicked("btn_reset", reset) {
        g.reset()
    }
}

func (g *Game) Render(ctx *runtime.Context) []render.Command {
    score, reset := hudLayout(ctx.Camera.Viewport)

    ctx.UI.Panel(score)
    ctx.UI.Label("0 - 0", score.Center())
    ctx.UI.Button("btn_reset", "Reset", reset)

    return ctx.World.RenderCommands()
}
```

## O que nao pertence ao pacote ui

Nao colocar em `ui`:

- regras de jogo;
- estado de partida;
- WebSocket;
- DOM;
- Canvas;
- renderizacao de pixels;
- layout engine complexo sem necessidade real.

Estado especifico do jogo deve ficar no jogo. Se ele precisa sobreviver a
rewind, deve ser capturado pelo runtime como estado de jogo em uma camada
propria, nao como componente especifico dentro de `ui`.

## Resumo

```text
ui.UI:
  interface imediata usada pelo jogo

Clicked:
  query deterministica chamada no Update

Button/Label/Panel/ProgressBar:
  widgets chamados no Render

Theme:
  estilo visual dos widgets

LayerBase:
  faixa reservada para UI acima do jogo
```

O pacote existe para permitir UI simples sem acoplar jogos a plataformas e sem
quebrar o modelo deterministico da simulacao.
