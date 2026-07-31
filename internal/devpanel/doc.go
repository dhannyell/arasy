// Package devpanel is the engine's dev tooling overlay: pause, step, speed,
// scrubbing, debug layers and recording, built out of ui widgets and emitted
// as ordinary render commands.
//
// It is deliberately backend-free. There is no DOM here, no window handle and
// no syscall — the panel takes input events, drives a runtime.Driver through
// its public control surface, and returns render commands. Every host that
// can already draw a frame can therefore mount the same tooling, and the
// whole thing is exercised headlessly in go test.
//
// The one thing a host must supply is file I/O for recordings, which is what
// RecordingPort is for: a browser downloads a Blob, a desktop writes a file,
// and nothing else about the panel has to know which one it is talking to.
package devpanel
