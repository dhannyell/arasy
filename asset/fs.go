package asset

import (
	"errors"
	"io/fs"
)

// layeredFS merges FS layers with last-layer-wins semantics: Open tries
// layers from last to first and returns the first hit. This mirrors
// readLayered/statLayered so a path resolves identically whether it was read
// eagerly during Load or lazily by a host through Manifest.FS().
//
// A nil or empty layers slice is safe to use: every Open call reports
// fs.ErrNotExist, matching the zero-value Manifest.
type layeredFS struct {
	layers []fs.FS
}

func (l layeredFS) Open(name string) (fs.File, error) {
	for i := len(l.layers) - 1; i >= 0; i-- {
		f, err := l.layers[i].Open(name)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
