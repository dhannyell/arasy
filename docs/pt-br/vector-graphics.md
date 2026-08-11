# Gráficos Vetoriais

Este guia cobre a API de geometria vetorial da Arasy (`render/vector`) e o
compilador `arasy svg` (`internal/svggen`) que transforma um SVG animado e
autorado em Go determinístico usando essa API. Veja
[`docs/pt-br/animation.md`](animation.md) para a camada `anim.Timeline`/
`render.Builder` sobre a qual esse conteúdo é construído.

## `render/vector`: paths, stroke e movimento

`vector.Path` é geometria de path imutável: o chamador constrói uma com
`vector.Builder` (`MoveTo`, `LineTo`, `QuadTo`, `CubicTo`, `Close`,
`Build()`), que valida invariantes de finitude e de subpath aberto antes de
retornar um `Path`. Internamente, um path guarda apenas segmentos de
move/line/quad/cubic/close — **não existe verbo de arco nativo**; qualquer
coisa com forma de arco é convertida para curvas de Bézier cúbicas antes de
chegar a esse pacote (veja a seção do compilador SVG abaixo).

```go
var b vector.Builder
b.MoveTo(math2d.Vec2{X: 0, Y: 0})
b.CubicTo(c1, c2, math2d.Vec2{X: 100, Y: 0})
b.Close()
path, err := b.Build()
```

### Fill e stroke

Preencher e traçar achatam as curvas do path em segmentos de reta a uma
tolerância dada e emitem `render.DrawTriangles`. O achatamento é uma
subdivisão recursiva adaptativa (limitada a 18 níveis de recursão) contra um
teste de achatamento, e cada `Path` guarda em cache a última tolerância
usada, então desenhos repetidos numa escala inalterada não re-achatam a
cada frame.

`StrokeStyle` carrega `LineCap`, `LineJoin` e `TrimStart`/`TrimEnd` (frações
em `[0, 1]` ao longo do comprimento total do path) para um stroke parcial —
o mecanismo do qual um "traço de anel" ou animação de arco de progresso é
construído. Valores de trim são validados: valores fora de faixa ou
invertidos (`TrimEnd < TrimStart`) retornam `ErrInvalidStroke` em vez de
desenhar algo degenerado.

### Amostragem de movimento

`Path.PointAt(progress, tolerance)` percorre o path achatado por
comprimento de arco e interpola linearmente uma posição e tangente em
`progress * comprimentoTotal` — a primitiva da qual uma animação de
movimento tipo "offset-path" (uma partícula ou ícone andando sobre uma
curva) é construída. Retorna `ErrInvalidProgress` para `progress` fora de
`[0, 1]` e `ErrDegenerateGeometry` para um path de comprimento zero.

### Erros

Toda operação falível retorna um erro sentinela em vez de entrar em
pânico: `ErrNonFinite`, `ErrNoCurrentSubpath`, `ErrEmptyPath`,
`ErrInvalidTolerance`, `ErrFlattenDepth`, `ErrFillContour`,
`ErrSelfIntersection`, `ErrDegenerateGeometry`, `ErrInvalidStroke`,
`ErrInvalidProgress`. Confira esses com `errors.Is` da mesma forma que os
erros do `render.Builder`.

## `arasy svg`: compilação de SVG para Go

`arasy svg <origem.svg> --output <arquivo.gen.go> --package <pkg> --name
<Nome>` compila um SVG animado e autorado em um arquivo `.gen.go` sem
dependência em tempo de execução de `internal/svggen`, XML ou CSS — o
código gerado só importa `anim`, `ease`, `math2d`, `render` e
`render/vector`.

```powershell
go run ./cmd/arasy svg scene.svg `
  --output examples/vectoranim/vector.gen.go `
  --package vectoranim `
  --name VectorScene `
  --font "Display=ui-display"
```

Flags: `--output`, `--package` e `--name` são obrigatórias. `--font
FAMILIA=NOME_NO_MANIFEST` (repetível) mapeia um `font-family` do SVG para um
nome que o chamador resolve em tempo de execução a partir de um
`asset.Manifest` carregado — mapeamentos de família duplicados são
rejeitados. `--strict` (padrão `true`) transforma qualquer atributo fora do
subconjunto suportado em erro de compilação em vez de descartá-lo
silenciosamente; desative só enquanto itera num SVG novo, nunca para um
asset já commitado.

