# Arquitetura

## Camadas

```text
foundation:     math2d
vocabulary:     render, input, ui, audio, asset, particle
simulation:     scene
orchestration:  runtime
hosts:          platform/raster, platform/wasm, platform/native
apps:           cmd/, examples/
```

```text
cmd/examples -> platform/* -> runtime -> scene/input/ui/render/audio/asset -> math2d
asset -> render/audio/particle -> math2d
platform/* -> asset/pixel -> asset
```

## Regras de dependência

- `math2d`, `render`, `input`, `ui`, `scene` e `runtime` não importam
  `platform/*`.
- `runtime` é a única API comportamental usada por plataformas e jogos.
- Plataformas chamam `runtime.Driver`; elas não dirigem `scene.World` diretamente.
- `runtime` não depende de JSON, HTTP, imagens ou conceitos de navegador.
- `asset` interpreta manifests. Runtime transporta o `asset.Manifest` resolvido.
- Jogos implementam `runtime.Game` e recebem o mundo por um contexto.

## Responsabilidades dos pacotes

### `math2d`

`math2d` contém matemática 2D pura, como `Vec2` e `Rect`. O pacote não possui
dependências de engine ou plataforma.

### `fixed` (módulo externo)

`github.com/dhannyell/fixed` (futuro `arasy-engine/fixed`) fornece o tipo
Q32.32 determinístico da simulação — `Q`, `Vec2`, `Rot` e trigonometria em
voltas; a engine o consome através do `dmath`.

### `render`

`render` define comandos independentes de backend, cores, medição de texto,
câmeras e `CommandSink`. `DrawImageQuad` mapeia uma textura para um quad
convexo. Formas, imagens e texto imediato usam a transformação mundo-tela da
câmera.

`BaseCommand.Transform` é uma transformação afim por comando. A identidade é o
valor zero. Ela é decorativa e não afeta hit testing. `DrawParticles`,
`DrawSpriteBatch` e `DrawTextBatch` possuem estado retido ou de emissor no
backend e não suportam todas as transformações de câmera.

`MSAAMode` é a opção compartilhada de antialiasing dos hosts GPU. Raster e
Canvas2D a ignoram. Raster é a referência para geometria e cores.

O vocabulário 2.5D também vive aqui: `Camera.DepthScale` e `Camera.View`
declaram a projeção, `GroundProjection` deriva delas a apresentação de
altura, `FrameGround` emoldura um retângulo de chão sob uma vista, e
`Projection.Wall`/`ExtrudeRect`/`LiftVector` autoram volume falso ao longo do
mesmo lift que levanta as entidades. `BaseCommand.BlendMode` escolhe o
operador de composição (`BlendAdditive` para brilhos que acumulam luz).

Veja [2.5D](2-5d.md) e [batches retidos](render-batches.md).

### `input`

`input` mapeia botões físicos, eixos, ponteiro, teclado e texto digitado para
ações do jogo. O driver encerra cada frame de input depois do passo de
simulação. Eventos de texto carregam valores Unicode e não adicionam bytes às
gravações. `PointerFrame.Clicks` responde a pergunta comum "o jogador
clicou, e onde". `Repeater` (cadência de tecla segurada) e `StickDPad`
(stick analógico como pad de quatro direções, com histerese) são helpers
guiados por dt com estado exportado, feitos para viver dentro do estado de
jogo snapshotado, para que replays os carreguem tick a tick.

### `ui`

`ui` fornece widgets immediate-mode e três espaços de coordenadas. O chamador
é dono dos valores dos widgets. O estado interno pertence a `ui.State` e é
capturado pelo driver para rewind. Popups usam a camada de overlay.

### `asset`

`asset` carrega manifests em camadas de `fs.FS`. A última camada vence. IDs de
textura e fonte são estáveis durante uma execução e resolvidos por nome. Eles
não são persistidos. Decodificação de imagens pertence a `asset/pixel`. O
parser SFNT e OpenType pertence a `asset/typeface`.

### `audio`

`audio` define IDs de clipes, cues de uma execução, tracks contínuas e estado de
track rebobinável. Hosts decodificam e tocam o áudio. Runtime transporta a
intenção de áudio.

### `particle`

`particle` define efeitos render-only e a referência determinística em CPU. O
pacote usa pools fixos de emissores, hashing de spawn, bitsets de slots ativos,
contabilidade de emissão e budgets adaptativos de fallback. WebGPU espelha essas
regras em compute shaders.

### `scene`

`scene` possui o mundo ECS e o passo de simulação. O pipeline é locomoção,
aceleração, balística, integração, bounce, broad phase, resolução de contatos e
damping.

O mundo suporta altitude, arrasto, força de Magnus, bounce, rolagem, decay de
spin, substeps, steering, ordenação de profundidade e apresentação 2.5D
render-only. A ordem usa `(Layer, depth key, EntityID)`, com a chave resolvida
uma vez por sprite por frame: a base do próprio sprite projetada no
`World.SortAxis`, deslocada pelo `SortOffset`. O desempate final por
`EntityID` torna o frame reproduzível. Height afeta a apresentação, mas não a
chave de profundidade. `CommandSprite` deixa um prop de comandos crus entrar
nessa ordenação, `Attachment` ancora um sprite a um pai para o composto
ordenar como uma coisa só, e `World.Hop` leva uma entidade em arco como
estado de simulação.

