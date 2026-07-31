# UI

Este guia explica o pacote `ui` da Arasy.

O pacote implementa uma UI imediata pequena, pensada para HUDs, menus simples,
debug overlays e ferramentas leves dentro da engine.

## Ideia principal

```text
Update:
  pointer frame -> ctx.UI.In(space).Clicked(...) -> muda estado deterministico

Render:
  world + cursor atual -> ctx.UI.In(space).Button(...) -> render.Command
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

O arquivo `ui/space.go` define o parametro `ui.Space`. `ctx.UI.In(space)`
devolve um scope leve com cada widget exposto uma unica vez — o espaco e dito
uma vez por bloco, e o scope retornado nao guarda estado proprio alem de
`(ui, space)`, entao nao ha nada para restaurar depois.

```go
d := ctx.UI.In(ui.SpacePlayfield)
d.Button("btn_reset", "Reset", reset)

in := ctx.UI.In(ui.SpacePlayfield)
if in.Clicked("btn_reset", reset) {
    g.reset()
}
```

Espacos disponiveis:

- `ui.SpaceRef`: usa a resolucao de referencia da UI (`Config.Camera.Viewport`)
  e aplica `AspectFit` dentro do viewport fisico. Use para menus, HUDs
  genericos e paineis que devem escalar de forma responsiva.
- `ui.SpacePlayfield`: ancora a UI dentro do retangulo onde o `World` realmente
  foi desenhado depois da estrategia de aspecto da camera. Use para elementos
  que nao podem sair das bordas do jogo, como score bar, minimapa ou indicadores
  presos ao campo.
- `ui.SpaceScreen`: usa pixels fisicos do viewport, sem escala e sem letterbox.
  Use para overlays pixel-exatos, como debug HUDs ou icones presos ao canto da
  janela.

Para a interacao funcionar corretamente, use o mesmo `Space` no `Update` e no
`Render` para o mesmo widget.

## Update vs Render

Interacao que muda simulacao deve acontecer em `Update`.

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    _, reset := hudLayout(ctx.Camera().Viewport)
    if ctx.UI.In(ui.SpaceRef).Clicked("btn_reset", reset) {
        g.resetMatch(ctx.World)
    }
}
```

Desenho deve acontecer em `Render`.

