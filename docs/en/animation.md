# Animation

This guide explains Arasy's presentation-animation stack: `anim.Timeline`,
`ui.Sequence`, `render.Builder` groups, and how they relate to simulation
ticks. It does not cover gameplay animation driven by deterministic
simulation state (sprite state machines tied to `scene` components) — that
belongs to game code, not this stack.

## When to use `anim`, `ui.Sequence`, or a presentation timeline

Arasy keeps two clocks strictly separate:

- **Simulation ticks** (`UpdateContext.Tick`) are deterministic, replayable,
  and rewindable. Gameplay state — position, health, whether an entity is
  alive — must never depend on wall-clock time, or replay/rewind desyncs.
- **Render-elapsed time** (`RenderContext.Time`/`Dt`) is cosmetic. It keeps
  advancing during pause, slow-motion, and scrubbing, and is the only clock
  `anim.Timeline` ever reads.

Use `anim.Timeline` (or the `ui.Sequence` alias) for anything that is purely
visual: fades, easing, particle-adjacent flourishes, UI transitions, one-shot
reveal animations, and the boot splash. If a change must survive
replay/rewind and affect simulation outcome, model it as scene state advanced
in `Update`, not as an animation timeline.

`ui.Sequence` is the same type as `anim.Timeline` (`type Sequence =
anim.Timeline`), used through the `ui` package's `Drawer` contract so widgets
can trigger a scripted animation keyed by an arbitrary state change
(`SequenceKeyed(id, key)`): passing `key == 0` means "never triggered" and
returns an inactive timeline; changing `key` restarts the sequence from the
current render time; an unchanged `key` keeps counting from the original
trigger.

A **presentation timeline** — the term used for the compiled engine splash
and other `runtime.SplashSegment` content — is the same `anim.Timeline`
value, just evaluated by `runtime.Driver` against boot-time elapsed seconds
instead of `ctx.Time`. There's only one implementation across all three
uses.

## Clock ownership

`anim.Timeline` is a stateless, presentation-only value: it carries no
wall-clock or frame-counter state of its own. It holds one caller-supplied
elapsed-time snapshot and evaluates tracks against it. The caller owns the
clock and reconstructs (not advances) a `Timeline` every frame:

```go
func (g *Game) Render(dst []render.Command, ctx runtime.RenderContext) []render.Command {
    timeline := anim.At(math.Mod(ctx.Time, duration)) // looping
    return g.vectorWorkspace.Append(dst, timeline, ctx.Camera().World, VectorSceneResources{})
}
```

`anim.At(elapsed)` builds an active timeline at that elapsed time;
`anim.Inactive()` builds one whose tracks stay at zero and report done —
useful as the default/never-triggered case. There is no `Advance(dt)`
method: nothing is mutated in place, so the same `Timeline` value can be
evaluated any number of times, from any goroutine, without synchronization.

`RenderContext.Time`/`Dt` are the render-elapsed clock this stack is built
on — see [`docs/en/runtime.md`](runtime.md) for how they diverge from
simulation ticks during pause and rewind. `UpdateContext` intentionally has
no `Time` field: deterministic simulation code has no legitimate use for
wall-clock time, so the type doesn't offer it.

## `Timeline.Track`

`Track(start, end float64, curve ease.Curve) Track` evaluates one eased
window of the timeline and returns a small value with `Value()`, `Lerp(a,
b)`, and `Done()`:

```go
pulse := timeline.Track(1.8, 2.6, ease.InOutQuad)
opacity := pulse.Lerp(0, 0.75)
```

`Done(duration)` on the timeline itself reports whether the timeline is
inactive or has reached `duration` — the usual way a segment decides it has
finished playing.

## `render.Builder`: groups, composition, and unsupported commands

`render.Builder` is what `Timeline`-driven code (hand-authored or generated
by `arasy svg`) uses to emit `render.Command` values with composed
transform/opacity/clip/layer state, instead of every call site doing that
math itself:

```go
var b render.Builder
b.Reset(dst)
b.Push(render.Group{
    Transform: math2d.Scale(pulse.Lerp(1, 1.3), pulse.Lerp(1, 1.3)).About(center),
    Opacity:   pulse.Lerp(0, 0.75),
})
b.Add(render.DrawCircle{Center: center, Radius: 12, Color: accent})
b.Pop()
commands := b.Commands()
if err := b.Err(); err != nil {
    // handle/log — see below
}
```

Nested `Push` calls compose with their parent: transforms multiply,
opacities multiply, clips intersect, and layer offsets add. Every command
added under a group gets that composed state baked into its own
`BaseCommand` fields, so a backend never has to walk a group stack itself.

Builder methods never panic on misuse. Instead, the first error is stored
and every subsequent `Push`/`Add`/`Pop` becomes a no-op until you inspect
`Err()`:

- `ErrInvalidGroup` — a `Group` with a non-finite field.
- `ErrStackUnderflow` — `Pop()` with nothing pushed.
- `ErrUnclosedGroup` — `Commands()` called with an open group.
- `ErrUnsupportedGroupedCommand` — either a command type with no known
  per-vertex composition rule, or one of `DrawParticles`, `DrawSpriteBatch`,
  `DrawTextBatch` added under a **non-identity** group. Those three are
  retained/emitter commands whose backend-side state can't be re-derived
  per vertex from an ad hoc transform/opacity, so the builder refuses rather
  than silently drawing them wrong.

Check `Err()` (or `errors.Is`/`errors.As` against the sentinel kinds) after
building; generated `arasy svg` code and hand-authored builder code follow
the same contract.

## `DrawTriangles` and backend parity

`render.DrawTriangles` is the one backend-agnostic primitive that curve
tessellation (from `render/vector`, or from generated SVG fills/strokes)
ultimately emits: indexed solid-color triangles, validated so index count is
a multiple of three and every index is in range. Because tessellation
happens once, upstream of any backend, raster, Canvas2D, WebGPU, and native
all rasterize the exact same vertex/index/color data with the same
transform — parity comes from sharing the geometry, not from four separate
implementations agreeing by convention. See `AGENTS.md`'s package
responsibility table for the current per-backend caveats (e.g. Canvas2D's
sprite/label caps).

## Related documents

- [`docs/en/vector-graphics.md`](vector-graphics.md) — the `render/vector`
  path/stroke/motion API and the `arasy svg` SVG-to-Go compiler that
  produces generated `Timeline`-driven content.
- [`docs/en/runtime.md`](runtime.md) — `RenderContext`/`UpdateContext`
  and the tick/render clock split.
- [`docs/en/ui.md`](ui.md) — the `Drawer` contract, including
  `SequenceKeyed`.
