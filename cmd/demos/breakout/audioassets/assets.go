package audioassets

import (
	"embed"
	"io/fs"
)

//go:embed *.wav
var files embed.FS

// FS returns the Breakout demo audio files. The runtime AudioClips paths are
// relative to this filesystem.
func FS() fs.FS {
	return files
}
