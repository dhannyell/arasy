# Vector animation

Generated and hand-authored vector animation.

[![Run in your browser](https://img.shields.io/badge/Run_in_your_browser-2ea44f?style=for-the-badge)](https://dhannyell.github.io/arasy/examples/vectoranim/)

![A frame of the example](vectoranim.png)

## What it shows

- Timelines from the `anim` package that drive shape, color, and path properties.
- Generated animation next to hand-authored animation in one scene.

## Run locally

From the repository root:

```bash
go run ./cmd/arasy dev ./examples/vectoranim/wasm
```

Then open the address that the command prints.

Native window:

```bash
go run ./cmd/arasy run ./examples/vectoranim/native
```

Offline PNG through the raster host:

```bash
go run ./examples/vectoranim/raster
```
