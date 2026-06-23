// Package raster renders Frameforge render commands into images using the Go
// standard library.
//
// This package is a CPU rasterizer. It converts backend-agnostic render
// commands, such as render.DrawRect, render.DrawCircle and render.DrawLine,
// into pixels in an image.RGBA buffer.
//
// The raster renderer is intended for simple PNG output, debug visualizations,
// golden-image style tests and headless rendering. It is deliberately small and
// dependency-free.
//
// It does not currently provide anti-aliasing, text rendering, GPU acceleration
// or real-time presentation. Future platform backends can translate the same
// render commands to Canvas, WebGPU, OpenGL or another rendering target.
package raster
