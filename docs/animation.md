# Animação

Este guia explica a pilha de animação de apresentação da Arasy:
`anim.Timeline`, `ui.Sequence`, grupos do `render.Builder`, e como eles se
relacionam com os ticks de simulação. Não cobre animação de jogabilidade
guiada por estado determinístico de simulação (máquinas de estado de sprite
ligadas a componentes de `scene`) — isso é responsabilidade do código do
jogo, não desta pilha.

## Quando usar `anim`, `ui.Sequence` ou uma timeline de apresentação

A Arasy mantém dois relógios estritamente separados:

- **Ticks de simulação** (`UpdateContext.Tick`) são determinísticos,
  replayable e permitem rewind. Estado de jogabilidade — posição, vida, se
  uma entidade está viva — nunca deve depender do relógio de parede, sob
  pena de dessincronizar replay/rewind.
- **Tempo decorrido de renderização** (`RenderContext.Time`/`Dt`) é
  cosmético. Continua avançando durante pausa, câmera lenta e scrubbing, e é
  o único relógio que `anim.Timeline` lê.

Use `anim.Timeline` (ou o alias `ui.Sequence`) para qualquer coisa puramente
visual: fades, easing, floreios próximos a partículas, transições de UI,
animações de revelação de uma vez só e a splash de boot. Se uma mudança
precisa sobreviver a replay/rewind e afetar o resultado da simulação, modele
como estado de scene avançado em `Update`, não como uma timeline de
animação.

`ui.Sequence` é o mesmo tipo que `anim.Timeline` (`type Sequence =
anim.Timeline`), usado através do contrato `Drawer` do pacote `ui` para que
widgets possam disparar uma animação roteirizada chaveada por alguma
mudança de estado arbitrária (`SequenceKeyed(id, key)`): passar `key == 0`
significa "nunca disparado" e retorna uma timeline inativa; mudar `key`
reinicia a sequência a partir do tempo de renderização atual; um `key`
inalterado continua contando a partir do disparo original.

Uma **timeline de apresentação** — o termo usado para a splash compilada da
engine e outro conteúdo de `runtime.SplashSegment` — é o mesmo valor
`anim.Timeline`, só que avaliado pelo `runtime.Driver` contra segundos
decorridos de boot em vez de `ctx.Time`. Existe apenas uma implementação
para os três usos.

## Ownership do relógio

`anim.Timeline` é um valor sem estado, apenas de apresentação: não carrega
relógio de parede ou contador de frames próprio. Ele guarda um snapshot de
tempo decorrido fornecido pelo chamador e avalia as tracks contra ele. O
chamador é dono do relógio e reconstrói (não avança) um `Timeline` a cada
frame:

```go
func (g *Game) Render(dst []render.Command, ctx runtime.RenderContext) []render.Command {
    timeline := anim.At(math.Mod(ctx.Time, duration)) // em loop
    return g.vectorWorkspace.Append(dst, timeline, ctx.Camera().World, VectorSceneResources{})
}
```

`anim.At(elapsed)` constrói uma timeline ativa naquele tempo decorrido;
`anim.Inactive()` constrói uma cujas tracks ficam em zero e reportam
concluído — útil como caso padrão/nunca-disparado. Não existe método
`Advance(dt)`: nada é mutado in-place, então o mesmo valor `Timeline` pode
ser avaliado qualquer número de vezes, de qualquer goroutine, sem
sincronização.

`RenderContext.Time`/`Dt` são o relógio de tempo decorrido de renderização
sobre o qual essa pilha é construída — veja [`docs/runtime.md`](runtime.md)
para como eles divergem dos ticks de simulação durante pausa e rewind.
`UpdateContext` propositalmente não tem campo `Time`: código de simulação
determinístico não tem uso legítimo para tempo de parede, então o tipo não
o oferece.

## `Timeline.Track`

`Track(start, end float64, curve ease.Curve) Track` avalia uma janela com
easing da timeline e retorna um valor pequeno com `Value()`, `Lerp(a, b)` e
`Done()`:

