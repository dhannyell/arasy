package runtime

import "github.com/dhannyell/frameforge/render"

// Game is the user-facing contract.
type Game interface {
	//Setup returns the static configuration. Called once before the loop.
	Setup() Config

	//Init populates a new World. Called at the start AND on every reset.
	Init(ctx *Context)

	// Update advances one fixed-timestep tick: reads ctx.Input, drives entities,
	// calls ctx.World.Step(dt, ctx.StepConfig), and reacts to ctx.World.Contacts().
	// Full control — the World is never hidden.
	Update(ctx *Context, dt float64)

	// Render returns the drawing commands for the current state of the World.
	Render(ctx *Context) []render.Command
}
