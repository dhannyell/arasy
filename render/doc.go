// Package render defines backend-agnostic render commands.
//
// The package does not draw directly. Instead, game and simulation code produce
// commands such as DrawRect, DrawCircle and DrawText. Platform backends can then
// translate those commands to Canvas, PNG, native rendering or another target.
package render
