# 2.5D: estado atual e pontos de melhoria

Levantamento de onde a engine está para jogos 2.5D em geral — não só o
isométrico do `examples/outpost`. Escrito em 2026-08; revisite quando
qualquer item virar feature.

## O que já está resolvido

O princípio que organiza o 2.5D aqui é que a projeção é **declarada, e o
resto deriva** — inclinação, altura, sombra e ordenação não são problema do
jogo:

- `Camera.DepthScale` e `Camera.View` — a projeção mora na câmera, então
  picking (`ScreenToWorld`), debug overlays e física continuam no grid
  quadrado.
- `Height` + `Ballistics` + `GroundProjection` — altura é estado de simulação
  e a sombra que descola do corpo sai de graça.
- `PlaneUpright`/`PlaneGround` com correção automática — o engine calcula a
  forma que um sprite em pé precisa ter sob a vista, sem correção de
  cisalhamento escrita à mão no jogo.
- Base-sort com `SortAxis`/`SortOffset`, `CommandSprite` (props de comandos
  crus no painter sort), `Attachment` (compostos que ordenam como uma coisa
  só) e `Projection.Wall`/`ExtrudeRect` (volume extrudado no mesmo lift das
  entidades).

## Lacunas, em ordem de impacto

### 1. Texturas em lote ignoram a `View`

`DrawSpriteBatch`, `DrawTextBatch` e `DrawParticles` recebem a câmera como
escala+offset e não carregam cisalhamento (limite já documentado). Efeito
prático: **um jogo isométrico feito de arte em textura — o caso mais comum —
não pode usar os caminhos rápidos**; o outpost contorna sendo 100% vetorial.
É a lacuna que mais restringe "que tipo de jogo 2.5D dá para fazer". Exige
uniform de matriz completa nos dois backends de GPU.

### 2. Não existe tilemap

Nenhum suporte a mapa de tiles — nem renderer, nem autotile, nem import dos
formatos de editor de mapa que os jogos já usam (Tiled, LDtk). Para 2.5D
isso pesa dobrado, porque o chão é sempre um tilemap. O `boardMap` de texto
do outpost é o embrião do que uma API mínima poderia ser: grid + atlas +
camadas, com a `View` fazendo o resto.

### 3. Altura não entra no picking

`ScreenToWorld` inverte a vista no plano do chão (z=0). Clicar num corpo
levantado — unidade no alto do pulo, prop num terreno elevado — acerta o
tile *atrás* dele, porque o lift deslocou o corpo na tela. Falta o inverso
do lift: um `Projection.ScreenToGround(cam, screen, z)` que responda "que
ponto do chão está sob este pixel, assumindo altura z". Hoje cada jogo teria
que subtrair `LiftVector().Mul(z)` à mão.

### 4. Altura não entra na colisão

Colliders são 2D: um projétil com `Height.Z = 10` ainda colide com um muro
que ele visualmente sobrevoa. O componente que falta é pequeno — uma altura
no collider (`Tall`/`ZExtent`) e o teste "só colide se os intervalos de Z se
cruzam" — e destrava arremessos por cima de obstáculos, pontes e o tiro em
arco de qualquer jogo de ação isométrico.

### 5. Terreno não tem elevação

`Height` é por entidade; o chão é plano. Jogos de tática com plataformas
precisam de elevação por tile: o desenho já sai com `ExtrudeRect`, mas pouso
do projétil (`Ballistics` assume chão em z=0), sombra (sempre projetada em
z=0) e picking não sabem que o chão subiu. Um `GroundHeight` consultável
pela projeção e pela balística cobriria os três.

### 6. Oclusão sem resposta de gameplay

Personagem atrás de um prédio some — e o padrão do gênero tático é silhueta
ou recorte. A engine não tem "este sprite está ocluso por aquele". Uma
consulta barata (comparar chave de profundidade + sobreposição de footprint
na tela) permitiria ao jogo desenhar a silhueta; o recorte per-pixel é outro
problema (depth buffer) e pode continuar fora.

### 7. Estilos de 2.5D ainda sem caminho

- **Parallax de camadas** (side-view 2.5D): uma câmera só; falta fator de
  parallax por layer ou câmeras por grupo de comandos. A engine que declara
  projeção poderia declarar isso também.
- **Chão em perspectiva (estilo Mode-7)**: `View` é afim; perspectiva pede
  homografia. `DrawImageQuad` já interpola com correção de perspectiva,
  então o material existe — falta a câmera saber usá-lo.
- **Billboard num raycaster** está razoavelmente fora de escopo, e tudo bem.

## Leitura recomendada de prioridade

O item 1 é o único que *bloqueia* uma classe de jogo; 2 e 3 são os que todo
segundo jogo 2.5D vai pedir; 4 e 5 transformam o 2.5D de visual em mecânica;
6 e 7 são polimento de gênero.
