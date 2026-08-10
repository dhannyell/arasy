# Vector Graphics

This guide covers Arasy's vector geometry API (`render/vector`) and the
`arasy svg` compiler (`internal/svggen`) that turns an authored, animated SVG
into deterministic Go using that API. See
[`docs/en/animation.md`](animation.md) for the `anim.Timeline`/
`render.Builder` layer this content is built on.

## `render/vector`: paths, stroke, and motion

`vector.Path` is immutable path geometry: a caller builds one with
`vector.Builder` (`MoveTo`, `LineTo`, `QuadTo`, `CubicTo`, `Close`,
`Build()`), which validates finiteness and open-subpath invariants before
returning a `Path`. Internally, a path stores only move/line/quad/cubic/close
segments — **there is no native arc verb**; anything arc-shaped is converted
to cubic Béziers before it ever reaches this package (see the SVG compiler
section below).

```go
var b vector.Builder
b.MoveTo(math2d.Vec2{X: 0, Y: 0})
b.CubicTo(c1, c2, math2d.Vec2{X: 100, Y: 0})
b.Close()
path, err := b.Build()
```

### Fill and stroke

Filling and stroking flatten the path's curves into line segments at a given
tolerance and emit `render.DrawTriangles`. Flattening is adaptive recursive
subdivision (bounded at 18 levels of recursion) against a flatness test, and
each `Path` caches its most recently flattened tolerance so repeated
draws at an unchanged scale don't re-flatten every frame.

`StrokeStyle` carries `LineCap`, `LineJoin`, and `TrimStart`/`TrimEnd`
(fractions in `[0, 1]` along the path's total length) for a partial stroke —
the mechanism a "ring trace" or progress-arc animation is built from. Trim
values are validated: out-of-range or inverted (`TrimEnd < TrimStart`) values
return `ErrInvalidStroke` rather than drawing something degenerate.

### Motion sampling

`Path.PointAt(progress, tolerance)` walks the flattened path by arc length
and linearly interpolates a position and tangent at `progress * totalLength`
— the primitive an "offset-path" motion animation (a particle or icon
riding a curve) is built from. It returns `ErrInvalidProgress` for
`progress` outside `[0, 1]` and `ErrDegenerateGeometry` for a zero-length
path.

### Errors

Every fallible operation returns a sentinel error instead of panicking:
`ErrNonFinite`, `ErrNoCurrentSubpath`, `ErrEmptyPath`, `ErrInvalidTolerance`,
`ErrFlattenDepth`, `ErrFillContour`, `ErrSelfIntersection`,
`ErrDegenerateGeometry`, `ErrInvalidStroke`, `ErrInvalidProgress`. Check
these with `errors.Is` the same way you would `render.Builder`'s errors.

## `arasy svg`: SVG-to-Go compilation

`arasy svg <source.svg> --output <file.gen.go> --package <pkg> --name
<Name>` compiles an authored, animated SVG into a `.gen.go` file with no
runtime dependency on `internal/svggen`, XML, or CSS — the generated code
only imports `anim`, `ease`, `math2d`, `render`, and `render/vector`.

```powershell
go run ./cmd/arasy svg scene.svg `
  --output examples/vectoranim/vector.gen.go `
  --package vectoranim `
  --name VectorScene `
  --font "Display=ui-display"
```

Flags: `--output`, `--package`, and `--name` are required. `--font
FAMILY=MANIFEST_NAME` (repeatable) maps an SVG `font-family` to a name the
caller resolves at runtime from a loaded `asset.Manifest` — duplicate
family mappings are rejected. `--strict` (default `true`) turns any
attribute outside the supported subset into a compile error instead of
silently dropping it; only disable it while iterating on a new SVG source,
never for a checked-in asset.

### Supported subset

Elements: `svg, g, rect, circle, ellipse, line, polyline, polygon, path,
text, tspan, style`. Anything else — a different namespace, an unsupported
tag — fails to parse.

Attributes are an explicit allow-list covering geometry (`x, y, cx, cy, r,
rx, ry, width, height, points, d, x1, y1, x2, y2`), presentation (`fill,
stroke, stroke-width, stroke-linecap, stroke-linejoin, stroke-miterlimit,
opacity, stroke-dasharray, stroke-dashoffset`), text (`font-family,
font-size, font-weight, text-anchor, letter-spacing`), transform
(`transform, transform-box, transform-origin, viewBox,
preserveAspectRatio`), CSS keyframe animation (`animation, animation-name,
animation-duration, animation-delay, animation-iteration-count,
animation-direction, animation-fill-mode, animation-timing-function`),
motion (`offset-path, offset-distance`), plus `id, class, style,
pathLength, filter, type, media`. In non-strict mode an attribute outside
this list is dropped; in strict mode it's a compile error.

Forbidden content is rejected unconditionally, regardless of `--strict`:
doctype/entity declarations, XML stylesheet processing instructions,
`<script>`, `javascript:` URLs, `@import`, and `url(http:/https:///)`
references — nothing externally fetched or executable survives compilation.
There are also hard limits on input size, element count, nesting depth,
path command count, coordinate magnitude, and generated output size, so a
pathological input fails fast instead of producing a runaway `.gen.go`.

Path data accepts the classic command set — `M/m, L/l, H/h, V/v, C/c, S/s,
Q/q, T/t, A/a, Z/z` — including arcs. Arcs are converted to a sequence of
cubic Bézier commands during parsing, so by the time geometry reaches
`render/vector` only move/line/quad/cubic/close remain; there is no
compiled-time arc primitive to keep the runtime vocabulary small.

### Cost model: LOD and tolerance

Generated code buckets the current viewport scale into one of three levels
of detail — coarse below 0.75x, fine above 1.5x, standard in between — with
hysteresis at the boundaries so a viewport hovering near a threshold doesn't
flicker between LOD levels frame to frame. Each LOD level maps to a fixed
flatten tolerance (looser at the coarse level, tighter at the fine level),
which feeds directly into `vector.Path`'s fill/stroke flattening. Node
traversal also culls subtrees against the viewport bounds once a generated
asset has enough nodes for that check to pay for itself, rather than always
walking the full tree.

### Generated file shape

Every `.gen.go` file exposes the same shape, only prefixed by `--name`:

- `<Name>Resources` — one `render.FontID` field per `--font` mapping.
- `<Name>Metrics` / `<Name>Budget` / `WithinBudget` — structural cost
  limits (node/path/text/depth/command counts) a caller can enforce.
- `<Name>AssetMetrics` — the compile-time computed counts for this asset.
- `<Name>Workspace` — retained per-call state (the command buffer, a
  `render.Builder`, per-node fill/stroke tessellation workspaces, and LOD
  state), reused across frames to avoid reallocating.
- `(w *<Name>Workspace) Append(timeline, viewport, resources)
  []render.Command` and a stateless package-level `Append<Name>(dst,
  timeline, viewport, resources) []render.Command` wrapper.
- `<Name>FrameMetrics` — visited/culled node counts, emitted command count,
  LOD, and tolerance for the most recent `Append` call, useful for
  profiling or golden-testing traversal cost.

### Workflow: source SVG → generated Go → manifest resources

1. Author or generate the SVG (hand-drawn, or produced by an authoring tool
   external to the engine).
2. Compile it with `arasy svg`, choosing `--package`/`--name` for the call
   site that will own it, and `--font` mappings for any text the SVG
   contains.
3. Commit the generated `.gen.go` file next to the code that calls it — it
   has no build-time dependency on `internal/svggen`, so nothing at
   runtime needs the compiler.
4. At the call site, build (or reuse a cached) `<Name>Workspace`, resolve
   any `--font` mappings against a loaded `asset.Manifest` into the
   generated `<Name>Resources` struct, and call `Append` once per frame
   with a `Timeline` evaluated from `ctx.Time` (or boot-elapsed seconds,
   for splash content).

Text glyphs are outlined into path data at generation time rather than
referencing `render.DrawText` with a `FontID` when the SVG is meant to ship
without redistributing the authoring font — the engine splash does this: it
never populates its own `Resources` struct's font field, because its
wordmark is already plain path data. Populate `--font` mappings only when
the SVG genuinely needs live, engine-rendered text (e.g. runtime-editable
labels), not for a fixed wordmark or logotype.

### Regeneration and code review

`arasy svg` output is deterministic — generating from the same parsed
document twice yields byte-identical Go — but nothing currently re-runs the
compiler in CI and diffs it against the committed `.gen.go`. Treat the
source SVG as the reviewable artifact for design changes, and treat the
generated file as build output: review a regenerated `.gen.go` diff the way
you'd review any other build artifact refresh (does the diff's shape match
the source change, not "does every line make sense on its own"), and never
hand-edit a `.gen.go` file — regenerate it instead.

