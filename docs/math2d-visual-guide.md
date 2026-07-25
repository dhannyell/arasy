# Math2D Visual Guide

Este guia explica visualmente as ideias basicas do pacote `math2d`.

O objetivo nao e substituir os testes ou a documentacao do Go, mas ajudar a criar intuicao para `Vec2`, `Rect`, distancias, direcoes e regras de borda.

## Sistema de coordenadas

`math2d` nao define sozinho se o eixo Y cresce para cima ou para baixo. Isso depende do sistema que usa os valores:

- Mundo do jogo.
- Tela.
- Canvas.
- Campo normalizado.
- UI.

Os exemplos deste guia usam o eixo X crescendo para a direita e o eixo Y crescendo para cima apenas para facilitar a leitura:

```text
Y
^
|
|
|
+----------------------> X
```

## Vec2

`Vec2` representa um ponto, uma direcao, uma velocidade ou um deslocamento em 2D.

```go
type Vec2 struct {
    X float64
    Y float64
}
```

Exemplo:

```go
position := math2d.Vec2{X: 2, Y: 1}
```

Visualmente:

```text
Y
^
|
|        position {2,1}
|           *
|
+----------------------> X
```

## Vec2.Add

`Add` soma dois vetores componente por componente.

```go
a := math2d.Vec2{X: 2, Y: 1}
b := math2d.Vec2{X: 3, Y: 2}

result := a.Add(b)
// result = {X: 5, Y: 3}
```

Visualmente:

```text
Y
^
|
|                    result {5,3}
|                       *
|                    /
|                 /  b {3,2}
|              /
|     a {2,1}
|        *
|
+------------------------------> X
```

Uso comum:

```go
nextPosition := position.Add(velocity)
```

## Vec2.Sub

`Sub` subtrai dois vetores componente por componente.

O uso mais importante e descobrir o deslocamento de um ponto ate outro.

```go
position := math2d.Vec2{X: 2, Y: 1}
target := math2d.Vec2{X: 6, Y: 4}

direction := target.Sub(position)
// direction = {X: 4, Y: 3}
```

Visualmente:

```text
Y
^
|
|                         target {6,4}
|                            *
|                         /
|                      / direction {4,3}
|                   /
|      position {2,1}
|          *
|
+------------------------------------> X
```

Uso comum:

```go
toBall := ballPosition.Sub(playerPosition)
```

## Vec2.Mul

`Mul` multiplica um vetor por um numero.

```go
direction := math2d.Vec2{X: 1, Y: 0}
speed := 5.0

velocity := direction.Mul(speed)
// velocity = {X: 5, Y: 0}
```

Visualmente:

```text
Antes:

{1,0}
---->

Depois de Mul(5):

{5,0}
-------------------->
```

Uso comum:

```go
movement := velocity.Mul(deltaTime)
```

## Vec2.Div

`Div` divide um vetor por um numero.

```go
v := math2d.Vec2{X: 10, Y: 4}
half := v.Div(2)
// half = {X: 5, Y: 2}
```

Uso comum:

- Reduzir escala.
- Calcular medias.
- Apoiar normalizacao.

## Vec2.Dot

`Dot` calcula o produto escalar.

```go
a := math2d.Vec2{X: 1, Y: 0}
b := math2d.Vec2{X: 1, Y: 0}

dot := a.Dot(b)
// dot = 1
```

Intuicao:

```text
Mesmo sentido:

a ---->
b ---->

Dot alto e positivo.
```

```text
Sentidos opostos:

a ---->
b <----

Dot negativo.
```

```text
Perpendiculares:

a ---->
b
^
|

Dot perto de zero.
```

Uso comum:

- Saber se dois vetores apontam em direcoes parecidas.
- Calcular projecoes.
- Fisica e colisao.
- Campo de visao.

## Vec2.Len e Vec2.LenSq

`Len` retorna o comprimento real do vetor.

```go
v := math2d.Vec2{X: 3, Y: 4}

length := v.Len()
// length = 5
```

Visualmente:

```text
Y
^
|
|        * {3,4}
|       /
|      / length = 5
|     /
|    *
|
+----------------------> X
```

`LenSq` retorna o comprimento ao quadrado.

```go
v.LenSq()
// 25
```

Use `LenSq` quando voce so precisa comparar distancias, porque evita calcular raiz quadrada.

```go
if player.DistanceSq(ball) <= pickupRadius*pickupRadius {
    // esta perto o suficiente
}
```

## Vec2.Normalize

`Normalize` transforma um vetor em outro vetor com o mesmo sentido, mas comprimento `1`.

```go
v := math2d.Vec2{X: 3, Y: 4}

unit := v.Normalize()
// unit = {X: 0.6, Y: 0.8}
```

Visualmente:

```text
Mesmo sentido, tamanhos diferentes:

v {3,4}
-------------------->

unit {0.6,0.8}
---->
```

Uso comum:

```go
direction := target.Sub(position).Normalize()
velocity := direction.Mul(speed)
```

Passos:

```text
1. target.Sub(position) descobre para onde ir.
2. Normalize transforma isso em direcao pura.
3. Mul(speed) aplica a velocidade desejada.
```

Caso especial:

```go
math2d.Vec2{}.Normalize()
// retorna {0,0}
```

Isso evita divisao por zero.

## Vec2.Distance e Vec2.DistanceSq

`Distance` retorna a distancia entre dois pontos.

```go
a := math2d.Vec2{X: 1, Y: 1}
b := math2d.Vec2{X: 4, Y: 5}

distance := a.Distance(b)
// distance = 5
```

Visualmente:

