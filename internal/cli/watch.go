package cli

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhannyell/arasy/asset/pixel"
	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/simfuzz"
	"github.com/dhannyell/arasy/internal/wasmserve"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/raster"
	"github.com/dhannyell/arasy/runtime"
	"github.com/spf13/cobra"
)

var (
	watchOutput    string
	watchFramesDir string
	watchWidth     int
	watchHeight    int
	watchLive      bool
	watchBrowser   bool
	watchAddr      string
)

var watchCmd = &cobra.Command{
	Use:   "watch <corpus> <replay.arrp>",
	Short: "watch a recorded fuzz case live, or render it as a GIF",
	Long: `watch loads a Go fuzz corpus entry and its paired .arrp recording,
reconstructs them with runtime.ReplayForViewing, and lets you look at the
case -- checksums already prove the simulation is correct, but only a look
at the rendered frames catches a camera stuck in place, a shadow that drifts
off its body, or a playback pace that does not match the recording.

With --live, it opens a real window and plays the
recording once at normal speed through the native host.

With --browser, it builds the WASM viewer under internal/simfuzz/wasm and
serves it with the recorded corpus/replay pair, auto-playing on load; the
page includes the existing dev panel (Backquote/~) for Play/Pause/Step/Scrub.

Without --live or --browser, it renders a scripted sequence -- play start to
end, rewind to the middle and play again, then scrub back and forth
mid-playback -- into a GIF (and, optionally, a PNG frame sequence) for
offline inspection.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWatch(cmd, args[0], args[1])
	},
}

func init() {
	watchCmd.Flags().StringVarP(&watchOutput, "output", "o", "", "output GIF path (default: output/watch/<replay-name>.gif)")
	watchCmd.Flags().StringVar(&watchFramesDir, "frames-dir", "", "optional directory for a frame-NNNN.png sequence")
	watchCmd.Flags().IntVar(&watchWidth, "width", 480, "render width in pixels")
	watchCmd.Flags().IntVar(&watchHeight, "height", 480, "render height in pixels")
	watchCmd.Flags().BoolVar(&watchLive, "live", false, "open a real window and play the recording once (needs a CLI built with -tags arr_dev and cgo)")
	watchCmd.Flags().BoolVar(&watchBrowser, "browser", false, "serve a WASM viewer with the dev panel enabled")
	watchCmd.Flags().StringVar(&watchAddr, "addr", "127.0.0.1:8095", "server address for --browser")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, corpusPath, replayPath string) error {
	script, err := readFuzzCorpus(corpusPath)
	if err != nil {
		return fmt.Errorf("read corpus %s: %w", corpusPath, err)
	}

	replayBytes, err := os.ReadFile(replayPath)
	if err != nil {
		return fmt.Errorf("read replay %s: %w", replayPath, err)
	}
	rec, err := runtime.ReadRecordingFromBytes(replayBytes)
	if err != nil {
		return fmt.Errorf("decode replay %s: %w", replayPath, err)
	}

	name := strings.TrimSuffix(filepath.Base(replayPath), filepath.Ext(replayPath))

	if watchBrowser {
		return runWatchBrowser(cmd, script, replayBytes, name)
	}

	log := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), nil))
	game := simfuzz.NewViewerGame(simfuzz.DecodeScript(script))

	// Probe once with the full simulation bounds as camera to find where the
	// actual content lives, then rebuild with a tighter camera: most fuzz
	// cases are tiny relative to the fixed world bounds, and a handful of
	// pixels in a fixed-size window reads as "nothing rendered".
	probe := runtime.ReplayForViewing(game, log, rec)
	game.FitContentToRecording(probe)
	driver := runtime.ReplayForViewing(game, log, rec)

	if watchLive {
		if !watchLiveSupported() {
			return fmt.Errorf("--live needs a CLI built with the native host: go run -tags arr_dev ./cmd/arasy (cgo on windows, linux or darwin, amd64 or arm64)")
		}
		return runWatchLive(driver, game, "watch: "+name)
	}

	target := newWatchTarget(driver, watchWidth, watchHeight)
	watchProcedure(driver, target)

	output := watchOutput
	if output == "" {
		output = filepath.Join("output", "watch", name+".gif")
	}
	if err := writeWatchGIF(output, target.frames); err != nil {
		return fmt.Errorf("write GIF %s: %w", output, err)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d frames)\n", output, len(target.frames)); err != nil {
		return err
	}

	if watchFramesDir != "" {
		if err := writeWatchFrames(watchFramesDir, target.frames); err != nil {
			return fmt.Errorf("write frame sequence %s: %w", watchFramesDir, err)
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s/frame-*.png (%d frames)\n", watchFramesDir, len(target.frames)); err != nil {
			return err
		}
	}

	return nil
}

// runWatchBrowser serves internal/simfuzz/wasm (the browser viewer) with the
// decoded corpus and the raw .arrp bytes staged alongside it as static
// files; the viewer fetches them at startup (see internal/simfuzz/wasm's
// doc comment) instead of anything being embedded at build time, so this is
// just a build + serve, mirroring `arasy dev`.
func runWatchBrowser(cmd *cobra.Command, corpus, replayBytes []byte, title string) error {
	dir, err := os.MkdirTemp("", "Arasy-watch-*")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := removeTempDir(dir); cleanupErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: remove temporary build directory %q: %v\n", dir, cleanupErr)
		}
	}()
	stopInterruptCleanup := watchTempDirInterrupt(dir)
	defer stopInterruptCleanup()

	if err := os.WriteFile(filepath.Join(dir, "watch-corpus"), corpus, 0o644); err != nil {
		return fmt.Errorf("stage corpus: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch-replay.arrp"), replayBytes, 0o644); err != nil {
		return fmt.Errorf("stage replay: %w", err)
	}
	if err := buildWASM("./internal/simfuzz/wasm", dir, "watch: "+title, wasmBuildOptions{}, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
		return fmt.Errorf("build browser viewer: %w", err)
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "serving watch viewer at http://%s (Backquote/~ opens the dev panel)\n", watchAddr); err != nil {
		return err
	}
	return wasmserve.Start(&wasmserve.ServerOptions{
		Dir: dir, Title: "watch: " + title, WASMFile: "app.wasm", Addr: watchAddr,
	})
}

// readFuzzCorpus decodes a Go fuzz seed corpus entry (the "go test fuzz v1"
// format) into the raw bytes the fuzz target received, mirroring the decode
// step Go's own fuzzing engine performs when replaying a corpus file.
func readFuzzCorpus(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Normalize CRLF first: a Windows checkout with core.autocrlf can turn the
	// repo's LF-only corpus file into CRLF locally.
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(normalized), "\n")
	if len(lines) != 2 || lines[0] != "go test fuzz v1" ||
		!strings.HasPrefix(lines[1], "[]byte(") || !strings.HasSuffix(lines[1], ")") {
		return nil, fmt.Errorf("not a single-[]byte-argument Go fuzz corpus entry")
	}
	quoted := strings.TrimSuffix(strings.TrimPrefix(lines[1], "[]byte("), ")")
	decoded, err := strconv.Unquote(quoted)
	if err != nil {
		return nil, fmt.Errorf("unquote corpus entry: %w", err)
	}
	return []byte(decoded), nil
}

// watchTarget renders each driver frame into a raster image and keeps an
// independent copy, so the sequence survives past the renderer's shared
// target buffer. The fuzz game never publishes textures or retained batches
// (see internal/simfuzz/render.go), so UpdateResources has nothing to do.
type watchTarget struct {
	renderer *raster.Renderer
	buffer   *image.RGBA
	frames   []*image.RGBA
}

func newWatchTarget(driver *runtime.Driver, width, height int) *watchTarget {
	cfg := driver.Config()

	textures, _ := pixel.BuildRegistry(cfg.Assets)
	fonts, _ := typeface.BuildRegistry(cfg.Assets)

	cam := cfg.Camera
	if cam.World.Width() <= 0 || cam.World.Height() <= 0 {
		// The fuzz game does not set a Camera in Setup(); fall back to its
		// simulation bounds so the rendered frame isn't degenerate.
		cam.World = cfg.Bounds
	}
	cam.Viewport = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)})

	renderer := &raster.Renderer{
		Width:    width,
		Height:   height,
		Camera:   cam,
		Clear:    cfg.Clear,
		Textures: textures,
		Fonts:    fonts,
		Assets:   cfg.Assets,
	}
	return &watchTarget{renderer: renderer, buffer: renderer.NewTarget()}
}

func (t *watchTarget) UpdateResources(runtime.FrameUpdates) {}

func (t *watchTarget) Render(frame runtime.Frame) {
	t.renderer.RenderInto(t.buffer, frame.Commands)
	snapshot := image.NewRGBA(t.buffer.Bounds())
	copy(snapshot.Pix, t.buffer.Pix)
	t.frames = append(t.frames, snapshot)
}

// watchProcedure exercises the driver exactly as a manual review should:
// play the whole recording from the start, rewind to the middle and play
// again, then scrub back and forth mid-playback. Every step renders one
// frame, so the output is a faithful record for visual inspection --
// pacing, camera, shadow alignment and keyframe continuity are all things a
// checksum cannot catch.
func watchProcedure(driver *runtime.Driver, target *watchTarget) {
	capture := func() { driver.RenderFrame(0, target) }
	scrubTo := func(tick uint64) {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: tick})
		driver.AdvanceTime(0)
	}
	playOneTick := func() {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlReplayPlay})
		driver.Step()
	}

	maxTick := driver.State().MaxRecordedTick

	scrubTo(0)
	capture()
	for driver.State().Tick < maxTick {
		playOneTick()
		capture()
	}

	mid := maxTick / 2
	scrubTo(mid)
	capture()
	for driver.State().Tick < maxTick {
		playOneTick()
		capture()
	}

	scrubTo(mid)
	capture()
	for _, tick := range scrubBackAndForth(mid, maxTick) {
		scrubTo(tick)
		capture()
	}
}

// scrubBackAndForth builds a short zig-zag of ticks around mid, staying
// inside [0, maxTick], to exercise repeated scrubbing during a watch session.
func scrubBackAndForth(mid, maxTick uint64) []uint64 {
	step := maxTick / 8
	if step == 0 {
		step = 1
	}

	sequence := make([]uint64, 0, 4)
	cursor := int64(mid)
	for _, delta := range []int64{int64(step), -2 * int64(step), 3 * int64(step), -int64(step)} {
		cursor += delta
		switch {
		case cursor < 0:
			cursor = 0
		case cursor > int64(maxTick):
			cursor = int64(maxTick)
		}
		sequence = append(sequence, uint64(cursor))
	}
	return sequence
}

// watchGIFPalette is a small, hand-picked palette matching the fuzz
// renderer's solid colors (see internal/simfuzz/render.go) plus background,
// outline and shadow tones. The standard 256-color palette.Plan9 with
// Floyd-Steinberg dithering measured over 500ms per 800x800 frame --
// prohibitive for the hundreds of frames a long recording produces. This
// renderer only ever draws a handful of solid, non-gradient colors, so a
// small palette with plain nearest-color quantization looks the same and is
// close to 30x faster.
var watchGIFPalette = color.Palette{
	color.White,
	color.Black,
	color.RGBA{R: 128, G: 128, B: 128, A: 255},
	color.RGBA{R: 89, G: 89, B: 107, A: 255},   // bounds outline
	color.RGBA{R: 115, G: 115, B: 128, A: 255}, // static body
	color.RGBA{R: 77, G: 179, B: 230, A: 255},  // dynamic body
	color.RGBA{R: 217, G: 179, B: 64, A: 255},  // body without a Body component
	color.RGBA{R: 242, G: 115, B: 89, A: 255},  // velocity vector
}

func writeWatchGIF(path string, frames []*image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	anim := &gif.GIF{
		Image: make([]*image.Paletted, 0, len(frames)),
		Delay: make([]int, 0, len(frames)),
	}
	for _, frame := range frames {
		paletted := image.NewPaletted(frame.Bounds(), watchGIFPalette)
		draw.Draw(paletted, frame.Bounds(), frame, image.Point{}, draw.Src)
		anim.Image = append(anim.Image, paletted)
		anim.Delay = append(anim.Delay, 2) // ~50fps, close to one GIF frame per recorded tick
	}
	err = gif.EncodeAll(f, anim)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func writeWatchFrames(dir string, frames []*image.RGBA) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, frame := range frames {
		path := filepath.Join(dir, fmt.Sprintf("frame-%04d.png", i))
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		err = png.Encode(f, frame)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
