# Math2D Visual Guide

This guide explains the basic ideas in the `math2d` package visually.

The goal is not to replace tests or Go documentation, but to build intuition
for `Vec2`, `Rect`, distances, directions, and edge rules.

## Coordinate System

`math2d` does not decide whether the Y axis grows upward or downward. That
depends on the system using the values:

- game world;
- screen;
- canvas;
- normalized field;
- UI.

The examples in this guide use X growing to the right and Y growing upward only
to make the drawings easy to read:

```text
Y
^
|
|
|
+----------------------> X
```

## Vec2

`Vec2` represents a point, direction, velocity, or displacement in 2D.

```go
type Vec2 struct {
    X float64
    Y float64
}
```

Example:

```go
position := math2d.Vec2{X: 2, Y: 1}
```

Visually:

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

`Add` adds two vectors component by component.

```go
a := math2d.Vec2{X: 2, Y: 1}
b := math2d.Vec2{X: 3, Y: 2}

result := a.Add(b)
// result = {X: 5, Y: 3}
```

Common use:

```go
nextPosition := position.Add(velocity)
```

## Vec2.Sub

`Sub` subtracts two vectors component by component.

The most important use is finding the displacement from one point to another.

```go
position := math2d.Vec2{X: 2, Y: 1}
target := math2d.Vec2{X: 6, Y: 4}

direction := target.Sub(position)
// direction = {X: 4, Y: 3}
```

Common use:

```go
toBall := ballPosition.Sub(playerPosition)
```

## Vec2.Mul

`Mul` multiplies a vector by a number.

```go
direction := math2d.Vec2{X: 1, Y: 0}
speed := 5.0

velocity := direction.Mul(speed)
// velocity = {X: 5, Y: 0}
```

Common use:

```go
movement := velocity.Mul(deltaTime)
```

## Vec2.Div

`Div` divides a vector by a number.

```go
v := math2d.Vec2{X: 10, Y: 4}
half := v.Div(2)
// half = {X: 5, Y: 2}
```

Common uses:

- reducing scale;
- calculating averages;
- supporting normalization.

## Vec2.Dot

`Dot` calculates the dot product.

```go
a := math2d.Vec2{X: 1, Y: 0}
b := math2d.Vec2{X: 1, Y: 0}

dot := a.Dot(b)
// dot = 1
```

Intuition:

```text
Same direction:

a ---->
b ---->

Dot is high and positive.
```

```text
Opposite directions:

a ---->
b <----

Dot is negative.
```

```text
Perpendicular:

a ---->
b
^
|

Dot is close to zero.
```

Common uses:

- checking whether two vectors point in similar directions;
- calculating projections;
- physics and collision;
- fields of view.

## Vec2.Len And Vec2.LenSq

`Len` returns the real vector length.

```go
v := math2d.Vec2{X: 3, Y: 4}

length := v.Len()
// length = 5
```

`LenSq` returns squared length.

```go
v.LenSq()
// 25
```

Use `LenSq` when you only need to compare distances, because it avoids a square
root.

```go
if player.DistanceSq(ball) <= pickupRadius*pickupRadius {
    // close enough
}
```

## Vec2.Normalize

`Normalize` turns a vector into another vector with the same direction and
length `1`.

```go
v := math2d.Vec2{X: 3, Y: 4}

unit := v.Normalize()
// unit = {X: 0.6, Y: 0.8}
```

Common use:

```go
direction := target.Sub(position).Normalize()
velocity := direction.Mul(speed)
```

Steps:

```text
1. target.Sub(position) finds where to go.
2. Normalize turns that into pure direction.
3. Mul(speed) applies the desired speed.
```

Special case:

```go
math2d.Vec2{}.Normalize()
// returns {0,0}
```

This avoids division by zero.

## Vec2.Distance And Vec2.DistanceSq

`Distance` returns the distance between two points.

```go
a := math2d.Vec2{X: 1, Y: 1}
b := math2d.Vec2{X: 4, Y: 5}

distance := a.Distance(b)
// distance = 5
```

`DistanceSq` returns squared distance.

Use `DistanceSq` when comparing against radii:

