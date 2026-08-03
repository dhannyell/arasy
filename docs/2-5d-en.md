# 2.5D: Tilt, Planes, Height And Depth Order

**The default is 2D, and everything on this page is optional.** A `Camera{}`
looks straight down, a `BaseSprite{}` behaves exactly as it always has, and a
game that never mentions the fields below pays nothing for them existing —
not in API to learn, not in pixels, not in cost per frame. Read on only when
you want a scene to read as a tilted world rather than a flat one.

## The one number: `Camera.DepthScale`

2.5D needs a single declared quantity: how far the camera is tilted away from
straight-down. The camera says it as a **foreshortening** of the world's depth
axis.

```go
cam := render.Camera{
    World:      math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 160, Y: 90}),
    Viewport:   math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 160, Y: 90}),
    Aspect:     render.AspectFit,
    DepthScale: 0.5,
}
```

`DepthScale` is how much of its depth the ground keeps on screen:

| Value | Meaning |
|---|---|
| `1` (and the zero value) | No tilt. A top-down camera. |
| `0.5` | The classic 2:1 view of pixel-art dimetric projection. |
| `0.25` | A shallow, almost side-on look. |

It is the cosine of the tilt from vertical, so `0.5` is a 60 degree tilt. The
API talks about foreshortening rather than degrees because that is how the
result is usually described: people reach for "2:1", not "60 degrees". Values
outside `(0, 1]` are clamped.

The zero value means `1` rather than "flat as a line", so a `Camera{}` written
before this field existed keeps meaning exactly what it meant.

The foreshortening happens around the centre of the visible world, so changing
it never drags the scene out of frame.

## What foreshortens, and what does not

A tilted camera does two different things, and telling them apart is the whole
idea:

- **Positions** foreshorten. Ten units to the north take up less screen than
  ten units to the east, because north is the direction the camera is looking
  along.
- **Shapes** foreshorten only if they lie in the ground plane. A character
  standing up faces the camera and keeps its full height; a marking painted on
  the floor squashes with the floor.

A sprite says which of the two it is:

```go
world.SetRectSprite(id, scene.RectSprite{
    Rect:       courtBounds,
    BaseSprite: scene.BaseSprite{Plane: scene.PlaneGround}, // a painted court
})

world.SetCircleSprite(id, scene.CircleSprite{
    Radius:     4,
    BaseSprite: scene.BaseSprite{}, // PlaneUpright: a ball, facing the camera
})
```

`PlaneUpright` is the zero value, so no existing sprite changes and a standing
character is the default. `PlaneGround` is for decals, puddles, shadows,
painted markings — anything that lives on the floor.

Both sprites move with the foreshortened ground either way: an upright sprite
that ignored the tilt entirely would drift away from the ground it stands on.

Under an untilted camera the two planes are identical and the field costs
nothing to ignore.

## Height and its shadow

Altitude is a component, and it is simulation state like any other:

```go
world.SpawnBundle(scene.Bundle{
    CircleSprite: &scene.CircleSprite{Radius: 4},
    Height:       &scene.Height{Z: 12},
    Ballistics:   &scene.Ballistics{Gravity: 90, BounceDamping: 0.6, RestVZ: 2},
})
```

How `Height.Z` turns into pixels is `render.Projection`, and the way to build
one is to derive it from the camera and the scene's light:

```go
world.SetProjection(render.GroundProjection(cam, render.ProjectionLight{
    Direction: math2d.Vec2{X: -1, Y: -1}, // where the light comes from
    Tilt:      0.18,                      // 0 overhead, 1 grazes the horizon
    Softness:  0.5,                       // 0 a point source, 1 diffuse
}))
```

That one call fills in everything: how far a unit of height lifts the body up
the screen, how large the ground shadow is, how it shrinks and fades as the
object rises, and which way it leans away from the light.

The lift comes out of the tilt as `tan(acos(DepthScale))`. **With
`DepthScale = 1` the lift is zero**, which looks like a bug and is not: a
camera looking straight down has nowhere on screen to put height. A game that
wants visible height wants a tilted camera.

`Projection`'s fields stay public, so a game with its own art direction can
override one of them — `ShadowAlpha`, say — without giving up the derivation
for the rest.

## Depth order: who is drawn behind whom

Sprites are painted back to front. Within a layer, each sprite gets one number
— its **sorting point** projected onto the world's **sort axis** — and lower
draws first, which is to say further away.

### The sorting point is the sprite's base

A sprite sorts by its base, not its centre. That is the difference between a
scene that reads correctly and the single most common y-sort bug: a tall tree
and a short bush planted on the same ground line have centres at very
different depths and bases at the same one, so sorting by centre draws the
tree behind the bush standing next to it.

Each sprite type finds its own base — the bottom of a circle's radius, of a
rect, of an image's size — so mixing types does not reintroduce the problem.
`SortOffset` moves the point from there:

