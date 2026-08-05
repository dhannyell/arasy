# Distribution

This guide explains how to ship a native Arasy game: the two layouts a build
can take, where the assets are looked up, what an icon needs, which platforms
are supported and to what degree, and what a player sees when they open a
build that nobody signed.

A Portuguese translation of this guide is still pending; the rest of `docs/`
keeps PT/EN pairs.

## Two layouts

```powershell
go run ./cmd/arasy build -p native ./examples/breakout/native
go run ./cmd/arasy build -p native --mode bundle --assets examples/breakout/assets ./examples/breakout/native
```

| Layout | What ships | When to choose it |
| --- | --- | --- |
| `portable` (default) | one executable, assets compiled into it | a jam entry, a demo, a build attached to a bug report |
| `bundle` | the executable plus an `assets/` directory beside it | a large game, a store release, anything you want mods for |

`bundle` does **not** make the executable smaller. If the game keeps its
`//go:embed` of the assets, the bytes exist in both places. What `bundle` buys
is cheap patching and mods.

Cheap patching is the concrete reason. A store like Steam splits every file
into chunks of about a megabyte and ships only the chunks that changed.
Shifting bytes inside one large file breaks the alignment of everything after
them, so recompiling for a one-line fix moves the embedded assets and the
player re-downloads the whole executable. Loose files next to the executable
do not move when the code changes.

`pack` produces the archive for either layout:

```powershell
go run ./cmd/arasy pack -p native --mode bundle --assets examples/breakout/assets ./examples/breakout/native
```

The archive is named `<game>-<goos>-<goarch>.zip`, because a native build
always targets the machine that built it (see "No cross-compilation").

## Where assets are looked up

`asset.Load` takes FS layers and the last layer that has a path wins. The
canonical order for a native game is:

1. the game's `embed.FS` — the floor, optional;
2. `assets/` next to the executable — what `bundle` installs;
3. a user/mod directory — not in this release; the order reserves the place.

```go
layers := []fs.FS{embedded}
if external, ok := asset.ExecutableDirFS("assets"); ok {
	layers = append(layers, external)
}
manifest, err := asset.Load(layers...)
```

`ExecutableDirFS` resolves the directory from the executable's own path,
following symlinks — never from the working directory. A game launched by a
store client or a desktop shortcut starts in a directory that has nothing to
do with where it was installed, and reading assets relative to it is the
classic "works on my machine" bug.

A missing directory is not an error. A portable build simply reports `false`
and runs on its embedded floor.

## The layout on disk

```text
Game Name/
  game.exe        (no extension on the other operating systems)
  assets/         loose files, mirroring the manifest
  LICENSE
```

Loose files, not one archive: that is what keeps delta patching cheap and mods
possible.

A `THIRD_PARTY.md` listing the licenses of the dependencies belongs next to
these files. Nothing in the repository produces one yet — it is a known gap,
not an oversight.

## Icons

There are three icons and a developer thinks of them as one:

| Where | Mechanism |
| --- | --- |
| the `.exe` in Explorer | a Win32 resource compiled into the binary |
| taskbar and Alt+Tab | the same Win32 resource |
| the window itself | `Options.Icon`, which the host hands to GLFW |

