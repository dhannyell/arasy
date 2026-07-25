# Camera, World, And Viewport

This guide explains the difference between `World` and `Viewport` in
Arasy's camera.

These concepts matter because the engine should not mix game coordinates with
screen pixels.

## Main Idea

```text
World:
  game coordinates

Viewport:
  image/screen coordinates where the game is drawn
```

Example:

```text
World:
  football field measuring 100 x 75 units

Viewport:
  PNG image measuring 800 x 600 pixels
```

The camera maps one to the other:

```text
World 100 x 75  ->  Viewport 800 x 600
```

That means:

```text
1 world unit = 8 pixels
```

Because:

```text
800 / 100 = 8
600 / 75  = 8
```

## Why Separate World And Viewport?

Without this separation, the simulation would need to know the screen size.

That would be bad:

```go
playerPosition := Vec2{X: 640, Y: 360}
```

What does that value mean?

- Screen pixel?
- Field position?
- World position?
- Canvas position?

With `World`, the simulation uses game coordinates:

```go
playerPosition := Vec2{X: 50, Y: 37.5}
```

That means:

```text
middle of the field
```

Then the camera turns it into pixels:

```text
{50, 37.5} in world -> {400, 300} on screen
```

The same game can be drawn at different resolutions:

```text
World: {50,37.5}

Viewport 800 x 600:
  screen = {400,300}

Viewport 1600 x 1200:
  screen = {800,600}
```

The game position stays the same.

## World

`World` represents the game area the camera sees.

In Arasy:

```go
World: math2d.NewRect(
    math2d.Vec2{X: 0, Y: 0},
    math2d.Vec2{X: 100, Y: 75},
)
```

This means:

```text
Min = {0,0}
Max = {100,75}
```

Everything in the simulation uses these coordinates:

```text
player: {30,35}
ball:   {52,36}
goal:   {100,37.5}
```

## Viewport

`Viewport` represents the image/screen area where the world is drawn.

In Arasy:

```go
Viewport: math2d.NewRect(
    math2d.Vec2{X: 0, Y: 0},
    math2d.Vec2{X: 800, Y: 600},
)
```

This means:

```text
Min = {0,0}
Max = {800,600}
```

In the raster renderer, these values are image pixels.

On canvas, they are backing pixels for the drawing surface.

## Camera

The camera combines `World` and `Viewport`.

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

It answers:

```text
where does this game point land on the screen?
```

## WorldToScreen

`WorldToScreen` converts a world position to a screen position.

Example:

```go
screen := camera.WorldToScreen(math2d.Vec2{X: 50, Y: 37.5})
```

Result:

```text
{400,300}
```

Simplified formula:

```text
normalizedX = (point.X - World.Min.X) / World.Width
screenX     = Viewport.Min.X + normalizedX * Viewport.Width

normalizedY = (point.Y - World.Min.Y) / World.Height
screenY     = Viewport.Min.Y + normalizedY * Viewport.Height
```

Numerical example:

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

`ScreenToWorld` does the inverse.

It converts a screen position to a world position.

This is useful for input.

Example:

```go
world := camera.ScreenToWorld(math2d.Vec2{X: 400, Y: 300})
```

Result:

```text
{50,37.5}
```

Use case:

```text
user clicks pixel {400,300}
camera converts it to world {50,37.5}
game understands that the click was in the middle of the field
```

## Viewport With Offset

The viewport does not need to start at `{0,0}`.

Imagine a full screen of 1280 x 720, with the field inside a smaller area:

```text
full screen:
  1280 x 720

field viewport:
  x = 100
  y = 80
  width = 1000
  height = 560
```

In code:

```go
Viewport: math2d.NewRect(
    math2d.Vec2{X: 100, Y: 80},
    math2d.Vec2{X: 1000, Y: 560},
)
```

This allows drawing the game inside UI panels.

## Scale

Scale says how many pixels correspond to one world unit.

Example:

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

So:

```text
1 world unit = 8 pixels
```

If a player has radius `2.5` in world units:

```text
2.5 * 8 = 20 pixels
```

If a line has width `0.35` in world units:

```text
0.35 * 8 = 2.8 pixels
```

## worldWidthToScreen

This helper is not a required public API, but may exist in the raster renderer
for converting measurements.

Example:

```go
func (r Renderer) worldWidthToScreen(width float64) float64 {
    worldSize := r.Camera.World.Size()
    viewportSize := r.Camera.Viewport.Size()

    return width * (viewportSize.X / worldSize.X)
}
```

It converts:

```text
width in world -> width in pixels
```

Usage:

```go
radius := r.worldWidthToScreen(command.Radius)
lineWidth := r.worldWidthToScreen(command.Width)
strokeWidth := r.worldWidthToScreen(command.StrokeWidth)
```

## Aspect Ratio

The ideal example has the same scale on X and Y:

```text
World:    100 x 75
Viewport: 800 x 600

Scale X = 8
Scale Y = 8
```

That is not always true.

Example:

```text
World:    100 x 100
Viewport: 800 x 600

Scale X = 8
Scale Y = 6
```

With direct mapping:

```text
circles may look like ellipses
the world may look stretched
```

For V1, the simple solution is to choose `World` and `Viewport` with the same
proportion.

Good example:

```text
World:    100 x 75
Viewport: 800 x 600
Aspect:   4:3 in both
```

The camera also supports aspect behavior through `render.AspectMode`, such as
expand or fit modes.

## Y Axis

This guide draws Y upward, but images usually use Y growing downward:

```text
image:

{0,0} is the top-left corner
Y grows downward
```

Arasy's camera does not flip the Y axis by itself.

It assumes `World` and `Viewport` use the same orientation.

That is enough for the current engine.

Later, the engine may add:

```go
FlipY bool
```

or establish a clear convention:

```text
game world uses Y downward
```

## Summary

```text
World:
  game coordinates
  example: field 100 x 75

Viewport:
  visual output coordinates
  example: PNG 800 x 600

Camera:
  converts between World and Viewport

WorldToScreen:
  game -> screen

ScreenToWorld:
  screen -> game
```

The separation lets simulation stay independent from resolution, renderer, and
platform.
