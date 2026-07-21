// Package renderbatch holds the canonical, backend-mirrorable helpers for
// retained sprite and text batches: defensive copies on intake, world
// bounds, flip-book/bounce/pulse animation evaluation, conservative culling
// inflation and view intersection.
//
// Raster, Canvas2D and the WGSL shaders must mirror these rules; the
// intermediate operations that feed the shader use float32 precision. The
// package is render-only and never enters the simulation path, so
// transcendentals are allowed here; batch content never enters checksums,
// snapshots, replays or recordings.
package renderbatch