Arasy has no `icon` subcommand: [`go-winres`](https://github.com/tc-hib/go-winres)
already does the Win32 half, and `go build` embeds any `.syso` it finds in the
`main` package's directory without a flag.

```powershell
go install github.com/tc-hib/go-winres@latest
go-winres simply --icon icon.png      # writes rsrc_windows_<arch>.syso
go-winres make                        # or, from a winres/ directory
```

Run those in the directory of the native entry point, then build as usual.

The window icon is the engine's half: fill `Options.Icon` with any
`image.Image` and the host hands it to GLFW.

```go
//go:embed icon.png
var iconPNG []byte

icon, err := png.Decode(bytes.NewReader(iconPNG))
if err != nil {
	return err
}
err = native.Run(game, native.Options{Title: "My Game", Icon: icon})
```

A window icon is drawn small — a title bar, a task switcher entry — so a
64-pixel source is a better choice than the largest art you have: the window
system scales whatever it is given, and scaling down from 1024 costs sharpness
at 16 pixels. The demos in this repository use the engine's own icon, which is
why they all wear the same one; a game ships its own.

Generated `.syso` files are build artifacts of a specific game, not of the
engine; keep them out of version control unless you commit the icon pipeline
alongside them.

## Platform support

Three tiers, stated honestly:

| Tier | Platforms | What it means |
| --- | --- | --- |
| 1 — supported | windows/amd64 | CI compiles it, runs the tests, and the maintainer plays on it |
| 2 — supported | linux/amd64 | CI compiles it and runs the tests; a human validates a real window before a release tag |
| 3 — best effort | darwin/arm64, linux/arm64, windows/arm64 | CI **compiles** it; nobody has played it. Declared untested, with no promise |

macOS is not tested. It sits in tier 3 for lack of hardware to validate it on,
which is a statement about this project, not a prediction about macOS. One
named risk: macOS requires window creation and the event loop to run on the
process's main thread. The host locks its goroutine to an OS thread, which is
necessary but not sufficient — locking to *a* thread does not guarantee it is
the *main* one. That is an item to verify when tier 3 gets hardware, not a
change to make blind.

### No cross-compilation

The native host needs cgo: GLFW is cgo, ALSA output on Linux is literally a
`#cgo pkg-config` line, and the WebGPU binding links prebuilt static
libraries. Cross-compiling cgo needs the target's C toolchain, which in
practice means you do not cross-compile — you build on each platform and
merge the artifacts afterwards. `build -p native` compiles for the host, and a
release matrix is a matrix of runners.

Build dependencies on Debian/Ubuntu:

```bash
sudo apt-get install -y \
  libegl1-mesa-dev mesa-vulkan-drivers libasound2-dev \
  libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev \
  libxkbcommon-dev libwayland-dev
```

## What the player needs

A driver with Vulkan or DirectX 12 support, and nothing else. The binary is
statically linked, so there is no redistributable to install — no VC++
runtime, no DirectX installer.

The console window is gone from a Windows release build: it links into the GUI
subsystem. That also discards stdout and stderr, so a released game writes its
diagnostics to a file instead:

```go
log, closeLog := native.NewLogger("Breakout")
defer func() { _ = closeLog() }()
```

One file under the user's cache directory (`%LocalAppData%` on Windows,
`~/.cache` on Linux, `~/Library/Caches` on macOS), truncated at boot, with its
own absolute path as the first line. That path is what a player attaches to a
bug report. Pass `--console` to `build`/`pack` to keep the console instead.

## Steam

- The depot holds loose files. Steam does not want an installer: the depot
  installs the content directly, and install scripts exist only for system
  redistributables — and a statically linked binary has none.
- Go applications can freeze under the Steam overlay on Windows. The published
  mitigation is to disable asynchronous preemption at link time:

  ```powershell
  go build -ldflags "-H=windowsgui -s -w -X=runtime.godebugDefault=asyncpreemptoff=1" ./native
  ```

  This is not a flag on the Arasy CLI, and it should not be: turning off
  asynchronous preemption has a cost of its own and must not be imposed on
  people who do not publish on Steam.
- A Linux binary linked against the runner's glibc is a promise about the
  player's glibc. The correct fix is to build inside the Steam Runtime
  container (`sniper` today; the older `scout` remains the publishing default
  for compatibility). This release documents the requirement; automating the
  container belongs to whoever publishes.

## Without code signing

Neither an Authenticode certificate nor an Apple Developer account is in reach
for this project, so builds are unsigned and players see it:

- **Windows**: SmartScreen shows "Windows protected your PC" and hides the run
  button behind "More info" → "Run anyway".
- **macOS**: Gatekeeper refuses the first launch of an unsigned, unnotarized
  binary; the player has to open it from the context menu, or clear it in
  System Settings → Privacy & Security.

Say so in the download page rather than letting a player discover it.
