# Text render benchmark

Build and serve the standalone browser benchmark:

```powershell
go run ./cmd/frameforge dev ./examples/bench/textbench/wasm -a 127.0.0.1:8093
```

It lays out N HUD-style text labels (`"Player 1234  42 pts"`) on a
deterministic 50-column grid across a 1920x1080 world and emits one
`render.DrawText` per label. This isolates the streaming text render path:
per-frame atlas resolution, one-pass proportional layout into compact glyph
records, one shared transform record per label, and the textured glyph-atlas
draw runs that consume them. It deliberately does not retain shaped labels
between frames; registered, persistent text belongs to the separate batch
feature.

Controls:

- `UP` doubles the label count, `DOWN` halves it (starts at 500, clamped to
  [1, 20,000]).
- `T` toggles between one shared font size and four distinct sizes cycled by
  label index. Each distinct size requests its own `(FontID, PX)` atlas, so
  multi-size mode forces the per-frame atlas batching to resolve four atlases
  instead of one.
- `E` toggles between the same label text repeated for every entry and a
  unique string per label (distinct player id and score derived from the
  index). Unique text is the worst case for streaming layout because every
  label and glyph is emitted independently each frame.

The HUD reports label count, size mode, text mode, rolling FPS, frame time,
P99 frame time, the WebGPU render pass GPU time, and the same CPU tick/render
and tessellation/upload/encode breakdown reported by the sprite and shape
benchmarks.
