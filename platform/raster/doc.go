// Package raster renders Arasy render commands into images using the Go
// standard library.
//
// It converts backend-agnostic render commands into pixels in an image.RGBA
// buffer. The package is intended for PNG/GIF output, debug visualization,
// tests and headless rendering.
//
// The renderer is deliberately small. It does not provide antialiasing, GPU
// acceleration or real-time presentation.
package raster
