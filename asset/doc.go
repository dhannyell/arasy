// Package asset declares game assets (textures, sprites, audio clips) loaded from a
// data-driven manifest, and resolves names to the stable per-run handles
// that flow through render commands. Decoding pixel data is not this
// package's job: native backends use asset/pixel, and the web canvas ships
// the encoded bytes to the browser.
package asset