Componentes de simulação entram em `World.Checksum()`. Valores de apresentação,
como altura de render, transformações de sprite, projeção e plano do sprite,
não entram.

### `runtime`

`runtime` possui `Game`, `Driver`, execução em timestep fixo, controles de
desenvolvimento, rewind, tempo de frame, estatísticas de frame, estatísticas de
GC, `StateSlot[T]`, consumo de áudio e publicação de batches retidos.

Runtime define o significado de um tick. Hosts não devem duplicar a lógica de
tick. Runtime também transporta assets resolvidos e expõe contextos de render.

### `internal/devpanel`

O painel de desenvolvimento é um overlay UI portátil. Ele só compila com a tag
`arr_dev`. Ele possui seu próprio `ui.State`; o estado do painel não entra em
snapshots ou checksums do jogo. Input capturado pelo painel não chega ao jogo ou
a uma gravação. Hosts de browser e native fornecem o seam `RecordingPort`.

O painel fornece controles de simulação, scrub de timeline, gravação e watch,
telemetria de performance, inspeção de estado, seleção de entidades, edição de
campos escalares e booleanos e camadas de debug. Ele emite comandos acima das
camadas do jogo. Um painel fechado não emite comandos. O painel mantém o
ponteiro enquanto está sob hover ou um widget está ativo e mantém o teclado
enquanto um widget possui foco.

### `internal/renderbatch`

`internal/renderbatch` possui cópias defensivas, bounds de mundo, avaliação de
animação, inflação de culling e interseção com a view para batches retidos.
Raster, Canvas2D e WGSL usam as mesmas regras.

### `anim`, `render/vector` e `internal/svggen`

`anim` fornece timelines de apresentação sem estado mutável. `render/vector`
fornece geometria imutável de paths cúbicos e quadráticos. `internal/svggen`
compila o subconjunto suportado de SVG/CSS animado para código Go determinístico
de render.

### Pacotes GPU

`internal/render/gpu/tess` constrói streams ordenados de pipelines GPU. O host
WASM WebGPU o usa, e o renderer native compartilha o mesmo núcleo. `fontatlas`
fornece atlas de glyphs baseado em manifest. `shaders` contém fontes WGSL
embutidas.
`internal/render/frameplan` normaliza a política de passes e alvos para todos os
backends; os hosts apenas ativam recursos e codificam os passes resultantes.

### Hosts de plataforma

- `platform/raster` renderiza para `image.RGBA` e é a referência semântica
  offline.
- `platform/wasm` dirige `requestAnimationFrame`, usa WebGPU por padrão e cai
  para Canvas2D. WebGPU envia texturas e atlas de glyphs para cada
  `(FontID, pixelSize)`, gera mip chains, usa amostragem anisotrópica e executa
  partículas com compute e draws indiretos. Canvas2D aproxima quads e partículas
  em CPU, usa `FontFace` do browser para fontes externas e limita batches
  retidos a 2.000 sprites e 500 labels.
- `platform/wasm` usa `Options.MSAA`, `Options.DevTools` e
  `Options.RecordReplay`. Dev tools exigem `arr_dev`; gravação no browser usa
  Blob e file picker.
- `platform/native` usa GLFW e wgpu-native. Ele trava a thread do SO, enfileira
  callbacks, aplica input por `Driver.RunTick` e suporta áudio native.
  `Options.GPUTiming` pede timestamp queries na criação do device. O readback é
  assíncrono, então a amostra reportada fica atrasada. Frames com partículas
  usam intervalos de emit, simulate e render. Frames sem partículas pulam compute.
- Dev tools native suportam fullscreen, gravação, sondas de sistema, métricas de
  frame, tempo por pass GPU e telemetria de GC. O renderer pode usar um caminho
  offscreen para partículas e cai para render direto quando a configuração
  offscreen opcional falha.
- A criação do device pede limites do adapter e cai para defaults do driver em
  caso de erro. O host native usa o fork configurado de wgpu-native e um sink
  global de processo `oto` para áudio do manifest.

Os dois hosts GPU compartilham tessellation, shaders, partículas, MSAA e métricas
de frame. O ciclo de vida e a validação de cada host ficam na documentação de
host.

## Restrições de design

- Mantenha os pacotes da engine independentes de regras de aplicação.
- Prefira APIs pequenas e explícitas.
- Adicione complexidade apenas sob pressão real do projeto.
- Mantenha comandos de render independentes de backend.
- Meça antes de otimizar.
- Prefira a biblioteca padrão. As dependências externas da engine são as
  documentadas em `go.mod`.

## Documentos relacionados

- [Runtime](runtime.md)
- [Testes](testing.md)
- [Comandos de desenvolvimento](development.md)
- [Projeção 2.5D](2-5d.md)
- [Batches retidos e interpolação](render-batches.md)
- [Boot splash](boot-splash.md)
