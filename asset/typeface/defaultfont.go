//go:build !arr_no_default_font

package typeface

import (
	"sync"

	"golang.org/x/image/font/gofont/gomono"
)

var (
	defaultOnce sync.Once
	defaultFace *Face
)

// DefaultFace returns the engine's embedded default font (Go Mono, BSD-3),
// used to render FontID 0 when the game declares no font. It is nil in builds
// compiled with the arr_no_default_font tag, where FontID 0 falls back to the
// built-in bitmap font instead.
func DefaultFace() *Face {
	defaultOnce.Do(func() {
		f, err := Parse(gomono.TTF)
		if err == nil {
			defaultFace = f
		}
	})
	return defaultFace
}
