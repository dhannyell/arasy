// Package loop provides fixed-step planning helpers used by the runtime.
//
// Steps converts elapsed render time into a number of fixed simulation steps,
// preserving leftover time in an accumulator and optionally clamping catch-up
// work. Hosts that use runtime.Driver should report elapsed time through
// Driver.AdvanceTime or call Driver.Step for deterministic tick-driven flows
// instead of consuming this package directly.
package loop
