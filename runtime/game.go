package runtime

import (
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// Game is the user-facing contract.
type Game interface {
	// Setup returns the static configuration. Called once before the loop.
	Setup() Config

	// Init populates a new World. Called at the start AND on every reset.
	Init(ctx *UpdateContext)

	// Update reads input and drives entities for one fixed-timestep tick. The
	// Driver automatically steps ctx.World after Update; call
	// ctx.HoldSimulation() to keep this tick frozen. Games that need contacts
	// from the same tick can implement PostStepper.
	Update(ctx *UpdateContext, dt float64)

	// Render returns the drawing commands for the current state of the World.
	Render(ctx *RenderContext) []render.Command
}

// PostStepper is an optional game interface for reacting immediately after
// the Driver's automatic World.Step. It is not called when Update holds the
// simulation for the tick.
type PostStepper interface {
	PostStep(ctx *UpdateContext, stats scene.StepStats)
}
