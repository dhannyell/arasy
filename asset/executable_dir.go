package asset

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ExecutableDirFS returns the named directory next to the running executable
// as an FS layer, and reports whether it exists. Games pass it as the last
// layer to Load so a bundled install can override the assets compiled into
// the binary:
//
//	layers := []fs.FS{embedded}
//	if external, ok := asset.ExecutableDirFS("assets"); ok {
//		layers = append(layers, external)
//	}
//	manifest, err := asset.Load(layers...)
//
// A missing directory is not an error: a portable build ships a single
// executable with nothing beside it, and reports false.
//
// The directory is resolved from the executable's own path, never from the
// working directory: a game launched by a store client or a desktop shortcut
// starts in a directory that has nothing to do with where it was installed.
func ExecutableDirFS(name string) (fs.FS, bool) {
	exe, err := os.Executable()
	if err != nil {
		return nil, false
	}
	return dirFSNextTo(exe, name)
}

// dirFSNextTo resolves name beside the given executable path. Symlinks are
// followed first: a launcher may invoke the game through a link, and the
// assets sit beside the real binary, not beside the link.
func dirFSNextTo(exe, name string) (fs.FS, bool) {
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	path := filepath.Join(filepath.Dir(exe), name)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil, false
	}
	return os.DirFS(path), true
}
