//go:build windows && amd64

package native

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// recordingDir is where the native host keeps .arrp files, relative to the
// working directory the game was started from. A fixed directory rather than
// a file dialog: a dialog is a platform toolkit dependency, and the flow this
// serves — record, quit, relaunch, load the last one — does not need one.
const recordingDir = "recordings"

// recordingPort is the desktop half of the dev panel's recording I/O. Save
// writes a timestamped file so a session's recordings accumulate instead of
// overwriting each other; Load opens the most recent one.
type recordingPort struct {
	dir string
}

func newRecordingPort() *recordingPort {
	return &recordingPort{dir: recordingDir}
}

func (p *recordingPort) Save(name string, data []byte) error {
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", p.dir, err)
	}
	path := filepath.Join(p.dir, timestampedName(name, time.Now()))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Load reads the newest recording in the directory. An empty or missing
// directory is not an error — there is simply nothing to load yet.
func (p *recordingPort) Load() ([]byte, string, bool, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", false, nil
		}
		return nil, "", false, fmt.Errorf("read %s: %w", p.dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".arrp") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return nil, "", false, nil
	}

	// Names carry a sortable timestamp, so the newest is the last one in
	// lexical order — no stat call per file, and no dependence on a
	// filesystem's modification-time resolution.
	sort.Strings(names)
	name := names[len(names)-1]

	data, err := os.ReadFile(filepath.Join(p.dir, name))
	if err != nil {
		return nil, "", false, fmt.Errorf("read %s: %w", name, err)
	}
	return data, name, true, nil
}

// timestampedName inserts a sortable timestamp before the extension, so two
// recordings from one session do not collide and the newest sorts last.
func timestampedName(name string, now time.Time) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return base + "-" + now.Format("20060102-150405") + ext
}
