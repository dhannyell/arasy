package render

// MSAAMode selects the multisample antialiasing level a GPU host renders
// with. It is backend-agnostic vocabulary, not a command: hosts that
// multi-sample (the native and WebGPU renderers) honor it, and hosts that
// cannot (the software raster renderer, the Canvas2D fallback) ignore it.
type MSAAMode uint8

const (
	// MSAADefault is 4x, the level WebGPU guarantees on every adapter and the
	// quality/cost sweet spot for 2D geometry. It is the zero value, so a
	// caller that never picks a level keeps antialiasing.
	MSAADefault MSAAMode = iota
	// MSAAOff disables multi-sampling: no multisample target is allocated and
	// the pass renders straight into the surface. It is the cheapest fill
	// rate, and at high resolutions it frees a substantial amount of video
	// memory, at the cost of hard edges on circles and polygon silhouettes.
	MSAAOff
	// MSAA2x is half the default's samples: cheaper, visibly coarser. WebGPU
	// guarantees only 1 and 4 samples, so this level depends on the adapter and
	// is never available in a browser; a host that cannot provide it falls back
	// to the default and says so in the log.
	MSAA2x
	// MSAA4x is the default stated explicitly.
	MSAA4x
	// MSAA8x is the highest level. Like MSAA2x it is adapter-specific rather
	// than guaranteed, and falls back to the default where it is unavailable.
	MSAA8x
)

// Samples maps the mode to the sample count a pipeline and render target are
// built with. A mode outside the enum falls back to the default: an unknown
// value must never fail renderer creation. ok reports whether the mode was
// recognized, so a host can log the fallback.
func (m MSAAMode) Samples() (samples int, ok bool) {
	switch m {
	case MSAADefault, MSAA4x:
		return 4, true
	case MSAAOff:
		return 1, true
	case MSAA2x:
		return 2, true
	case MSAA8x:
		return 8, true
	default:
		return 4, false
	}
}
