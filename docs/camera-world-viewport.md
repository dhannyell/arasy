# Camera, World e Viewport

Este guia explica a diferenca entre `World` e `Viewport` na camera da Frameforge.

Esses conceitos sao importantes porque a engine nao deve misturar coordenadas do jogo com pixels da tela.

## Ideia principal

```text
World:
  coordenadas do jogo

Viewport:
  coordenadas da imagem/tela onde o jogo sera desenhado
```

Exemplo:

```text
World:
  campo de futebol medindo 100 x 75 unidades

Viewport:
  imagem PNG medindo 800 x 600 pixels
```

A camera mapeia um para o outro:

```text
World 100 x 75  ->  Viewport 800 x 600
```

Ou seja:

```text
1 unidade do mundo = 8 pixels
```

Porque:

```text
800 / 100 = 8
600 / 75  = 8
```

## Por que separar World e Viewport?

Sem essa separacao, a simulacao precisaria saber o tamanho da tela.

Isso seria ruim:

```go
playerPosition := Vec2{X: 640, Y: 360}
```

Esse valor significa o que?

- Pixel da tela?
- Posicao no campo?
- Posicao no mundo?
- Posicao no Canvas?

Com `World`, a simulacao usa coordenadas do jogo:

```go
playerPosition := Vec2{X: 50, Y: 37.5}
```

Isso significa:

```text
meio do campo
```

Depois a camera transforma isso em pixel:

```text
{50, 37.5} no mundo -> {400, 300} na tela
```

Assim o mesmo jogo pode ser desenhado em resolucoes diferentes:

```text
World: {50,37.5}

Viewport 800 x 600:
  screen = {400,300}

Viewport 1600 x 1200:
  screen = {800,600}
```

A posicao no jogo continua igual.

## World

`World` representa a area do jogo que a camera esta vendo.

Na Frameforge:

```go
World: math2d.NewRect(
    math2d.Vec2{X: 0, Y: 0},
    math2d.Vec2{X: 100, Y: 75},
)
```

Isso significa:

```text
Min = {0,0}
Max = {100,75}
```

Visualmente:

```text
World coordinates

Y=75  +--------------------------------+
      |                                |
      |                                |
      |          campo do jogo         |
      |                                |
Y=0   +--------------------------------+
      X=0                            X=100
```

Tudo na simulacao usa essas coordenadas:

```text
jogador: {30,35}
bola:    {52,36}
gol:     {100,37.5}
```

## Viewport

`Viewport` representa a area da imagem/tela onde o mundo sera desenhado.

Na Frameforge:

```go
Viewport: math2d.NewRect(
    math2d.Vec2{X: 0, Y: 0},
    math2d.Vec2{X: 800, Y: 600},
)
```

Isso significa:

```text
Min = {0,0}
Max = {800,600}
```

Visualmente:

```text
Viewport pixels

Y=600 +--------------------------------+
      |                                |
      |                                |
      |        imagem renderizada      |
      |                                |
Y=0   +--------------------------------+
      X=0                            X=800
```

No renderer raster, esses valores sao pixels da imagem.

No futuro, no Canvas, tambem podem ser pixels ou unidades CSS/canvas.

## Camera

A camera junta `World` e `Viewport`.

```go
camera := render.Camera{
    World: math2d.NewRect(
        math2d.Vec2{X: 0, Y: 0},
        math2d.Vec2{X: 100, Y: 75},
    ),
    Viewport: math2d.NewRect(
        math2d.Vec2{X: 0, Y: 0},
        math2d.Vec2{X: 800, Y: 600},
    ),
}
```

Ela responde:

```text
onde esse ponto do jogo fica na tela?
```

## WorldToScreen

`WorldToScreen` converte uma posicao do mundo para uma posicao na tela.

Exemplo:

```go
screen := camera.WorldToScreen(math2d.Vec2{X: 50, Y: 37.5})
```

Resultado:

```text
{400,300}
```

Visualmente:

```text
World 100 x 75                  Viewport 800 x 600

{50,37.5}                       {400,300}
    *          ----camera---->      *
```

Formula simplificada:

```text
normalizedX = (point.X - World.Min.X) / World.Width
screenX     = Viewport.Min.X + normalizedX * Viewport.Width

normalizedY = (point.Y - World.Min.Y) / World.Height
screenY     = Viewport.Min.Y + normalizedY * Viewport.Height
```

Exemplo numerico:

```text
point.X = 50
World.Min.X = 0
World.Width = 100
Viewport.Width = 800

normalizedX = (50 - 0) / 100
normalizedX = 0.5

screenX = 0 + 0.5 * 800
screenX = 400
```