```go
radius := 5.0

if a.DistanceSq(b) <= radius*radius {
    // inside the radius
}
```

## Vec2.Lerp

`Lerp` linearly interpolates between two points.

```go
start := math2d.Vec2{X: 0, Y: 0}
end := math2d.Vec2{X: 10, Y: 0}

middle := start.Lerp(end, 0.5)
// middle = {X: 5, Y: 0}
```

Visually:

```text
t = 0.0          t = 0.5          t = 1.0
start ---------- middle ---------- end
{0,0}            {5,0}             {10,0}
```

Other examples:

```text
t = 0.25 is 25% of the way.
t = 0.75 is 75% of the way.
```

Common uses:

- animation;
- interpolation between ticks;
- camera following a target;
- replay;
- smooth visual movement.

## Rect

`Rect` represents an axis-aligned rectangle.

It can also be called an AABB: Axis-Aligned Bounding Box.

```go
type Rect struct {
    Min Vec2
    Max Vec2
}
```

Example:

```go
rect := math2d.Rect{
    Min: math2d.Vec2{X: 10, Y: 20},
    Max: math2d.Vec2{X: 40, Y: 60},
}
```

## Rect.NewRect

`NewRect` creates a rectangle from position and size.

```go
rect := math2d.NewRect(
    math2d.Vec2{X: 10, Y: 20},
    math2d.Vec2{X: 30, Y: 40},
)
```

Result:

```text
Min = {10,20}
Max = {40,60}
```

Because:

```text
Max = position + size
Max = {10,20} + {30,40}
Max = {40,60}
```

## Rect.Width, Rect.Height, And Rect.Size

`Width` returns:

```text
Max.X - Min.X
```

`Height` returns:

```text
Max.Y - Min.Y
```

`Size` returns both as a `Vec2`.

Example:

```text
Min = {10,20}
Max = {40,60}

Width  = 30
Height = 40
Size   = {30,40}
```

## Rect.Center

`Center` returns the rectangle center point.

```text
Min = {10,20}
Max = {40,60}

Center = {25,40}
```

## Rect.Contains

`Contains` reports whether a point is inside the rectangle.

Recommended edge rule for Arasy:

```text
Min inclusive
Max exclusive
```

In mathematical notation:

```text
[Min.X, Max.X)
[Min.Y, Max.Y)
```

The bracket `[` includes.
The parenthesis `)` excludes.

Example:

```text
Rect:
Min = {10,20}
Max = {40,60}
```

Results:

```text
Contains({10,20}) = true
Contains({20,30}) = true
Contains({39,59}) = true
Contains({40,60}) = false
Contains({40,30}) = false
Contains({20,60}) = false
```

This rule avoids ambiguity in pixels, tiles, buttons, and adjacent rectangles.

Example:

```text
A: [0,10)
B: [10,20)

Point X=10 belongs to B, not both.
```

## Rect.Overlaps

`Overlaps` reports whether two rectangles have positive shared area.

Real overlap:

```text
A.Overlaps(B) = true
```

Only touching edges does not count:

```text
A.Overlaps(B) = false
```

This is useful when you want to detect real interpenetration, not just contact.

## Rect.TouchesOrOverlaps

`TouchesOrOverlaps` reports whether two rectangles overlap or merely touch.

Use it when edge contact matters:

- triggers;
- grid snapping;
- adjacency;
- contact checks.

## Edge Rule Summary

```text
Contains:
  Min is included
  Max is not included

Overlaps:
  shared area is required
  touching does not count

TouchesOrOverlaps:
  shared area counts
  touching also counts
```

## When To Use Each Function

```text
Vec2.Add:
  move a position by displacement

Vec2.Sub:
  create a direction between two points

Vec2.Mul:
  apply speed, scale, or delta time

Vec2.Normalize:
  turn displacement into pure direction

Vec2.Distance:
  measure real distance

Vec2.DistanceSq:
  compare distances without square root

Vec2.Lerp:
  interpolate movement, camera, or replay

Rect.Contains:
  mouse inside a button, point inside an area

Rect.Overlaps:
  collision with real shared area

Rect.TouchesOrOverlaps:
  contact or adjacency
```