```text
Y
^
|
|             b {4,5}
|                *
|              /
|            / distance = 5
|          /
|   a {1,1}
|      *
|
+--------------------------> X
```

`DistanceSq` retorna a distancia ao quadrado.

Use `DistanceSq` quando for comparar raios:

```go
radius := 5.0

if a.DistanceSq(b) <= radius*radius {
    // dentro do raio
}
```

## Vec2.Lerp

`Lerp` faz interpolacao linear entre dois pontos.

```go
start := math2d.Vec2{X: 0, Y: 0}
end := math2d.Vec2{X: 10, Y: 0}

middle := start.Lerp(end, 0.5)
// middle = {X: 5, Y: 0}
```

Visualmente:

```text
t = 0.0          t = 0.5          t = 1.0
start ---------- middle ---------- end
{0,0}            {5,0}             {10,0}
```

Outros exemplos:

```text
t = 0.25 fica a 25% do caminho.
t = 0.75 fica a 75% do caminho.
```

Uso comum:

- Animacao.
- Interpolacao entre ticks.
- Camera seguindo alvo.
- Replay.
- Movimento visual suave.

## Rect

`Rect` representa um retangulo alinhado aos eixos.

Ele tambem pode ser chamado de AABB: Axis-Aligned Bounding Box.

```go
type Rect struct {
    Min Vec2
    Max Vec2
}
```

Exemplo:

```go
rect := math2d.Rect{
    Min: math2d.Vec2{X: 10, Y: 20},
    Max: math2d.Vec2{X: 40, Y: 60},
}
```

Visualmente:

```text
          X=10                 X=40
            |                    |
Y=60  ------+--------------------+------
            |                    |
            |        Rect        |
            |                    |
Y=20  ------+--------------------+------
            |                    |

Min = {10,20}
Max = {40,60}
```

## Rect.NewRect

`NewRect` cria um retangulo a partir de posicao e tamanho.

```go
rect := math2d.NewRect(
    math2d.Vec2{X: 10, Y: 20},
    math2d.Vec2{X: 30, Y: 40},
)
```

Resultado:

```text
Min = {10,20}
Max = {40,60}
```

Porque:

```text
Max = position + size
Max = {10,20} + {30,40}
Max = {40,60}
```

## Rect.Width, Rect.Height e Rect.Size

`Width` retorna:

```text
Max.X - Min.X
```

`Height` retorna:

```text
Max.Y - Min.Y
```

`Size` retorna os dois como `Vec2`.

Exemplo:

```text
Min = {10,20}
Max = {40,60}

Width  = 30
Height = 40
Size   = {30,40}
```

## Rect.Center

`Center` retorna o ponto central do retangulo.

```text
Min = {10,20}
Max = {40,60}

Center = {25,40}
```

Visualmente:

```text
+--------------------+
|                    |
|         *          |
|      Center        |
|                    |
+--------------------+
```

## Rect.Contains

`Contains` diz se um ponto esta dentro do retangulo.

Regra de borda recomendada para Arasy:

```text
Min inclusivo
Max exclusivo
```

Em notacao matematica:

```text
[Min.X, Max.X)
[Min.Y, Max.Y)
```

O colchete `[` inclui.
O parenteses `)` exclui.

Exemplo:

```text
Rect:
Min = {10,20}
Max = {40,60}
```

Resultados:

```text
Contains({10,20}) = true
Contains({20,30}) = true
Contains({39,59}) = true
Contains({40,60}) = false
Contains({40,30}) = false
Contains({20,60}) = false
```

Visualmente:

```text
X: [10 ................................ 40)
Y: [20 ................................ 60)
```

Essa regra evita ambiguidade em pixels, tiles, botoes e retangulos adjacentes.

Exemplo:

```text
A: [0,10)
B: [10,20)

O ponto X=10 pertence a B, nao aos dois.
```

## Rect.Overlaps

`Overlaps` diz se dois retangulos tem area positiva em comum.

Sobreposicao real:

```text
A
+---------+
|         |
|    B    |
|    +---------+
|    |    |    |
+----|----+    |
     |         |
     +---------+

A.Overlaps(B) = true
```

Apenas encostar a borda nao conta:

```text
A         B
+----+----+----+
|    |    |    |
|    |    |    |
+----+----+----+

A.Overlaps(B) = false
```

Essa regra e util quando voce quer detectar interpenetracao real, nao apenas contato.

## Rect.TouchesOrOverlaps

`TouchesOrOverlaps` diz se dois retangulos se sobrepoem ou apenas encostam.

```text
A         B
+----+----+----+
|    |    |    |
|    |    |    |
+----+----+----+

A.TouchesOrOverlaps(B) = true
```

Use quando contato de borda for relevante:

- Triggers.
- Snap em grid.
- Adjacencia.
- Checagens de contato.

## Resumo das regras de borda

```text
Contains:
  Min entra
  Max nao entra

Overlaps:
  precisa ter area compartilhada
  encostar nao conta

TouchesOrOverlaps:
  area compartilhada conta
  encostar tambem conta
```

## Quando usar cada funcao

```text
Vec2.Add:
  mover posicao por deslocamento

Vec2.Sub:
  criar direcao entre dois pontos

Vec2.Mul:
  aplicar velocidade, escala ou delta time

Vec2.Normalize:
  transformar deslocamento em direcao pura

Vec2.Distance:
  medir distancia real

Vec2.DistanceSq:
  comparar distancias sem raiz quadrada

Vec2.Lerp:
  interpolar movimento, camera ou replay

Rect.Contains:
  mouse dentro de botao, ponto dentro de area

Rect.Overlaps:
  colisao com area real compartilhada

Rect.TouchesOrOverlaps:
  contato ou adjacencia
```

