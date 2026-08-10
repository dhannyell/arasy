# 2.5D: current state and points of improvement

A survey of where the engine stands for 2.5D games in general — not only the
isometric `examples/outpost`. Written in 2026-08; revisit whenever any item
becomes a feature.

## What is already solved

The principle organizing 2.5D here is that the projection is **declared, and
everything else derives** — tilt, height, shadow and ordering are not the
game's problem:

- `Camera.DepthScale` and `Camera.View` — the projection lives on the camera,
  so picking (`ScreenToWorld`), debug overlays and physics stay on the square
  grid.
- `Height` + `Ballistics` + `GroundProjection` — height is simulation state
  and the shadow that detaches from the body comes for free.
- `PlaneUpright`/`PlaneGround` with automatic correction — the engine works
  out the shape an upright sprite needs under the view, with no hand-written
  shear correction in the game.
- Base-sort with `SortAxis`/`SortOffset`, `CommandSprite` (raw-command props
  in the painter sort), `Attachment` (composites that sort as one thing) and
  `Projection.Wall`/`ExtrudeRect` (extruded volume on the same lift as the
  entities).

## The gaps, in impact order

### 1. Batched textures — closed 2026-08-09

`DrawSpriteBatch`, `DrawTextBatch` and `DrawParticles` now carry the complete
camera matrix, including `View` and `DepthScale`, in both GPU backends. Their
positions therefore follow `Camera.WorldToScreen`. Sprite instances remain
screen-space billboards by default; ground tiles set `ProjectCorners` so all
four destination corners follow the same camera projection. Batch culling
transforms bounds conservatively through that `View`. The remaining exception
is `DrawParticles.BaseCommand.Transform`, which is deliberately outside the
camera contract.

### 2. There is no Tiled import

The engine has a retained grid-and-atlas tilemap path, but it does not yet
import map editor formats such as Tiled or LDtk. For 2.5D this still weighs
double, because hand-maintaining the grid is content work. Plan the import
around the existing square world grid, atlas layers, and `ProjectCorners`
ground path.

### 3. Height does not enter picking

`ScreenToWorld` inverts the view on the ground plane (z=0). Clicking a lifted
body — a unit at the top of its hop, a prop on raised terrain — hits the tile
*behind* it, because the lift moved the body on screen. What is missing is
the inverse of the lift: a `Projection.ScreenToGround(cam, screen, z)` that
answers "which ground point is under this pixel, assuming height z". Today
every game would subtract `LiftVector().Mul(z)` by hand.

### 4. Height does not enter collision

Colliders are 2D: a projectile with `Height.Z = 10` still collides with a
wall it visibly flies over. The missing component is small — a height on the
collider (`Tall`/`ZExtent`) plus the test "only collides if the Z intervals
overlap" — and it unlocks throws over obstacles, bridges, and the arcing shot
of any isometric action game.

### 5. Terrain has no elevation

`Height` is per entity; the ground is flat. Tactics games with platforms need
per-tile elevation: the drawing already works with `ExtrudeRect`, but
projectile landing (`Ballistics` assumes ground at z=0), the shadow (always
projected at z=0) and picking do not know the ground rose. A `GroundHeight`
queryable by the projection and by ballistics would cover all three.

### 6. Occlusion without a gameplay answer

A character behind a building vanishes — and the tactics-genre standard is a
silhouette or a cutout. The engine has no "this sprite is occluded by that
one". A cheap query (compare depth key + screen-footprint overlap) would let
the game draw the silhouette; the per-pixel cutout is another problem (depth
buffer) and can stay out.

### 7. 2.5D styles still without a path

- **Layer parallax** (side-view 2.5D): one camera only; missing a parallax
  factor per layer or cameras per command group. The engine that declares
  projection could declare this too.
- **Perspective ground (Mode-7 style)**: `View` is affine; perspective wants
  a homography. `DrawImageQuad` already interpolates with perspective
  correction, so the material exists — the camera does not know how to use
  it yet.
- **Billboard in a raycaster** is reasonably out of scope, and that is fine.

### 8. The shadow is a global blob, and only exists airborne

**Contact and style closed**: `scene.ShadowStyle` is the
render-only opt-in for the contact shadow at `Z == 0` and the per-entity
adjustment — `Scale`, `Alpha`, `Offset`, `Disable` — over the projection's
emission. `examples/outpost` consumes it. What remains open:

- **No projected silhouette**: the skewed, darkened copy of the sprite itself
  on the ground (the standard for textured art under directional light) is
  technically a `DrawImage` with a skew transform derived from the
  `LiftVector` + black tint — the material exists, the path does not.
  Trigger: the first textured-art demo under strong directional light.

The blob is the genre's right default for compact bodies; with style and
contact in place, the remaining gap is silhouette only.

### 9. There is no light channel over sprites

**Closed in v1**: `scene.AmbientLight` and `scene.PointLight` are the
declared render-only lights. `World.SetLitLayers` selects the layers they
affect. For each affected sprite, the renderer evaluates the ambient and
point-light contributions at the interpolated ground position, clamps each
RGB channel to `[0, 1]`, and multiplies the result by the authored
`Tint`/`Color`. This happens before commands are emitted, so it needs no new
render command or backend code, and lights never enter a checksum.
Authored `Tint`/`Color` remains simulation state, and lighting never writes
to it. `examples/outpost` consumes the channel with dusk ambient light and a
plaza lantern; the lighting section of `2-5d.md` documents the design. Out by
decision:

- **Normal maps: not supported.** Per-pixel lighting needs a new shader in
  every backend — Canvas2D has none to program — plus a second authored
  texture per sprite. Color modulation is the declared substitute at this
  tier.
- Lighting for retained batches and `DrawParticles` (the likely shape is a
  per-batch uniform; the trigger is the first lit tilemap scene), and
  colored shadows (contact shadows remain black).

## Recommended priority reading

Item 1 is closed; item 2 is now the next practical blocker, and item 3 is what
every second 2.5D game will ask for; 4 and 5 turn 2.5D from visual into mechanics;
6 and 7 are genre polish. 8 and 9 are what separates "sprite placed in the
scene" from "sprite belonging to the scene", and both now have v1 solutions:
styled contact shadows and declared light modulation. Projected silhouettes
and per-pixel lighting remain deliberate next steps. Item 2 gets much cheaper
after item 1 (an atlas tilemap is a retained
batch under the `View`).
