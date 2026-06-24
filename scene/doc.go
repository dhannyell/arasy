// Package scene provides a small entity/component world for 2D simulations.
//
// The package is intentionally simple. An EntityID identifies an entity, while
// components store data associated with that entity. Systems are regular World
// methods that operate over entities with specific component combinations.
//
// Current component combinations:
//
//	Transform + CircleSprite
//		Produces render.DrawCircle commands through RenderCommands.
//
//	Transform + Velocity
//		Moves entities through Integrate.
//
//	Transform + Velocity + CircleCollider
//		Allows BounceCirclesInside to keep moving circles inside a Rect.
//
// The scene package does not draw pixels and does not depend on a concrete
// platform backend. Rendering is expressed as backend-agnostic commands from
// the render package.
//
// World currently stores components in maps keyed by EntityID. This keeps the
// first version easy to understand, but map iteration order is not deterministic
// in Go. Systems whose output depends on entity order should use an explicit
// ordering strategy before determinism-sensitive behavior relies on it.
package scene
