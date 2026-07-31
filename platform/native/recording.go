//go:build windows && amd64 && arr_dev

package native

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhannyell/arasy/internal/devpanel"
)

// recordingDir is where the native host keeps .arrp files, relative to the
// working directory the game was started from. A fixed directory rather than
// a file dialog: a dialog is a platform toolkit dependency, and the flow this
// serves — record, quit, relaunch, load the last one — does not need one.
const recordingDir = "recordings"

// recordingFileName is the base name a recording is saved under, before the
// timestamp is inserted. It matches what the panel asks for.
const recordingFileName = "arasy-replay.arrp"

// recordingPort is the desktop half of the dev panel's recording I/O. Save
// writes a timestamped file so a session's recordings accumulate instead of
// overwriting each other, and the panel lists them to pick from.
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

// Load reads the newest recording in the directory. It is the fallback for a
// panel that has not picked one; the dev panel lists them and asks for a name
// instead. An empty or missing directory is not an error — there is simply
// nothing to load yet.
func (p *recordingPort) Load() ([]byte, string, bool, error) {
	names, err := p.names()
	if err != nil || len(names) == 0 {
		return nil, "", false, err
	}

	name := names[len(names)-1]
	data, err := p.LoadNamed(name)
	if err != nil {
		return nil, "", false, err
	}
	return data, name, true, nil
}

// List enumerates the saved recordings, oldest first. The label drops the
// shared prefix and the extension: in a list of nothing but recordings, the
// timestamp is the only part that distinguishes one from another.
func (p *recordingPort) List() ([]devpanel.RecordingEntry, error) {
	names, err := p.names()
	if err != nil {
		return nil, err
	}

	entries := make([]devpanel.RecordingEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, devpanel.RecordingEntry{
			Name:  name,
			Label: recordingLabel(name),
		})
	}
	return entries, nil
}

func (p *recordingPort) LoadNamed(name string) ([]byte, error) {
	// The name came from List, but it reaches us as a string and ends up in a
	// path, so it is checked rather than trusted.
	if name != filepath.Base(name) {
		return nil, fmt.Errorf("invalid recording name %q", name)
	}
	data, err := os.ReadFile(filepath.Join(p.dir, name))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return data, nil
}

// names lists the recording files in lexical order. Names carry a sortable
// timestamp, so that order is chronological — no stat call per file, and no
// dependence on a filesystem's modification-time resolution.
func (p *recordingPort) names() ([]string, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", p.dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".arrp") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// recordingLabel shortens a file name for the panel's picker.
func recordingLabel(name string) string {
	label := strings.TrimSuffix(name, filepath.Ext(name))
	return strings.TrimPrefix(label, strings.TrimSuffix(recordingFileName, ".arrp")+"-")
}

// timestampedName inserts a sortable timestamp before the extension, so two
// recordings from one session do not collide and the newest sorts last.
func timestampedName(name string, now time.Time) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return base + "-" + now.Format("20060102-150405") + ext
}
