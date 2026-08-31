# Architecture

## Layers

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

## Dependency rules

- `math2d`, `render`, `input`, `ui`, `scene`, and `runtime` do not import
  `platform/*`.
- `runtime` is the only behavioral API used by platforms and games.
- Platforms call `runtime.Driver`; they do not drive `scene.World` directly.
- `runtime` does not depend on JSON, HTTP, images, or browser concepts.
- `asset` parses manifests. Runtime carries the resolved `asset.Manifest`.
- Games implement `runtime.Game` and receive the world through a context.

## Package responsibilities

### `math2d`

`math2d` contains pure 2D math such as `Vec2` and `Rect`. It has no engine or
platform dependency.

### `fixed` (external module)

`github.com/dhannyell/fixed` (future `arasy-engine/fixed`) provides the
deterministic Q32.32 type of the simulation — `Q`, `Vec2`, `Rot`, and
trigonometry in turns; the engine consumes it through `dmath`.

### `render`

`render` defines backend-agnostic commands, colors, text measurement, cameras,
and `CommandSink`. `DrawImageQuad` maps a texture to a convex quad. Shapes,
images, and immediate text use the camera's world-to-screen transform.

`BaseCommand.Transform` is a per-command affine transform. Its identity is the
zero value. It is decorative and does not affect hit testing. `DrawParticles`,
`DrawSpriteBatch`, and `DrawTextBatch` have backend-specific retained or emitter
state and do not support every camera transform.

`MSAAMode` is the shared antialiasing option for GPU hosts. Raster and Canvas2D
ignore it. Raster is the reference for geometry and color behavior.

The 2.5D vocabulary lives here too: `Camera.DepthScale` and `Camera.View`
declare the projection, `GroundProjection` derives the height presentation
from them, `FrameGround` frames a ground rectangle under a view, and
`Projection.Wall`/`ExtrudeRect`/`LiftVector` author fake volume along the
same lift the entities use. `BaseCommand.BlendMode` selects the compositing
operator (`BlendAdditive` for light-accumulating glows).

See [2.5D](2-5d.md) and [render batches](render-batches.md).

### `input`

`input` maps physical buttons, axes, pointer events, keyboard events, and typed
text to game actions. The driver ends each input frame after the simulation
step. Text events carry Unicode values and do not add bytes to recordings.
`PointerFrame.Clicks` answers the common "did the player click, and where".
`Repeater` (held-key cadence) and `StickDPad` (analog stick as a four-way pad
with hysteresis) are dt-driven helpers with exported state, built to live
inside snapshotted game state so replays carry them tick for tick.

### `ui`

`ui` provides immediate-mode widgets and three coordinate spaces. The caller
owns widget values. Widget-internal state belongs to `ui.State` and is captured
by the driver for rewind. Popups use the overlay layer.

### `asset`

`asset` loads layered `fs.FS` manifests. The last layer wins. Texture and font
IDs are stable for one run and are resolved by name. They are not persisted.
Image decoding belongs to `asset/pixel`. SFNT and OpenType parsing belongs to
`asset/typeface`.

### `audio`

`audio` defines clip IDs, one-shot cues, continuous tracks, and rewindable track
state. Hosts decode and play audio. Runtime transports audio intent.

### `particle`

`particle` defines render-only effects and the deterministic CPU reference. It
uses fixed emitter pools, spawn hashing, active-slot bitsets, emission
accounting, and adaptive fallback budgets. WebGPU mirrors these rules in
compute shaders.

### `scene`

`scene` owns the ECS world and simulation step. The step pipeline is locomotion,
acceleration, ballistics, integration, bounce, broad phase, contact resolution,
and damping.

The world supports altitude, drag, Magnus force, bounce, rolling, spin decay,
substeps, steering, sprite depth order, and render-only 2.5D presentation.
Depth order uses `(Layer, depth key, EntityID)`, with the key resolved once per
sprite per frame: the sprite's own base projected onto `World.SortAxis`, moved
by `SortOffset`. The final `EntityID` tie-break makes the frame reproducible.
Height affects presentation but not the depth key. `CommandSprite` lets a
raw-command prop join that sort, `Attachment` anchors a sprite to a parent so
a composite sorts as one thing, and `World.Hop` flies an entity in an arc as
simulation state.

