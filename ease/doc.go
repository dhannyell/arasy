// Package ease provides easing curves for tween interpolation.
//
// Easing functions map a linear progress value in [0, 1] to a non-linear
// output, creating acceleration or deceleration effects. The package supplies
// a small set of named curves and two pure functions: Progress converts
// elapsed time and duration into a normalized value, and Apply shapes that
// value using the chosen curve.
//
// The ease package is consumed by the scene animation system. Games rarely
// call it directly.
package ease
