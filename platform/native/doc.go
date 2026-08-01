//go:build (windows || linux || darwin) && (amd64 || arm64)

// Package native implements the desktop host over wgpu-native and GLFW using
// the shared GPU core. It covers windows, linux and darwin on amd64 and
// arm64; see docs/distribution-en.md for what each tier promises.
package native
