package cli

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/dhannyell/arasy/internal/wasmserve"
)

// devRebuilder skips no-op rebuilds in `arasy dev`: it fingerprints the
// module's source tree and only invokes go build when something moved since
// the last attempt. Combined with buildWASM's write-only-if-changed swap,
// reloading the browser without any edit costs a stat-only walk plus a 304.
type devRebuilder struct {
	pkg, dir, title string
	boot            wasmserve.BootOptions

	attempted   bool
	fingerprint string
	buildErr    error
}

// rebuild replays the last build result when the source fingerprint is
// unchanged — including a previous failure, which a reload alone cannot fix.
// A fingerprint walk error never blocks a build attempt.
func (r *devRebuilder) rebuild() error {
	fingerprint, err := fingerprintSources(".")
	if err == nil {
		if r.attempted && fingerprint == r.fingerprint {
			return r.buildErr
		}
		r.fingerprint = fingerprint
	}
	r.attempted = true
	r.buildErr = rebuildWASM(r.pkg, r.dir, r.title, r.boot)
	return r.buildErr
}

// fingerprintSources hashes the identity (path, size, mtime) of every file
// under root, skipping directories that never feed a build. Every extension
// counts — //go:embed can pull any file into the binary, so a changed PNG
// must invalidate the fingerprint exactly like a changed .go file.
func fingerprintSources(root string) (string, error) {
	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"dist":         true,
		"output":       true,
		"test-results": true,
		"tmp_preview":  true,
	}
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if skipDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		// hash.Write never fails.
		_, _ = hash.Write([]byte(path))
		var binaryFields [16]byte
		binary.LittleEndian.PutUint64(binaryFields[:8], uint64(info.Size()))
		binary.LittleEndian.PutUint64(binaryFields[8:], uint64(info.ModTime().UnixNano()))
		_, _ = hash.Write(binaryFields[:])
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint sources: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
