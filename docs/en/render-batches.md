# Render Batches And Interpolation

## Retained render batches

`render.DrawSpriteBatch` and `render.DrawTextBatch` draw content published once
through `runtime.BatchPort`. The port is shared by `Game.Init`, `Game.Update`,
and `Game.Render`.

Use retained batches for stable or rarely changing content such as crowds,
scenery, and HUD elements. Keep dynamic ECS sprites on the streaming path.

### Publication contract

- `UpdateSprites` and `UpdateText` replace the complete batch.
- `DeleteSprites` and `DeleteText` remove a batch.
- An empty replacement keeps an existing empty batch.
- A command that names a missing batch is skipped and produces one warning.
- A batch command owns one layer and one clip and creates a painter-order
  boundary.
- Retained content is render-only. It does not enter checksums, snapshots,
  recordings, or replay. Rewind does not undo a publication.

### Bounds and animation

The runtime makes defensive copies and computes world bounds when content is
published. Culling is whole-batch and conservative. The renderer inflates the
bounds for animation before it tests the view.

The supported visual animation is deterministic and presentation-only:

- `SpriteAnimation` supports flip-book, phase, and bounce behavior.
- `SpritePulse` supports pulse behavior.
- `SpriteBatchAnimation` controls batch-level animation.

Raster, Canvas2D, and WGSL evaluate these animations through
`internal/renderbatch`. The system does not accept arbitrary shaders, does not
reorder instances, and does not run per-instance culling or compute work.

`SpriteInstance.ProjectCorners` is the retained ground-geometry option. It
projects all four destination corners through `Camera.View`, so a square tile
becomes the same ground-plane shape as a `DrawImageQuad`. Leave it false for
characters, props, and other upright sprites that should remain billboards.

### Host behavior

WebGPU reuses the streaming storage-buffer pipelines. Stable content uploads
zero bytes per frame. Raster is the reference. The development Canvas2D host
caps retained content at 2,000 sprites and 500 labels.

Retained sprites use screen-space billboard corners by default. Projected
sprites use the complete `Camera.View`; text and particles remain billboards.

## Camera passes

`RenderContext.AddPass` adds an extra camera and command stream to the frame.
Renderers draw extra passes in declaration order, then draw the ordinary
commands returned by `Game.Render` with the frame camera. A pass with a zero
viewport uses the full target; an explicit viewport clips that pass to its
screen rectangle. Passes share the frame clear and remain presentation-only:
they do not affect checksums, snapshots, recordings, or replay.

## Render interpolation

`runtime.Config.RenderInterpolation` enables interpolation. It is disabled by
default. `runtime.RenderContext.Alpha` exposes the fraction between the current
and next fixed tick.

`scene.World.SyncRenderState()` captures the previous tick. Games request
interpolated commands with:

```go
ctx.World.RenderCommandsInterpolated(ctx.Alpha)
```

Interpolation applies only to `Position`, `Rotation`, and `Scale`. It uses
`Vec2.Lerp` and the math interpolation helper. Debug overlays such as colliders,
the grid, and velocity show current simulation state.

Interpolation is presentation-only. It does not enter checksums, snapshots,
replays, rewind, or recordings. `SetTransform` resynchronizes the render state
immediately so a teleport or reset does not smear across a frame.

## Camera limits

Immediate shapes, images, and text walk geometry through `WorldToScreen` and
honor the camera view. Ground tiles in retained batches opt into the same
four-corner projection; upright sprites and particles remain billboards. See
[2.5D](2-5d.md) for projection, plane, and depth-order rules.
