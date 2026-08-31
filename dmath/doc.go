// Package dmath provides reproducible transcendental functions for simulation.
// It uses only IEEE-754 operations with platform-independent results: +, -,
// *, /, and math.Sqrt.
//
// Reproducible does not mean exact. Results may differ from correctly rounded
// standard-library values, but equal inputs produce equal bits on every
// supported platform. This keeps replays and checksums stable.
//
// Simulation code must use dmath instead of the transcendental functions in
// math. The standard library delegates those functions to platform-specific
// libm implementations.
//
// dmath may also use math helpers that classify values or move their bits,
// such as Abs, IsNaN, IsInf, Signbit, Float64bits, Float64frombits, NaN, Inf,
// and math.Pi. These operations have one result for each input on every
// supported platform.
//
// The trigonometric functions delegate to the integer kernel of the
// github.com/dhannyell/fixed module. Integer operations carry the same
// reproducibility promise. Exp, Log, and Pow keep float bodies: their range
// exceeds the Q32.32 format.
package dmath