### Subconjunto suportado

Elementos: `svg, g, rect, circle, ellipse, line, polyline, polygon, path,
text, tspan, style`. Qualquer outra coisa — um namespace diferente, uma tag
não suportada — falha ao fazer parse.

Atributos são uma lista de permissão explícita cobrindo geometria (`x, y,
cx, cy, r, rx, ry, width, height, points, d, x1, y1, x2, y2`), apresentação
(`fill, stroke, stroke-width, stroke-linecap, stroke-linejoin,
stroke-miterlimit, opacity, stroke-dasharray, stroke-dashoffset`), texto
(`font-family, font-size, font-weight, text-anchor, letter-spacing`),
transform (`transform, transform-box, transform-origin, viewBox,
preserveAspectRatio`), animação CSS por keyframes (`animation,
animation-name, animation-duration, animation-delay,
animation-iteration-count, animation-direction, animation-fill-mode,
animation-timing-function`), movimento (`offset-path, offset-distance`),
além de `id, class, style, pathLength, filter, type, media`. No modo não
estrito, um atributo fora dessa lista é descartado; no modo estrito, é erro
de compilação.

Conteúdo proibido é rejeitado incondicionalmente, independente de
`--strict`: declarações de doctype/entity, instruções de processamento de
XML stylesheet, `<script>`, URLs `javascript:`, `@import` e referências
`url(http:/https:///)` — nada buscado externamente ou executável sobrevive
à compilação. Também há limites rígidos de tamanho de entrada, contagem de
elementos, profundidade de aninhamento, contagem de comandos de path,
magnitude de coordenadas e tamanho da saída gerada, então uma entrada
patológica falha rápido em vez de produzir um `.gen.go` descontrolado.

Dados de path aceitam o conjunto clássico de comandos — `M/m, L/l, H/h,
V/v, C/c, S/s, Q/q, T/t, A/a, Z/z` — incluindo arcos. Arcos são convertidos
para uma sequência de comandos de Bézier cúbica durante o parse, então
quando a geometria chega ao `render/vector` só restam
move/line/quad/cubic/close; não há primitiva de arco em tempo compilado,
para manter pequeno o vocabulário de runtime.

### Modelo de custo: LOD e tolerância

O código gerado classifica a escala atual do viewport em um de três níveis
de detalhe — grosseiro abaixo de 0.75x, fino acima de 1.5x, padrão no meio
— com histerese nas fronteiras para que um viewport pairando perto de um
limiar não pisque entre níveis de LOD de um frame para o outro. Cada nível
de LOD mapeia para uma tolerância de achatamento fixa (mais frouxa no nível
grosseiro, mais apertada no fino), que alimenta diretamente o achatamento
de fill/stroke do `vector.Path`. A travessia de nós também descarta
subárvores contra os limites do viewport assim que um asset gerado tem nós
suficientes para essa checagem valer a pena, em vez de sempre percorrer a
árvore inteira.

### Formato do arquivo gerado

Todo arquivo `.gen.go` expõe o mesmo formato, só prefixado por `--name`:

- `<Nome>Resources` — um campo `render.FontID` por mapeamento `--font`.
- `<Nome>Metrics` / `<Nome>Budget` / `WithinBudget` — limites de custo
  estrutural (contagens de nós/paths/texto/profundidade/comandos) que um
  chamador pode impor.
- `<Nome>AssetMetrics` — as contagens computadas em tempo de compilação
  para este asset.
- `<Nome>Workspace` — estado retido por chamada (o buffer de comandos, um
  `render.Builder`, workspaces de tesselação de fill/stroke por nó, e
  estado de LOD), reutilizado entre frames para evitar realocação.
- `(w *<Nome>Workspace) Append(timeline, viewport, resources)
  []render.Command` e um wrapper `Append<Nome>(dst, timeline, viewport,
  resources) []render.Command` sem estado, em nível de pacote.
- `<Nome>FrameMetrics` — contagens de nós visitados/descartados, contagem
  de comandos emitidos, LOD e tolerância da última chamada `Append`, útil
  para profiling ou golden tests de custo de travessia.

### Workflow: SVG de origem → Go gerado → recursos do manifest