## ScreenToWorld

`ScreenToWorld` faz o inverso.

Ele converte uma posicao da tela para uma posicao do mundo.

Isso e util para input.

Exemplo:

```go
world := camera.ScreenToWorld(math2d.Vec2{X: 400, Y: 300})
```

Resultado:

```text
{50,37.5}
```

Uso futuro:

```text
usuario clica no pixel {400,300}
camera converte para mundo {50,37.5}
jogo entende que o clique foi no meio do campo
```

## Viewport com offset

O viewport nao precisa comecar em `{0,0}`.

Imagine que a tela inteira tem 1280 x 720, mas o campo fica dentro de uma area menor:

```text
tela inteira:
  1280 x 720

viewport do campo:
  x = 100
  y = 80
  width = 1000
  height = 560
```

Em codigo:

```go
Viewport: math2d.NewRect(
    math2d.Vec2{X: 100, Y: 80},
    math2d.Vec2{X: 1000, Y: 560},
)
```

Visualmente:

```text
tela
+------------------------------------------------+
|                                                |
|   viewport do campo                            |
|   +----------------------------------------+   |
|   |                                        |   |
|   |              campo                     |   |
|   |                                        |   |
|   +----------------------------------------+   |
|                                                |
+------------------------------------------------+
```

Isso permite desenhar o jogo dentro de paineis de UI.

## Escala

A escala diz quantos pixels correspondem a uma unidade de mundo.

Exemplo:

```text
World:
  100 x 75

Viewport:
  800 x 600

Scale X:
  800 / 100 = 8

Scale Y:
  600 / 75 = 8
```

Entao:

```text
1 unidade de mundo = 8 pixels
```

Se um jogador tem raio `2.5` no mundo:

```text
2.5 * 8 = 20 pixels
```

Se uma linha tem largura `0.35` no mundo:

```text
0.35 * 8 = 2.8 pixels
```

## worldWidthToScreen

Essa funcao helper ainda nao e parte obrigatoria da API publica, mas pode existir no renderer raster para converter medidas.

Exemplo:

```go
func (r Renderer) worldWidthToScreen(width float64) float64 {
    worldSize := r.Camera.World.Size()
    viewportSize := r.Camera.Viewport.Size()

    return width * (viewportSize.X / worldSize.X)
}
```

Ela converte:

```text
largura no mundo -> largura em pixels
```

Uso:

```go
radius := r.worldWidthToScreen(command.Radius)
lineWidth := r.worldWidthToScreen(command.Width)
strokeWidth := r.worldWidthToScreen(command.StrokeWidth)
```

## Aspect ratio

O exemplo ideal tem a mesma escala em X e Y:

```text
World:    100 x 75
Viewport: 800 x 600

Scale X = 8
Scale Y = 8
```

Mas nem sempre isso acontece.

Exemplo:

```text
World:    100 x 100
Viewport: 800 x 600

Scale X = 8
Scale Y = 6
```

Nesse caso, se voce mapear diretamente:

```text
circulos podem parecer elipses
o mundo pode ficar esticado
```

Para a V1, a solucao simples e escolher `World` e `Viewport` com a mesma proporcao.

Exemplo bom:

```text
World:    100 x 75
Viewport: 800 x 600
Aspect:   4:3 nos dois
```

Mais tarde, a camera pode ganhar recursos como:

- manter aspect ratio;
- letterbox;
- pillarbox;
- zoom;
- offset;
- inversao do eixo Y;
- rotacao.

## Eixo Y

Este guia desenha Y para cima, mas imagens normalmente usam Y crescendo para baixo:

```text
imagem:

{0,0} fica no canto superior esquerdo
Y cresce para baixo
```

Por enquanto, a camera da Frameforge nao inverte o eixo Y.

Ela assume que `World` e `Viewport` usam a mesma orientacao.

Isso e suficiente para a V1.

Mais tarde podemos adicionar:

```go
FlipY bool
```

ou criar uma convencao clara:

```text
mundo do jogo usa Y para baixo
```

## Resumo

```text
World:
  coordenadas do jogo
  exemplo: campo 100 x 75

Viewport:
  coordenadas da saida visual
  exemplo: PNG 800 x 600

Camera:
  converte entre World e Viewport

WorldToScreen:
  jogo -> tela

ScreenToWorld:
  tela -> jogo
```

A separacao permite que a simulacao seja independente da resolucao, do renderer e da plataforma.

