# Boot Splash

## Runtime phase

`runtime.Driver.RunSplashFrame` owns the phase before tick 0.

- Real-time WASM and native hosts provide viewport and input during the splash.
- Raster and replay paths skip the splash and start at tick 0.
- Splash time uses the wall clock only.
- Splash time does not advance the simulation tick, recorder, checksum, or
  rewind state.
- A skippable segment advances only after activation input.

## Playlist

The playlist can contain an optional engine segment followed by manifest
`"splash"` segments.

The engine segment is a compiled animation from `internal/enginesplash`. It is
rendered as texture-free vector commands. It has no manifest entry and requires
no texture upload. `Config.DisableDefaultSplash` disables it.

Game segments are texture flip-books from the manifest. Their fields are
`cell_w`, `cell_h`, `columns`, `frames`, `rate`, `min_time`, `fade`, and
`skippable`. Use `arasy atlas <frames-dir|gif> -o out.png` to pack frames.

## Host and web options

- `--no-engine-splash` is a host-only option. It keeps manifest segments and is
  enabled by default by `arasy dev`.
- `--no-loading` controls the HTML download indicator only.
- `--no-splash` controls runtime splash segments only.
- `--boot-background` sets the web phase-0 color.
- `Config.BootBackground` sets the runtime splash color and falls back to
  `Config.Clear`.

`HoldUntilReady` loading-screen behavior is not implemented. Every segment is
currently bounded by `MinTime`.