```go
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

    d := ctx.UI.In(ui.SpaceRef)
    d.Panel(score)
    d.Label("0 - 0", score.Center())
    d.Button("btn_reset", "Reset", reset)

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
- wheel acumulado no tick, em unidades de linha;
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

`ctx.UI` expõe apenas `In(space)` mais um punhado de metodos sem espaco
(`RowHeight`, `HeaderHeight`, `TableBodyRect`, `SetFont`, animacao e grupos).
Tudo que depende de um retangulo/posicao mora no scope devolvido por `In`.

`DrawScope` (Render, via `ctx.UI.In(space)`):

- `Button(id, label, rect)`
- `Card(rect, accent)`
- `Badge(text, center, bg)`
- `Label(text, pos)`
- `Panel(rect)`
- `ProgressBar(rect, value)`
- `Slider(id, rect, value, opts)`
- `TextInput(id, rect, text, opts)`
- `Checkbox(id, rect, checked, label)`
- `Dropdown(id, rect, labels, selected)`
- `Tabs(id, rect, labels)` / `TabsAt(headers, labels, active)`
- `Table(id, rect, cols, rowCount, selected, cell)`
- `TableStyled(id, rect, cols, rowCount, selected, cell, style)`
- `TableSorted(id, rect, cols, rowCount, selected, sort, cell)`
- `List(id, rect, rowH, rowCount, selected, draw)`
- `ScrollArea(id, view, contentH)`
- `Hovered(rect)`
- `FillRect(rect, fill)`
- `DrawText(text, pos, color, size, align, baseline)`
- `DrawTextRot(text, pos, color, size, rotation, align, baseline)`
- `DrawCircle(center, radius, fill, stroke, strokeWidth)`
- `DrawLine(from, to, width, color)`
- `DrawPoly(fill, stroke, strokeWidth, points...)`
- `FX()`

`InteractScope` (Update, via `ctx.UI.In(space)`):

- `Clicked(id, rect)`
- `Activated(id, rect)`
- `Focusable(id, rect)`
- `Hovered(rect)`
- `ScrollOffset(id, view, contentH)`
- `SliderInput(id, rect, value, opts)`
- `TextInput(id, rect, text, opts)`
- `CheckboxInput(id, rect, checked)`
- `DropdownInput(id, rect, count, selected)`
- `TabsInputValue(id, headers, current)` / `TabsInput(id, rect, count)`
- `TableHit(id, rect, cols, rowCount, selected)`
- `TableHeaderHit(id, rect, cols)`
- `TableInputValue(id, rect, cols, rowCount, selected)`
- `ListHit(id, rect, rowH, rowCount)`
- `ListInputValue(id, rect, rowH, rowCount, selected)`

`Button`, `Panel` e `ProgressBar` desenham retangulos. `Label` e o texto de
`Button` usam `render.DrawText` com alinhamento central e baseline middle.

`ProgressBar` faz clamp de `value` para `[0, 1]`.

`Slider` segue a separacao deterministica da UI em duas fases. O jogo consome
o pointer no `Update` com `SliderInput` e desenha o mesmo ID, retangulo e
opcoes no `Render` com `Slider`:

```go
var volumeSlider = ui.SliderOptions{Min: 0, Max: 1, Step: 0.05}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    if value, changed := ctx.UI.In(ui.SpaceRef).SliderInput("volume", volumeRect, g.Volume, volumeSlider); changed {
        g.Volume = value
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    ctx.UI.In(ui.SpaceRef).Slider("volume", volumeRect, g.Volume, volumeSlider)
    return nil
}
```

O valor pertence ao estado do jogo; a UI persiste somente captura e foco.
`Step <= 0` deixa o valor continuo. Um `Step` positivo quantiza incrementos a
partir de `Min`, mantendo `Max` como ponto final alcancavel. Clicar no trilho
reposiciona o thumb; arrastar continua
capturado fora do retangulo e faz clamp em `[Min, Max]`.

`Card` desenha um painel arredondado com uma barra de accent opcional. `Badge`
desenha um chip pequeno dimensionado por `render.MeasureText`.

## Texto digitado

Digitacao entra pelo vocabulario `input`, como qualquer outra entrada: o host
emite `input.EventText` com o codepoint resolvido pela plataforma (o browser
usa `KeyboardEvent.key`, o host nativo o char callback do GLFW) e o evento e
gravado no ARRP junto com os demais. Uma sessao digitada e replayavel e
scrubbavel de graca — nao existe caminho de texto fora do journal.

`TextInput` e caller-owned como o `Slider`: o jogo passa o texto que tem e
guarda o que volta.

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    result := ctx.UI.In(ui.SpaceRef).TextInput("search", searchRect, g.Query, ui.TextInputOptions{
        Placeholder: "BUSCAR",
        MaxRunes:    24,
    })
    if result.Changed {
        g.Query = result.Text
    }
    if result.Submitted { // Enter com o campo focado
        g.RunSearch()
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    ctx.UI.In(ui.SpaceRef).TextInput("search", searchRect, g.Query, ui.TextInputOptions{Placeholder: "BUSCAR"})
    return nil
}
```

O texto e estado do jogo (entra no fingerprint de replay e rewina com a
simulacao). O que o widget guarda em `ui.State` e a posicao do caret e o
relogio de auto-repeat — clonados no snapshot como o resto.

Edicao: caracteres inserem no caret; Backspace/Delete apagam; setas, Home e End
movem; Enter submete; Esc desfoca; clique posiciona o caret e clique fora tira
o foco. O auto-repeat de teclas de edicao e contado em ticks pela engine
(`~30` ticks de espera, depois um a cada `3`), nao pela taxa de repeat do SO —
por isso um Backspace segurado apaga o mesmo tanto ao vivo, no replay e depois
de um rewind. O caret pisca pelo relogio de render, nunca por estado de
simulacao.

Fora da v1 (follow-ups registrados): IME/composicao, selecao de texto,
clipboard, teclado virtual mobile e modificadores como eventos.

## Checkbox, dropdown e ordenacao

`CheckboxInput` recebe e devolve o bool, no mesmo padrao caller-owned.

