# Batches Retidos E Interpolação

## Batches retidos

`render.DrawSpriteBatch` e `render.DrawTextBatch` desenham conteúdo publicado
uma vez por `runtime.BatchPort`. A porta é compartilhada por `Game.Init`,
`Game.Update` e `Game.Render`.

Use batches retidos para conteúdo estável ou que muda raramente, como multidões,
cenário e elementos de HUD. Mantenha sprites ECS dinâmicos no caminho de
streaming.

### Contrato de publicação

- `UpdateSprites` e `UpdateText` substituem o batch completo.
- `DeleteSprites` e `DeleteText` removem um batch.
- Uma substituição vazia mantém um batch vazio existente.
- Um comando que nomeia um batch ausente é ignorado e gera um warning.
- Um comando de batch possui uma layer e um clip e cria uma fronteira de ordem
  de pintura.
- Conteúdo retido é render-only. Ele não entra em checksums, snapshots,
  gravações ou replay. Rewind não desfaz uma publicação.

### Bounds e animação

Runtime faz cópias defensivas e calcula bounds de mundo quando o conteúdo é
publicado. O culling é do batch inteiro e conservador. O renderer infla os
bounds pela animação antes de testar a view.

A animação visual suportada é determinística e apenas de apresentação:

- `SpriteAnimation` suporta flip-book, phase e bounce.
- `SpritePulse` suporta pulse.
- `SpriteBatchAnimation` controla a animação do batch.

Raster, Canvas2D e WGSL avaliam essas animações por `internal/renderbatch`. O
sistema não aceita shaders arbitrários, não reordena instâncias e não executa
culling ou compute por instância.

### Comportamento dos hosts

WebGPU reutiliza os pipelines de storage-buffer do caminho de streaming. Conteúdo
estável envia zero bytes por frame. Raster é a referência. O Canvas2D de
desenvolvimento limita conteúdo retido a 2.000 sprites e 500 labels.

Batches retidos e partículas não suportam `Camera.View` com skew. O caminho GPU
de tessellation recebe escala e offset, não um shear geral.

## Interpolação de render

`runtime.Config.RenderInterpolation` ativa a interpolação. O padrão é desativado.
`runtime.RenderContext.Alpha` expõe a fração entre o tick fixo atual e o próximo.

`scene.World.SyncRenderState()` captura o tick anterior. Jogos pedem comandos
interpolados com:

```go
ctx.World.RenderCommandsInterpolated(ctx.Alpha)
```

A interpolação se aplica somente a `Position`, `Rotation` e `Scale`. Ela usa
`Vec2.Lerp` e o helper de interpolação matemática. Overlays de debug, como
colliders, grid e velocidade, mostram o estado atual da simulação.

Interpolação é apenas de apresentação. Ela não entra em checksums, snapshots,
replays, rewind ou gravações. `SetTransform` ressincroniza o estado de render
imediatamente para que teleporte ou reset não cause smear em um frame.

## Limites da câmera

Formas imediatas, imagens e texto percorrem a geometria por `WorldToScreen` e
respeitam a view da câmera. Batches retidos e partículas não carregam shear
geral pelo stream GPU. Veja [2.5D](2-5d.md) para projeção, planos e ordenação de
profundidade.