```go
pulse := timeline.Track(1.8, 2.6, ease.InOutQuad)
opacity := pulse.Lerp(0, 0.75)
```

`Done(duration)` na própria timeline reporta se ela está inativa ou já
alcançou `duration` — a forma usual de um segmento decidir que terminou de
tocar.

## `render.Builder`: grupos, composição e comandos não suportados

`render.Builder` é o que código guiado por `Timeline` (escrito à mão ou
gerado pelo `arasy svg`) usa para emitir valores `render.Command` com
estado de transform/opacity/clip/layer já composto, em vez de cada ponto de
chamada fazer essa conta sozinho:

```go
var b render.Builder
b.Reset(dst)
b.Push(render.Group{
    Transform: math2d.Scale(pulse.Lerp(1, 1.3), pulse.Lerp(1, 1.3)).About(center),
    Opacity:   pulse.Lerp(0, 0.75),
})
b.Add(render.DrawCircle{Center: center, Radius: 12, Color: accent})
b.Pop()
commands := b.Commands()
if err := b.Err(); err != nil {
    // tratar/logar — ver abaixo
}
```

`Push` aninhados compõem com o pai: transforms se multiplicam, opacities se
multiplicam, clips se intersectam, e layer offsets se somam. Todo comando
adicionado sob um grupo recebe esse estado composto embutido nos próprios
campos de `BaseCommand`, então um backend nunca precisa percorrer uma pilha
de grupos sozinho.

Métodos do builder nunca entram em pânico por mau uso. Em vez disso, o
primeiro erro é armazenado e todo `Push`/`Add`/`Pop` seguinte vira no-op até
você inspecionar `Err()`:

- `ErrInvalidGroup` — um `Group` com campo não finito.
- `ErrStackUnderflow` — `Pop()` sem nada empilhado.
- `ErrUnclosedGroup` — `Commands()` chamado com um grupo ainda aberto.
- `ErrUnsupportedGroupedCommand` — um tipo de comando sem regra de
  composição por vértice conhecida, ou um dos `DrawParticles`,
  `DrawSpriteBatch`, `DrawTextBatch` adicionado sob um grupo **não
  identidade**. Esses três são comandos retained/de emissor cujo estado do
  lado do backend não pode ser re-derivado por vértice a partir de um
  transform/opacity ad hoc, então o builder recusa em vez de desenhar
  errado silenciosamente.

Confira `Err()` (ou `errors.Is`/`errors.As` contra os tipos sentinela)
depois de construir; código gerado pelo `arasy svg` e código de builder
escrito à mão seguem o mesmo contrato.

## `DrawTriangles` e paridade entre backends

`render.DrawTriangles` é a primitiva única e agnóstica de backend que a
tesselação de curvas (de `render/vector`, ou de fills/strokes de SVG
gerado) acaba emitindo: triângulos indexados de cor sólida, validados para
que a contagem de índices seja múltipla de três e todo índice esteja dentro
do intervalo. Como a tesselação acontece uma vez, antes de qualquer
backend, raster, Canvas2D, WebGPU e native rasterizam exatamente os mesmos
dados de vértice/índice/cor com o mesmo transform — a paridade vem de
compartilhar a geometria, não de quatro implementações separadas
concordarem por convenção. Veja a tabela de responsabilidades de pacotes no
`AGENTS.md` para as ressalvas atuais por backend (por exemplo, os limites
de sprite/label do Canvas2D).

## Documentos relacionados

- [`docs/vector-graphics.md`](vector-graphics.md) — a API de
  path/stroke/motion do `render/vector` e o compilador SVG-para-Go
  `arasy svg` que produz conteúdo guiado por `Timeline`.
- [`docs/runtime.md`](runtime.md) — `RenderContext`/`UpdateContext` e a
  separação entre relógio de tick e de renderização.
- [`docs/ui.md`](ui.md) — o contrato `Drawer`, incluindo `SequenceKeyed`.