`DropdownInput` devolve o indice selecionado; se o menu esta aberto agora e
estado do widget, guardado em `ui.State`. A lista aberta desenha no bloco de
layers reservado `LayerOverlay`, acima de qualquer widget declarado depois
dela, e reivindica aquela area em `ui.State` — assim a linha coberta pela
lista nao responde ao clique que escolheu um item. A lista em si e a `List`
virtualizada, entao mil opcoes custam o que oito custam.

Ordenacao de tabela e do jogo: `TableHeaderHit` devolve a coluna clicada (ou
-1) e `TableSorted` desenha a seta da `ui.TableSort` que o jogo passou. O
widget nao ordena nada — as linhas sao dados do jogo, e so ele sabe como uma
coluna compara.

```go
if col := in.TableHeaderHit("market", tableRect, cols); col >= 0 {
    g.Sort = toggleSort(g.Sort, col) // decisao do jogo
}
```

## Scroll, tabelas e foco

Scroll e tabelas seguem o padrao imediato em duas fases:

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    in := ctx.UI.In(ui.SpaceRef)
    if row := in.TableHit("players", tableRect, cols, len(players), selected); row >= 0 {
        selected = row
    }
    in.ScrollOffset("players", bodyRect, float64(len(players))*32)
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    ctx.UI.In(ui.SpaceRef).Table("players", tableRect, cols, len(players), selected, func(row, col int) string {
        return players[row].Cell(col)
    })
    return nil
}
```

`ScrollOffset` deve ser chamado no `Update`, porque consome
`PointerFrame.Wheel` e grava o offset em `ui.State`. `ScrollArea` e `Table`
desenham a scrollbar e aplicam `render.BaseCommand.Clip` aos comandos do corpo.
O offset vive no snapshot do runtime, entao rewind/scrub restauram a posicao da
lista.

`TableColumn.Width == 0` significa coluna flex: o espaco restante e dividido
entre as colunas flex. Use `AlignRight` para numeros; a largura de texto vem da
metrica canonica `render.MeasureText`.

Foco de teclado e minimo: `Focusable` registra widgets na ordem de chamada,
Tab avanca, Enter ativa o focado e Esc limpa o foco. `Activated` e o superset
recomendado de clique + Enter; `Clicked` continua disponivel quando o widget
deve responder somente ao pointer.

A ordem de foco e a ordem de declaracao — a UI e reconstruida todo frame,
entao a ordem que o jogo escreve e a ordem que o jogador tabula, sem nada para
manter em sincronia. **Regra de conflito**: a engine so consome Tab quando o
frame declarou pelo menos um widget focavel; sem UI focavel, a tecla continua
livre para bindings do jogo. Duas teclas ficam com o jogo de proposito:
Espaco nao ativa widget (e pulo/tiro/pause em jogo demais) e Shift+Tab ainda
nao volta o foco, porque modificadores nao fazem parte do vocabulario `input`.

`ListInputValue` e `TableInputValue` sao as variantes caller-owned da selecao
de linha: clique seleciona, e com a lista focada as setas, Home, End e
PageUp/PageDown tambem — os dois caminhos escrevem o mesmo indice, que e do
jogo. A linha selecionada pelo teclado e trazida para dentro da janela visivel
automaticamente.

## Animacao de UI

Animacao de UI e cosmetica: roda em `Render`, usa o clock de
`Driver.Frame(elapsed)` e nao entra nos snapshots de rewind. Pause, speed e
rewind da simulacao nao rebobinam esses efeitos; apenas o gatilho vindo do
estado deterministico do jogo pode reinicia-los.

A API principal e organizada em camadas:

- `ctx.UI.In(space).FX()` — receitas prontas para casos comuns.
- `ctx.UI.Effect(id, opts)` — timeline componivel, boa para HUDs e eventos
  customizados.
- primitivas (`Approach`, `Spring`, `Tween`, `Clock`, `Reset`,
  `SequenceKeyed`, `Track`) — escape hatch para ajuste fino.

`ScreenTransition` complementa essas camadas para trocas de tela inteiras: ele
usa `BeginGroup`/`EndGroup` por baixo para mover ou aplicar fade a todos os
comandos emitidos por uma tela.

### `FX`: receitas prontas

Use `FX` quando o efeito ja existe como receita e voce quer pouca configuracao.
`FX` e obtido de um `DrawScope`, entao ja carrega o espaco de coordenadas.
`Toast` desenha painel + texto nesse espaco:

```go
ctx.UI.In(ui.SpaceRef).FX().Toast("save_notice", "Saved", rect, ui.ToastOptions{Key: saveKey})

