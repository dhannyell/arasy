package runtime

import (
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

type autoStepGame struct {
	entity scene.EntityID
}

func (g *autoStepGame) Setup() Config {
	return Config{
		Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
}

func (g *autoStepGame) Init(ctx *UpdateContext) {
	g.entity = ctx.World.Spawn()
	ctx.World.SetTransform(g.entity, scene.Transform{Position: math2d.Vec2{X: 10, Y: 10}, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(g.entity, scene.Velocity{Linear: math2d.Vec2{X: 60, Y: 0}})
	ctx.World.SetCircleCollider(g.entity, scene.CircleCollider{Radius: 1})
}

func (g *autoStepGame) Update(ctx *UpdateContext, dt float64) {}

func (g *autoStepGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestDriverAutoStepsWorld(t *testing.T) {
	game := &autoStepGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step()

	transform, ok := driver.world.Transform(game.entity)
	if !ok {
		t.Fatal("expected entity transform")
	}
	if transform.Position.X != 11 {
		t.Fatalf("position X = %v, want 11", transform.Position.X)
	}
	if driver.world.StepTick() != 1 {
		t.Fatalf("world StepTick = %d, want 1", driver.world.StepTick())
	}
}

type holdSimulationGame struct{}

func (holdSimulationGame) Setup() Config {
	return Config{
		Bounds: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
}

func (holdSimulationGame) Init(ctx *UpdateContext) {
	entity := ctx.World.Spawn()
	ctx.World.SetTransform(entity, scene.Transform{Position: math2d.Vec2{X: 10, Y: 10}, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(entity, scene.Velocity{Linear: math2d.Vec2{X: 60, Y: 0}})
	ctx.World.SetCircleCollider(entity, scene.CircleCollider{Radius: 1})
}

func (holdSimulationGame) Update(ctx *UpdateContext, dt float64) {
	ctx.HoldSimulation()
}

func (holdSimulationGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestHoldSimulationSkipsStep(t *testing.T) {
	driver := NewDriver(holdSimulationGame{}, slog.Default())

	driver.Step()

	if driver.State().Tick != 1 {
		t.Fatalf("driver tick = %d, want 1", driver.State().Tick)
	}
	if driver.world.StepTick() != 0 {
		t.Fatalf("world StepTick = %d, want 0", driver.world.StepTick())
	}
}

type postStepContactGame struct {
	contactsInPostStep int
	postSteps          int
}

func (g *postStepContactGame) Setup() Config {
	return Config{
		Bounds: math2d.NewRect(math2d.Vec2{X: -100, Y: -100}, math2d.Vec2{X: 200, Y: 200}),
	}
}

func (g *postStepContactGame) Init(ctx *UpdateContext) {
	first := ctx.World.Spawn()
	second := ctx.World.Spawn()
	ctx.World.SetTransform(first, scene.Transform{Position: math2d.Vec2{X: 0, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetTransform(second, scene.Transform{Position: math2d.Vec2{X: 3, Y: 0}, Scale: math2d.Vec2{X: 1, Y: 1}})
	ctx.World.SetVelocity(first, scene.Velocity{})
	ctx.World.SetVelocity(second, scene.Velocity{})
	ctx.World.SetCircleCollider(first, scene.CircleCollider{Radius: 2})
	ctx.World.SetCircleCollider(second, scene.CircleCollider{Radius: 2})
	ctx.World.SetBody(first, scene.Body{InvMass: 1, Restitution: 1})
	ctx.World.SetBody(second, scene.Body{InvMass: 1, Restitution: 1})
}

func (g *postStepContactGame) Update(ctx *UpdateContext, dt float64) {}

func (g *postStepContactGame) PostStep(ctx *UpdateContext, stats scene.StepStats) {
	g.postSteps++
	g.contactsInPostStep = len(ctx.World.Contacts())
	if stats.Contacts != g.contactsInPostStep {
		panic("stats contacts differ from world contacts")
	}
}

func (g *postStepContactGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestPostStepRunsSameTickWithContacts(t *testing.T) {
	game := &postStepContactGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step()

	if game.postSteps != 1 {
		t.Fatalf("PostStep calls = %d, want 1", game.postSteps)
	}
	if game.contactsInPostStep != 1 {
		t.Fatalf("contacts in PostStep = %d, want 1", game.contactsInPostStep)
	}
}

type heldPostStepGame struct {
	postSteps int
}

func (g *heldPostStepGame) Setup() Config           { return Config{} }
func (g *heldPostStepGame) Init(ctx *UpdateContext) {}
func (g *heldPostStepGame) Update(ctx *UpdateContext, dt float64) {
	ctx.HoldSimulation()
}
func (g *heldPostStepGame) PostStep(ctx *UpdateContext, stats scene.StepStats) {
	g.postSteps++
}
func (g *heldPostStepGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestPostStepNotCalledWhenHeld(t *testing.T) {
	game := &heldPostStepGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step()

	if game.postSteps != 0 {
		t.Fatalf("PostStep calls = %d, want 0", game.postSteps)
	}
}

type manualStepGame struct{}

func (manualStepGame) Setup() Config           { return Config{} }
func (manualStepGame) Init(ctx *UpdateContext) {}
func (manualStepGame) Update(ctx *UpdateContext, dt float64) {
	ctx.World.Step(dt, ctx.StepConfig)
}
func (manualStepGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestManualStepSkipsAutoStep(t *testing.T) {
	driver := NewDriver(manualStepGame{}, slog.Default())

	driver.Step()

	if driver.world.StepTick() != 1 {
		t.Fatalf("world StepTick = %d, want exactly 1", driver.world.StepTick())
	}
}