1. Autore ou gere o SVG (feito à mão, ou produzido por uma ferramenta de
   autoria externa à engine).
2. Compile com `arasy svg`, escolhendo `--package`/`--name` para o ponto de
   chamada que vai possuí-lo, e mapeamentos `--font` para qualquer texto
   que o SVG contenha.
3. Faça commit do arquivo `.gen.go` gerado ao lado do código que o chama —
   ele não tem dependência de build de `internal/svggen`, então nada em
   tempo de execução precisa do compilador.
4. No ponto de chamada, construa (ou reutilize um `<Nome>Workspace` em
   cache), resolva quaisquer mapeamentos `--font` contra um
   `asset.Manifest` carregado dentro da struct `<Nome>Resources` gerada, e
   chame `Append` uma vez por frame com uma `Timeline` avaliada a partir de
   `ctx.Time` (ou segundos decorridos de boot, para conteúdo de splash).

Glifos de texto são delineados em dados de path em tempo de geração em vez
de referenciar `render.DrawText` com um `FontID` quando o SVG é feito para
não redistribuir a fonte de autoria — a splash da engine faz isso: nunca
popula o campo de fonte da própria struct `Resources`, porque seu wordmark
já é dado de path puro. Popule mapeamentos `--font` só quando o SVG
genuinamente precisa de texto vivo, renderizado pela engine (por exemplo,
labels editáveis em tempo de execução), não para um wordmark ou logotipo
fixo.

### Regeneração e revisão de código

A saída do `arasy svg` é determinística — gerar do mesmo documento parseado
duas vezes produz Go byte-idêntico — mas nada atualmente reroda o
compilador no CI e compara com o `.gen.go` commitado. Trate o SVG de origem
como o artefato revisável para mudanças de design, e trate o arquivo
gerado como saída de build: revise um diff de `.gen.go` regenerado da forma
que revisaria qualquer outra atualização de artefato de build (o formato do
diff bate com a mudança de origem, não "cada linha faz sentido sozinha"), e
nunca edite um arquivo `.gen.go` à mão — regenere-o.

## Matriz de backends

`render.DrawTriangles` — a primitiva que fill/stroke vetorial e conteúdo de
SVG gerado acabam emitindo — é rasterizado de forma idêntica por todo
backend porque a tesselação acontece uma vez, antes de qualquer backend.
Notas atuais por backend (veja a tabela de responsabilidades de pacotes no
`AGENTS.md` para a versão autoritativa e em evolução):

| Backend | Papel |
|---|---|
| `platform/raster` | Renderizador por software para `image.RGBA`; a referência semântica com a qual todo outro backend é comparado. |
| `platform/wasm` (WebGPU) | Backend web padrão; paridade completa com a referência. |
| `platform/wasm` (Canvas2D) | Backend web de fallback; aproxima quads com uma grade afim e limita tamanhos de batch retained de sprite/label (limites só de dev). |
| `platform/native` | Host wgpu-native + GLFW; batches retained estáticos de sprite usam registros persistentes na GPU. Batches animados de sprite e batches retained de texto continuam sem suporte. |

## Extensões futuras

O compilador e o `render/vector` deliberadamente deixam o seguinte fora de
escopo por enquanto — anotado aqui para que uma mudança futura saiba que é
uma lacuna intencional, não um descuido:

- Regras de preenchimento além de winding nonzero/evenodd, e buracos via
  paths compostos.
- Animação estilo SMIL (`<animate>`/`<animateTransform>`), como alternativa
  ao subconjunto atual, só de keyframes CSS.
- Gradientes e máscaras como paints de fill/stroke.
- Morphing de path (interpolar entre dois valores de `d`).
- Batches vetoriais retained, análogos ao caminho de batch retained de
  sprite/texto, para cenas com muitas formas vetoriais animadas
  independentemente.
- Um cache de custo de tesselação/achatamento medido e compartilhado entre
  assets, em vez do cache atual de tolerância única por `Path`.

## Documentos relacionados

- [`docs/pt-br/animation.md`](animation.md) — `anim.Timeline`, `ui.Sequence` e
  composição de grupos do `render.Builder`.
- [`docs/pt-br/testing.md`](testing.md) — onde vivem os testes de parser e
  codegen do `internal/svggen` (incluindo seus alvos de fuzz) e como estão
  organizados.
