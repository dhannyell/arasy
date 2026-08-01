//go:build (windows || linux || darwin) && (amd64 || arm64)

// Package native implements the desktop host over wgpu-native and GLFW using
// the shared GPU core. It covers windows, linux and darwin on amd64 and
// arm64; see docs/distribution.md for what each of those tiers promises.
package native
