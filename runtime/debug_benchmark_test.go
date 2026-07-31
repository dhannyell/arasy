package runtime

import (
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// allDebugLayers is what a developer gets by ticking every box in the dev
// panel, and the state these benchmarks measure: the overlay runs once per
// frame, so anything expensive here is paid at frame rate, not tick rate.
const allDebugLayers = scene.DebugColliders | scene.DebugContacts | scene.DebugGrid |
	scene.DebugActiveGrid | scene.DebugCandidateGrid | scene.DebugVelocities |
	scene.DebugEntityIDs | scene.DebugRenderLayers | scene.DebugInputState |
	scene.DebugRuntimeStats

// debugBenchGame populates a world of moving, colliding bodies over a world
// large enough that the grid overlay has real work to do.
type debugBenchGame struct{ entities int }

func (g debugBenchGame) Setup() Config {
	bounds := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 200, Y: 120})
	return Config{
		Bounds: bounds,
		Camera: render.Camera{
			World:    bounds,
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}),
		},
		Physics: scene.PhysicsConfig{CellSize: 4},
	}
}

func (g debugBenchGame) Init(ctx *UpdateContext) {
	for i := range g.entities {
		id := ctx.World.Spawn()
		x := float64(i%40)*5 + 2
		y := float64(i/40)*5 + 2
		ctx.World.SetTransform(id, scene.Transform{
			Position: math2d.Vec2{X: x, Y: y},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		ctx.World.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 3, Y: 2}})
		ctx.World.SetCircleCollider(id, scene.CircleCollider{Radius: 1})
	}
}

func (g debugBenchGame) Update(*UpdateContext, float64) {}

func (g debugBenchGame) Render(*RenderContext) []render.Command { return nil }

func benchDebugDriver(b *testing.B, entities int) *Driver {
	b.Helper()
	driver := NewDriver(debugBenchGame{entities: entities}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	driver.SkipSplash()
	driver.Control(ControlCommand{Kind: ControlSetDebugLayers, DebugLayers: allDebugLayers})
	for range 30 {
		driver.Step()
	}
	return driver
}

// BenchmarkDebugCommandsAllLayers is the frame cost of the overlay itself.
func BenchmarkDebugCommandsAllLayers(b *testing.B) {
	driver := benchDebugDriver(b, 200)
	cam := driver.cameraForSurface()
	gameCmds := benchGameCommands(400)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cmds := driver.debugCommands(gameCmds, cam)
		if len(cmds) == 0 {
			b.Fatal("overlay produced no commands")
		}
	}
}

// BenchmarkDebugCommandsGridOnly isolates the layer that dominates: the grid
// is static geometry, so its cost is pure per-frame rebuild.
func BenchmarkDebugCommandsGridOnly(b *testing.B) {
	driver := benchDebugDriver(b, 200)
	driver.Control(ControlCommand{Kind: ControlSetDebugLayers, DebugLayers: scene.DebugGrid})
	driver.Step()
	cam := driver.cameraForSurface()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if len(driver.debugCommands(nil, cam)) == 0 {
			b.Fatal("grid produced no commands")
		}
	}
}

// BenchmarkDebugCommandsRuntimeStatsOnly isolates the checksum, which is
// O(entities) over every component store.
func BenchmarkDebugCommandsRuntimeStatsOnly(b *testing.B) {
	driver := benchDebugDriver(b, 200)
	driver.Control(ControlCommand{Kind: ControlSetDebugLayers, DebugLayers: scene.DebugRuntimeStats})
	driver.Step()
	cam := driver.cameraForSurface()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if len(driver.debugCommands(nil, cam)) == 0 {
			b.Fatal("stats produced no commands")
		}
	}
}

func benchGameCommands(n int) []render.Command {
	cmds := make([]render.Command, 0, n)
	for i := range n {
		cmds = append(cmds, render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: i % 12},
		})
	}
	return cmds
}
