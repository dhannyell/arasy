// Package enginesplash holds the engine's built-in boot splash content,
// compiled offline from a SVG into deterministic Go that emits
// render.Command via anim.Timeline + render/vector (see arasy svg --help).
// The generated file has no dependency on internal/svggen, XML, or CSS at
// runtime.
package enginesplash

// EngineSplashMinTime is the engine brand segment's minimum on-screen time.
// It matches the duration of the compiled brand animation (internal/
// enginesplash) so the segment holds until the animation completes, unless
// skipped.
const EngineSplashMinTime = 2.0

// EngineSplashFade cross-fades the engine segment in/out at its own edges
// (splashFadeAlpha), so the handoff to tick 0 lands on the same background
// (Config.Clear, painted unfaded behind every segment) instead of a hard cut.
const EngineSplashFade = 0.4