```go
world.SetImageSprite(id, scene.ImageSprite{
    Size:       math2d.Vec2{X: 32, Y: 64},
    SortOffset: math2d.Vec2{Y: -8}, // art whose feet sit above the bottom edge
})
```

`Transform.Scale` deliberately does not enter the base: a sprite that pops or
breathes would otherwise reorder itself in the middle of its own animation.

### The axis is something the world declares

```go
world.SetSortAxis(math2d.Vec2{X: 0, Y: 1}) // plain y-sort, and the default
```

The zero value is `{0, 1}`: further down the screen draws in front, which is
right for a camera looking straight down and is what the engine did before the
axis was something you could say out loud. A tilted camera wants depth weighted
against height; a game that stacks along X wants `{X: 1}`. The vector is a
direction, so its length is ignored and negating it reverses depth.

### Height stays out of it

An entity in the air still sorts by where it stands on the ground. That is
where it is in the world, it is where its shadow is, and it is what lets a ball
pass behind a post while flying past it. Sorting by the lifted position would
snap the ball in front of the post halfway through the arc.

A game that wants the opposite composes it by hand with `Layer`.

### Ties, and why they are ordinary

The key is a `float64`, so two sprites landing on the same value is common
rather than rare — every sprite standing on the same ground line ties. `EntityID`
breaks the tie, which is what makes a frame reproducible: the same scene always
paints the same way, on any host, in any replay.

`SortAxis` and `SortOffset` are presentation. Neither enters `Checksum()`,
snapshots or replay.

### What sorting by a single number cannot do

Reducing a sprite to one scalar has three structural limits. They are written
here so you meet them on this page rather than in a shipped game:

1. **Interpenetration.** Ordering is per command and therefore all-or-nothing:
   a sprite is entirely in front or entirely behind. A character standing *in*
   a doorway, or half behind a fence, has no correct order, because the right
   answer differs per pixel. Only a per-pixel depth buffer resolves this.
2. **Long objects.** A wall ten units deep gets one key, but things at
   different depths along it should order differently against it.
3. **Cycles.** A over B, B over C, C over A. No total order satisfies that.

For the common case — compact sprites (characters, balls, trees, crates) on a
floor — sorting by the base is correct and complete. For the other two, the
standard fix is authoring rather than engine: **slice the long object into
pieces that sort separately**, which is what every 2D game does with walls,
bridges and doorways. `SortOffset` is what makes that cheap, because each piece
can claim its own depth without its sprite having to move.

## What never foreshortens

Foreshortening is a fact about the camera, not about the world. It stays out of:

- **Collision and physics.** Colliders, bounds, bounce and `Ballistics` all
  work in unforeshortened world units. A tilted camera never changes where two
  circles touch.
- **`World.Checksum()`, snapshots, recordings and replay.** `DepthScale`,
  `Plane` and `Projection` are presentation. `Height.Z` *is* simulation state
  and is hashed like everything else — what the projection decides is only how
  it becomes a pixel.
- **Scalar conversions.** `WorldScalarToScreen` is for quantities with no axis
  — stroke widths, corner radii — and ignores the tilt. Radii do have an axis
  and go through `WorldRadiiToScreen`, which is why a circle drawn on the
  ground reaches the screen as an ellipse.

Hit-testing keeps working because `Camera.ScreenToWorld` is the exact inverse
of `WorldToScreen` under any tilt. This matters beyond clicks: recordings
store pointer coordinates, so an asymmetric pair would break replay.

## The three choices are independent

Tilt, sprite plane and height do not imply one another, and no combination is
invalid:

- Blob shadows with no tilt at all: set the `Projection` fields yourself and
  leave `DepthScale` alone.
- A tilted world that never uses `Height`: set `DepthScale` and nothing else.
- A ground decal in an otherwise flat game: `PlaneGround` with no tilt is
  simply a no-op.

## Worked example

`examples/groundplane` is the smallest complete scene: a court painted on the
ground, two upright posts, and a ball with height bouncing around. All of the
2.5D in it is `DepthScale` in `Setup` and `GroundProjection` in `Init`.

`examples/dodgeball` is the same foundation used by a full game.

## Known limits

- **The shadow is a blob**, and can only be one: altitude is per entity, not
  per pixel. A shadow shaped like the object would need a height value for
  every pixel of the sprite.
- **Shadows do not sort among themselves.** Each shadow is emitted next to the
  body it belongs to so that the depth sort keeps an entity's commands
  together, which means one entity's shadow can fall over a neighbour already
  drawn in the same layer.
- **A very low light stretches the shadow into a sliver.** The lean grows with
  height without bound; keep `ProjectionLight.Tilt` modest, or set
  `ShadowSkewPerZ` yourself.
- **Sprites cannot interpenetrate.** A sprite is drawn entirely in front of or
  entirely behind another. Only a per-pixel depth buffer would change that.
