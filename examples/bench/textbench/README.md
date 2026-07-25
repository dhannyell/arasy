# Text render benchmark

Build and serve the standalone browser benchmark:

```powershell
go run ./cmd/Arasy dev ./examples/bench/textbench/wasm -a 127.0.0.1:8093
```

It lays out N HUD-style text labels (`"Player 1234  42 pts"`) on a
deterministic 50-column grid across a 1920x1080 world, and toggles between
two render modes at the same label count:

- **streaming** (default): one `render.DrawText` per label, every frame.
  Isolates the streaming text render path: per-frame atlas resolution,
  one-pass proportional layout into compact glyph records, one shared
  transform record per label, and the textured glyph-atlas draw runs that
  consume them.
- **retained**: labels are published once through `runtime.BatchPort` as a
  `render.TextBatchID` and `Render` emits a single `render.DrawTextBatch` per
  frame; the underlying glyph/label buffers only re-pack when content, the
  camera/projection, or the font atlas actually changes. A mutation control
  (below) selects what fraction of the batch changes text each tick, since
  v1 retained batches always replace the whole batch on any change.

Controls:

- `UP` doubles the label count, `DOWN` halves it (starts at 500, clamped to
  [1, 20,000]).
- `T` toggles between one shared font size and four distinct sizes cycled by
  label index. Each distinct size requests its own `(FontID, PX)` atlas, so
  multi-size mode forces the atlas batching to resolve four atlases instead
  of one.
- `E` toggles between the same label text repeated for every entry and a
  unique string per label (distinct player id and score derived from the
  index). Unique text is the worst case for streaming layout because every
  label and glyph is emitted independently each frame; in retained mode it
  only matters on a republish.
- `P` toggles streaming/retained at the current label count. Leaving retained
  mode deletes the batch so stats reflect only the active mode.
- `M` (retained mode only) cycles the mutated fraction of the batch:
  0% -> 1% -> 100%. Any value above 0% pays the same full-batch repack cost
  in v1 — the control makes that cost visible, it does not demonstrate a
  speedup at partial mutation.
- `U` (retained mode only) injects one "Unicode wave": marks 50 more labels
  (capped at the current label count) with a non-ASCII rune, forcing font
  atlas growth and exactly one repack. Triggering it again after every label
  is already marked is a no-op.

The HUD reports label count, size mode, text mode, render mode (plus
mutation % in retained mode), rolling FPS, frame time, P99 frame time, the
WebGPU render pass GPU time, the CPU tick/render and tessellation/upload/
encode breakdown shared with the sprite and shape benchmarks, and — once a
retained batch has been published — a `retained` line with GPU-resident
bytes, content bytes uploaded this frame (0 at steady state), visible/total
label count and retained draw calls. The retained line reads
`unavailable (Canvas2D)` on the Canvas2D fallback, which has no equivalent
counters.

## Retained-path results (RTX 4080 Super / 5800X3D, Chromium, manual HUD reads)

- 20,000 unique TTF labels, retained mode, 0% mutation: 60 FPS sustained,
  `upload 0 B/frame` once the batch is stable.
- Triggering a Unicode wave costs one frame of atlas growth and repack, then
  returns to `upload 0 B/frame` on the next frame — the rolling FPS average
  dips briefly because a single expensive sample is diluted across its
  averaging window, not because the cost is sustained.
- Mutation at any nonzero percentage forces the same full-batch repack cost
  as 100%: at tens of thousands of labels, even 1% mutation can cost more
  per frame than the equivalent streaming draw at the same count, because
  the whole batch is re-laid-out and re-uploaded. This is the expected v1
  tradeoff (whole-batch replace, no partial/dirty-range update), not a
  regression.