## Backend matrix

`render.DrawTriangles` — the primitive vector fill/stroke and generated SVG
content ultimately emit — is rasterized identically by every backend
because tessellation happens once, upstream of any backend. Current
per-backend notes (see `AGENTS.md`'s package responsibility table for the
authoritative, evolving version):

| Backend | Role |
|---|---|
| `platform/raster` | Software renderer to `image.RGBA`; the semantic reference every other backend is compared against. |
| `platform/wasm` (WebGPU) | Default web backend; full parity with the reference. |
| `platform/wasm` (Canvas2D) | Fallback web backend; approximates quads with an affine grid and caps retained sprite/label batch sizes (dev-only limits). |
| `platform/native` | wgpu-native + GLFW host; static retained sprite batches use persistent GPU records. Animated sprite and retained text batches remain unsupported. |

## Future extensions

The compiler and `render/vector` deliberately leave the following out of
scope for now — noted here so a future change knows it's an intentional
gap, not an oversight:

- Fill rules beyond nonzero/evenodd winding, and holes via compound paths.
- SMIL-style (`<animate>`/`<animateTransform>`) animation, as an alternative
  to the current CSS-keyframe-only subset.
- Gradients and masks as fill/stroke paints.
- Path morphing (interpolating between two `d` values).
- Retained vector batches, analogous to the retained sprite/text batch
  path, for scenes with many independently-animated vector shapes.
- A measured tessellation/flatten cost cache shared across assets, instead
  of the current per-`Path` single-tolerance cache.

## Related documents

- [`docs/en/animation.md`](animation.md) — `anim.Timeline`,
  `ui.Sequence`, and `render.Builder` group composition.
- [`docs/en/testing.md`](testing.md) — where `internal/svggen`'s parser
  and codegen tests (including its fuzz targets) live and how they're
  organized.