ctx.UI.In(ui.SpaceScreen).FX().Toast("pause", "Paused", rect, ui.ToastOptions{Key: pauseKey})
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

### Transicoes de tela

Use `ScreenTransition` quando o estado deterministico do jogo escolhe qual tela
de UI esta ativa e voce quer animar a troca visualmente. A chave (`key`) deve
identificar a tela atual: `0` limpa o estado retido; a primeira chave nao-zero
aparece sem animar; uma mudanca de chave inicia a transicao.

```go
tr := ctx.UI.ScreenTransition("main_screen", screenKey, ui.TransitionOptions{
    Kind:     ui.TransitionSlideLeft,
    Duration: 0.32,
    Ease:     ease.OutCubic,
})

if outgoing, ok := tr.Outgoing(); ok {
    ctx.UI.BeginGroup(tr.OutgoingGroup())
    drawScreen(ctx.UI, outgoing)
    ctx.UI.EndGroup()
}

ctx.UI.BeginGroup(tr.IncomingGroup())
drawScreen(ctx.UI, tr.Incoming())
ctx.UI.EndGroup()
```

As receitas prontas sao `TransitionFade`, `TransitionSlideLeft` e
`TransitionSlideRight`. Elas sao render-only: usam o `AnimStore` e o clock de
render, nao entram no checksum e nao alteram a simulacao.

`BeginGroup`/`EndGroup` tambem podem ser usados diretamente. Tudo que for
desenhado entre os dois recebe um `GroupOptions` com `Offset`, `Fade` e
`Space`. O valor zero e no-op: sem deslocamento e totalmente opaco.

```go
ctx.UI.BeginGroup(ui.GroupOptions{
    Offset: math2d.Vec2{X: 24},
    Fade:   0.25,
    Space:  ui.SpaceRef,
})
d := ctx.UI.In(ui.SpaceRef)
d.Panel(panel)
d.Label("Settings", panel.Center())
ctx.UI.EndGroup()
```

Input nao e suprimido automaticamente durante a transicao. `Clicked` ainda roda
em `Update`, enquanto a transicao roda em `Render`; se uma tela precisar
bloquear cliques por alguns ticks apos navegar, guarde esse bloqueio no estado
do jogo.

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
ctx.UI.In(ui.SpaceRef).FX().Toast("goal", "GOL!", rect, ui.ToastOptions{
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
ctx.UI.In(ui.SpaceRef).Button("btn_reset", "Reset", reset)
if ctx.UI.In(ui.SpaceRef).Clicked("btn_reset", reset) {
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
- progress bars;
- sliders;
- cards, rows, headers, badges, texto e foco.

Temas parciais sao completados por `ui.WithDefaults`: slots zero value herdam
os valores de `DefaultTheme()`. Isso permite que jogos antigos personalizem
apenas `Button` ou `Panel` sem precisar preencher os slots novos usados por
scroll, tabela e foco.

## Plataforma wasm

No canvas web, o browser envia eventos de ponteiro em backing pixels:

```text
pointermove  -> EventPointerMove
pointerdown  -> EventButtonDown
pointerup    -> EventButtonUp
pointerleave/pointercancel -> EventPointerLeave

O host WASM usa Pointer Events para mouse, toque e caneta. O canvas recebe
`touch-action: none`; multitouch continua fora do modelo de ponteiro unico.
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
    if ctx.UI.In(ui.SpaceRef).Clicked("btn_reset", reset) {
        g.reset()
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

    d := ctx.UI.In(ui.SpaceRef)
    d.Panel(score)
    d.Label("0 - 0", score.Center())
    d.Button("btn_reset", "Reset", reset)

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
ctx.UI.In(space):
  scope imediato usado pelo jogo, um por espaco de coordenadas

Clicked:
  query deterministica chamada no Update

Button/Label/Panel/ProgressBar/Slider:
  widgets chamados no Render

Theme:
  estilo visual dos widgets

LayerBase:
  faixa reservada para UI acima do jogo
```

O pacote existe para permitir UI simples sem acoplar jogos a plataformas e sem
quebrar o modelo deterministico da simulacao.
