// Package audio is the engine's backend-agnostic audio vocabulary, mirroring
// render and input: passive types and pure helpers with no knowledge of any
// backend. The engine never decodes or plays sound — platform hosts consume
// the Output delivered by runtime.Driver and drive the real audio device.
package audio
