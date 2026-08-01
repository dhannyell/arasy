//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// NewLogger builds the logger a distributed native game should run with. It
// writes to a file under the user's cache directory and, when the build still
// has a console, to stderr as well.
//
// The file is what makes a released build debuggable. A Windows release links
// into the GUI subsystem so double-clicking it does not flash a console
// window, and that also throws stdout and stderr away — without a file, the
// first thing a player's crash report loses is the reason for the crash.
//
// The log is one file, truncated at boot: the previous session's log is gone
// once a new one starts. Its absolute path is the first line written to it
// and is also printed to stdout, so "attach the file at this path" is an
// instruction a player can follow.
//
// Losing the cache directory is not worth failing a boot over: if the file
// cannot be opened, the logger falls back to stderr alone. The returned
// function closes the file and is safe to call even then.
//
//	log, closeLog := native.NewLogger("Breakout")
//	defer closeLog()
//	err := native.Run(&Breakout{}, native.Options{Title: "Breakout", Log: log})
func NewLogger(gameName string) (*slog.Logger, func() error) {
	file, path, err := openLogFile(gameName)
	if err != nil {
		log := slog.New(slog.NewTextHandler(os.Stderr, nil))
		log.Warn("game log file unavailable; diagnostics go to stderr only", "err", err)
		return log, func() error { return nil }
	}

	// The console is gone in a GUI-subsystem build, so a failed stderr write
	// must not stop the file from receiving the same line.
	log := slog.New(slog.NewTextHandler(logWriter{file: file, console: os.Stderr}, nil))
	log.Info("game log", "path", path)
	// A GUI-subsystem build has no stdout to print to, and that is the normal
	// case for a released game rather than a failure.
	_, _ = fmt.Fprintf(os.Stdout, "logging to %s\n", path)
	return log, file.Close
}

// openLogFile truncates and opens this session's log, and reports its
// absolute path.
func openLogFile(gameName string) (*os.File, string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	name := logDirName(gameName)
	dir := filepath.Join(cache, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log directory: %w", err)
	}
	path := filepath.Join(dir, name+".log")
	file, err := os.Create(path)
	if err != nil {
		return nil, "", fmt.Errorf("create log file: %w", err)
	}
	return file, path, nil
}

// logDirName turns a game title into a name every filesystem accepts.
func logDirName(gameName string) string {
	var out strings.Builder
	for _, r := range gameName {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			out.WriteRune(r)
		case r == ' ' || r == '.':
			out.WriteRune('-')
		}
	}
	name := strings.Trim(out.String(), "-")
	if name == "" {
		return "arasy-game"
	}
	return name
}

// logWriter mirrors every line to the console, best effort. Only the file
// write can fail the logger: a GUI-subsystem build has no console to write
// to, and that is the normal case for a released game, not an error.
type logWriter struct {
	file    io.Writer
	console io.Writer
}

func (w logWriter) Write(p []byte) (int, error) {
	if w.console != nil {
		_, _ = w.console.Write(p)
	}
	return w.file.Write(p)
}
