package audioassets

import (
	"embed"
	"io/fs"
)

//go:embed manifest.json *.wav
var files embed.FS

// FS returns the Breakout demo audio manifest and files.
func FS() fs.FS {
	return files
}
