package simfuzz

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// ViewerGame wraps a *Game and fills in what its Setup() never sets: a
// Camera (so the renderer doesn't collapse the world into a degenerate
// transform) and an opaque Clear color (its zero value is fully transparent
// black -- a PNG viewer shows that as white by compositing over its own
// background, but an opaque GPU swapchain shows it as a plain black screen).
// Every host that watches a recorded case interactively (the CLI's offline
// raster/native paths, the browser viewer) builds on this instead of NewGame
// directly, so they all agree on what the case looks like.
type ViewerGame struct {
	*Game
	cameraWorld math2d.Rect
}

// NewViewerGame builds a fuzz game for watching a recorded case rather than
// fuzzing or verifying one.
func NewViewerGame(script Script) *ViewerGame {
	return &ViewerGame{Game: NewGame(script)}
}

func (g *ViewerGame) Setup() runtime.Config {
	cfg := g.Game.Setup()
	switch {
	case g.cameraWorld.Width() > 0 && g.cameraWorld.Height() > 0:
		cfg.Camera.World = g.cameraWorld
	case cfg.Camera.World.Width() <= 0 || cfg.Camera.World.Height() <= 0:
		cfg.Camera.World = cfg.Bounds
	}
	if cfg.Clear.A == 0 {
		cfg.Clear = render.RGBA(0.05, 0.05, 0.08, 1)
	}
	return cfg
}

// FitContentToRecording scrubs driver through every recorded tick and
// measures the extent of everything the game actually draws, skipping the
// world-bounds outline itself, then keeps a padded version of that box as
// the camera for the game's next Setup() call. Most fuzz cases are tiny
// relative to the fixed simulation bounds; without this, a handful of pixels
// in an otherwise empty frame reads as "nothing rendered".
//
// Setup() is only read once, at driver construction, so the caller must
// build a fresh driver afterward (e.g. a second runtime.ReplayForViewing
// call) to pick up the new camera -- this call alone does not affect
// driver's own camera. It leaves driver at the tick it started from.
func (g *ViewerGame) FitContentToRecording(driver *runtime.Driver) {
	if bounds, ok := computeContentBounds(driver); ok {
		g.cameraWorld = padContentRect(bounds, 0.35, 8)
	}
}

func computeContentBounds(driver *runtime.Driver) (math2d.Rect, bool) {
	cfg := driver.Config()
	maxTick := driver.State().MaxRecordedTick
	startTick := driver.State().Tick

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	found := false
	expand := func(x, y float64) {
		if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
			return
		}
		found = true
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}

	scan := func() {
		for _, cmd := range driver.Frame(0).Commands {
			switch c := cmd.(type) {
			case render.DrawRect:
				if c.Rect == cfg.Bounds {
					continue // the world outline, not content
				}
				expand(c.Rect.Min.X, c.Rect.Min.Y)
				expand(c.Rect.Max.X, c.Rect.Max.Y)
			case render.DrawCircle:
				expand(c.Center.X-c.Radius, c.Center.Y-c.Radius)
				expand(c.Center.X+c.Radius, c.Center.Y+c.Radius)
			case render.DrawEllipse:
				expand(c.Center.X-c.RadiusX, c.Center.Y-c.RadiusY)
				expand(c.Center.X+c.RadiusX, c.Center.Y+c.RadiusY)
			case render.DrawLine:
				expand(c.From.X, c.From.Y)
				expand(c.To.X, c.To.Y)
			}
		}
	}
	scrubTo := func(tick uint64) {
		driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: tick})
		driver.AdvanceTime(0)
	}

	scrubTo(0)
	scan()
	for tick := uint64(1); tick <= maxTick; tick++ {
		scrubTo(tick)
		scan()
	}
	scrubTo(startTick)

	if !found {
		return math2d.Rect{}, false
	}
	return math2d.NewRectFromCorners(math2d.Vec2{X: minX, Y: minY}, math2d.Vec2{X: maxX, Y: maxY}), true
}

// padContentRect grows r by fraction of its own size on each axis, with
// minPad as a floor so a single near-static point (zero-size bounds) still
// gets a sensible viewing window instead of collapsing to nothing.
func padContentRect(r math2d.Rect, fraction, minPad float64) math2d.Rect {
	padX := max(r.Width()*fraction, minPad)
	padY := max(r.Height()*fraction, minPad)
	return math2d.NewRectFromCorners(
		r.Min.Sub(math2d.Vec2{X: padX, Y: padY}),
		r.Max.Add(math2d.Vec2{X: padX, Y: padY}),
	)
}
