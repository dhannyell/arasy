//go:build arr_no_default_font

package typeface

// DefaultFace returns nil: the embedded default font is compiled out
// (arr_no_default_font). FontID 0 falls back to the built-in bitmap font.
func DefaultFace() *Face { return nil }
