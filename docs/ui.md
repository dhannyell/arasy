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

Widgets recebem retangulos e posicoes em uma area de referencia da UI:

```go
reset := math2d.NewRect(
    math2d.Vec2{X: ctx.Camera().Viewport.Width() - 100, Y: 8},
    math2d.Vec2{X: 92, Y: 28},
)
```

O segundo argumento de `math2d.NewRect` e tamanho, nao canto maximo.

Por padrao, essa area de referencia e `Config.Camera.Viewport`. Isso permite
desenhar a UI em uma resolucao de design fixa e deixar o `runtime` escalar e
centralizar os widgets quando o viewport real muda.

Internamente, o pacote converte os valores da UI para world-space usando a
camera do contexto:

```text
UI rect -> responsive UI camera -> camera.ScreenToWorld -> render.DrawRect
UI text size -> responsive UI camera -> camera.ScreenScalarToWorld -> render.DrawText
```

Essa escolha permite que o pipeline de render atual continue unico: tanto jogo
quanto UI produzem comandos backend-agnostic.

## UI responsiva com `ui.Space`

O arquivo `ui/space.go` define o parametro `ui.Space`, usado pelas variantes
`*In(...)` dos widgets e por `ClickedIn(...)`.

```go
ctx.UI.ButtonIn("btn_reset", "Reset", reset, ui.SpacePlayfield)
if ctx.UI.ClickedIn("btn_reset", reset, ui.SpacePlayfield) {
    g.reset()
}
```

Espacos disponiveis:

- `ui.SpaceRef`: usa a resolucao de referencia da UI (`Config.Camera.Viewport`)
  e aplica `AspectFit` dentro do viewport fisico. E o padrao usado por
  `Button`, `Panel`, `Label`, `ProgressBar` e `Clicked`. Use para menus, HUDs
  genericos e paineis que devem escalar de forma responsiva.
- `ui.SpacePlayfield`: ancora a UI dentro do retangulo onde o `World` realmente
  foi desenhado depois da estrategia de aspecto da camera. Use para elementos
  que nao podem sair das bordas do jogo, como score bar, minimapa ou indicadores
  presos ao campo.
- `ui.SpaceScreen`: usa pixels fisicos do viewport, sem escala e sem letterbox.
  Use para overlays pixel-exatos, como debug HUDs ou icones presos ao canto da
  janela.

Os metodos sem sufixo sao atalhos para `SpaceRef`:

```go
ctx.UI.Button("btn_play", "Play", play)
// equivalente a:
ctx.UI.ButtonIn("btn_play", "Play", play, ui.SpaceRef)
```

Para a interacao funcionar corretamente, use o mesmo `Space` no `Update` e no
`Render` para o mesmo widget.

## Update vs Render

Interacao que muda simulacao deve acontecer em `Update`.

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    _, reset := hudLayout(ctx.Camera().Viewport)
    if ctx.UI.Clicked("btn_reset", reset) {
        g.resetMatch(ctx.World)
    }
}
```

Desenho deve acontecer em `Render`.

```go
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

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
- `ButtonIn(id, label, rect, space)`
- `Label(text, pos)`
- `LabelIn(text, pos, space)`
- `Panel(rect)`
- `PanelIn(rect, space)`
- `ProgressBar(rect, value)`
- `ProgressBarIn(rect, value, space)`
- `Clicked(id, rect)`
- `ClickedIn(id, rect, space)`

`Button`, `Panel` e `ProgressBar` desenham retangulos. `Label` e o texto de
`Button` usam `render.DrawText` com alinhamento central e baseline middle.

`ProgressBar` faz clamp de `value` para `[0, 1]`.

## Animacao de UI

Animacao de UI e cosmetica: roda em `Render`, usa o clock de
`Driver.Frame(elapsed)` e nao entra nos snapshots de rewind. Pause, speed e
rewind da simulacao nao rebobinam esses efeitos; apenas o gatilho vindo do
estado deterministico do jogo pode reinicia-los.

A API tem tres camadas:

- `ctx.UI.FX()` — receitas prontas para casos comuns.
- `ctx.UI.Effect(id, opts)` — timeline componivel, boa para HUDs e eventos
  customizados.
- primitivas (`Approach`, `Spring`, `Tween`, `Clock`, `Reset`,
  `SequenceKeyed`, `Track`) — escape hatch para ajuste fino.

### `FX`: receitas prontas

Use `FX` quando o efeito ja existe como receita e voce quer pouca configuracao.
`Toast` desenha painel + texto em coordenadas de UI:

```go
ctx.UI.FX().Toast("save_notice", "Saved", rect, ui.ToastOptions{Key: saveKey})
```

`ToastIn` aceita um `ui.Space` explicito:

```go
ctx.UI.FX().ToastIn("pause", "Paused", rect, ui.SpaceScreen, ui.ToastOptions{Key: pauseKey})
```

`ToastOptions.Key` segue a regra de `SequenceKeyed`: `0` significa inativo e
limpa o gatilho retido; um novo valor nao-zero reinicia a animacao. Use um tick
ou contador monotonicamente crescente do estado do jogo para disparar o efeito.

`ToastOptions` tambem aceita ajustes de duracao, layer e aparencia por
`ToastStyle`. A receita de movimento e fixa por design; se precisar alterar
entrada, fade ou pulso, prefira `Effect`.

### `Effect`: composicao sem boilerplate

Use `Effect` quando a animacao e especifica do jogo, mas ainda cabe em uma
timeline simples. Ele encapsula `SequenceKeyed` e nomeia transformacoes comuns:

```go
fx := ctx.UI.Effect("goal_banner", ui.EffectOptions{Key: lastGoalTick, Duration: 2})
if !fx.Done() {
    rect := fx.SlideY(finalRect, -60, 12, 0, 0.35, ease.OutBack)
    alpha := fx.FadeOut(1.6, 2, ease.InQuad)
    pulse := fx.Pulse(0.35, 1.6, 0.05, 8)

    // Monte comandos do jogo ou da UI usando rect/alpha/pulse.
    _ = rect
    _ = alpha
    _ = pulse
}
```

`Effect` e a melhor escolha para efeitos em world-space: calcule valores com
`Effect` e emita `render.Command` no `Render` do jogo, usando a camera do jogo.

### Primitivas: ajuste fino

Use as primitivas quando precisar controlar o estado numerico diretamente:

- `Approach`: valor segue alvo de forma exponencial, independente de FPS.
- `Spring`: mola amortecida para movimento com inercia.
- `Tween`/`Clock`/`Reset`: progresso por tempo.
- `SequenceKeyed`/`Track`: sequencias multi-etapa.

Widgets internos tambem usam essas primitivas. Por exemplo, `Button` anima
hover/press via `Approach` e `ButtonTheme.HoverSpeed`/`PressSpeed`.

### Direcao de evolucao

Novas receitas devem evitar crescer `ToastOptions` indefinidamente. Quando uma
familia de efeitos amadurecer, prefira criar um novo metodo em `FX` com options
minimas em vez de adicionar mais flags soltas:

```go
ctx.UI.FX().Toast("goal", "GOL!", rect, ui.ToastOptions{
    Key: goalTick,
    Style: ui.ToastStyle{TextSize: 24},
})
```

O papel de `FX` e ser rapido e opinativo. O papel de `Effect` e ser componivel,
incluindo movimento customizado. O papel das primitivas e permitir ajuste fino
sem forcar novas receitas.

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

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    _, reset := hudLayout(ctx.Camera().Viewport)
    if ctx.UI.Clicked("btn_reset", reset) {
        g.reset()
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

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
