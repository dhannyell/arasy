// Package scene provides a small entity/component world for 2D simulations.
//
// An EntityID identifies an entity. Components store data associated with that
// entity, and World methods act as systems over component combinations.
//
// Common component combinations:
//
//	Transform + CircleSprite/RectSprite
//		Produces backend-agnostic render commands.
//
//	Transform + Velocity
//		Moves entities during World.Step.
//
//	Velocity + Acceleration/Damping
//		Changes velocity before/after integration.
//
//	Transform + CircleCollider/RectCollider
//		Enables bounds checks, broad-phase detection and contact resolution.
//
// Step runs the simulation pipeline in order: acceleration, integration,
// bounds bounce, grid broad-phase, contact resolution and damping.
//
// The scene package does not draw pixels and does not depend on a concrete
// platform backend. Rendering and debug overlays are expressed as render
// commands.
//
// World currently stores components in maps keyed by EntityID. This keeps the
// implementation simple, but Go map iteration order is not deterministic.
// Systems whose output depends on entity order must sort explicitly.
package scene