Simulation components enter `World.Checksum()`. Presentation values such as
render height, sprite transforms, projection, and sprite plane do not.

### `runtime`

`runtime` owns `Game`, `Driver`, fixed timestep execution, development controls,
rewind, frame timing, frame statistics, garbage-collection statistics,
`StateSlot[T]`, audio consumption, and retained batch publication.

Runtime defines what a tick means. Hosts must not duplicate tick logic. The
runtime also carries resolved assets and exposes render contexts.

### `internal/devpanel`

The development panel is a portable UI overlay. It is compiled only with the
`arr_dev` tag. It has its own `ui.State`; panel state does not enter game
snapshots or checksums. Captured panel input does not reach the game or a
recording. Browser and native hosts provide the `RecordingPort` seam.

The panel provides simulation controls, timeline scrubbing, recording and
watching, performance telemetry, state inspection, entity picking, scalar and
boolean editing, and debug layers. It emits commands above game layers. A
closed panel emits no commands. The panel keeps the pointer while it is hovered
or a widget is active, and keeps the keyboard while a widget has focus.

### `internal/renderbatch`

`internal/renderbatch` owns defensive copies, world bounds, animation
evaluation, culling inflation, and view intersection for retained batches.
Raster, Canvas2D, and WGSL use the same rules.

### `anim`, `render/vector`, and `internal/svggen`

`anim` provides stateless presentation timelines. `render/vector` provides
immutable cubic and quadratic path geometry. `internal/svggen` compiles the
supported animated SVG/CSS subset into deterministic Go render code.

### GPU packages

`internal/render/gpu/tess` builds ordered GPU pipeline streams. The WASM WebGPU
host uses it, and the native renderer shares the same core. `fontatlas` provides
manifest-backed glyph atlases. `shaders` contains embedded WGSL sources.
`internal/render/frameplan` normalizes pass and target policy for every backend;
hosts only activate resources and encode the resulting passes.

### Platform hosts

- `platform/raster` renders to `image.RGBA` and is the offline semantic
  reference.
- `platform/wasm` drives `requestAnimationFrame`, uses WebGPU by default, and
  falls back to Canvas2D. WebGPU uploads manifest textures and glyph atlases
  for each `(FontID, pixelSize)`, builds mip chains, uses anisotropic sampling,
  and runs particles with compute and indirect draws. Canvas2D approximates
  quads and particles on the CPU, uses browser `FontFace` for external fonts,
  and caps retained batches at 2,000 sprites and 500 labels.
- `platform/wasm` uses `Options.MSAA`, `Options.DevTools`, and
  `Options.RecordReplay`. Dev tools require `arr_dev`; browser recording uses
  a Blob and a file picker.
- `platform/native` uses GLFW and wgpu-native. It locks the OS thread, queues
  callbacks, applies input through `Driver.RunTick`, and supports native audio.
  `Options.GPUTiming` requests timestamp queries at device creation. Readback is
  asynchronous, so the reported sample lags its frame. Particle frames use
  emit, simulate, and render intervals. Frames without particles skip compute.
- Native dev tools support fullscreen, recording, system probes, frame metrics,
  GPU pass timing, and GC telemetry. The renderer can use an offscreen particle
  path and falls back to direct rendering when optional offscreen setup fails.
- Device creation requests adapter limits and falls back to driver defaults on
  error. The native host uses the configured wgpu-native fork and a
  process-global `oto` sink for manifest audio.

Both GPU hosts share tessellation, shaders, particles, MSAA, and frame metrics.
Their detailed lifecycle and validation rules belong in host documentation.

## Design constraints

- Keep engine packages independent of app-specific rules.
- Prefer small explicit APIs.
- Add complexity only under real project pressure.
- Keep render commands backend-agnostic.
- Measure before optimizing.
- Prefer the standard library. Existing third-party engine dependencies are
  limited to the documented packages in `go.mod`.

## Related documents

- [Runtime](runtime.md)
- [Testing](testing.md)
- [Development commands](development.md)
- [2.5D projection](2-5d.md)
- [Render batches and interpolation](render-batches.md)
- [Boot splash](boot-splash.md)
