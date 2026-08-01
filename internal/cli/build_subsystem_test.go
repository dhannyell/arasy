package cli

import (
	"debug/pe"
	"io"
	"path/filepath"
	"runtime"
	"testing"
)

// A Windows release links into the GUI subsystem so double-clicking the game
// does not flash a console window. Reading the subsystem back out of the PE
// header is the only check that does not depend on someone watching a screen.
func TestBuildNativeReleaseLinksIntoTheGUISubsystem(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the PE subsystem only exists on Windows")
	}
	module := writeThrowawayModule(t, "game")
	t.Chdir(module)

	for _, tc := range []struct {
		name    string
		console bool
		want    uint16
	}{
		{name: "release", console: false, want: pe.IMAGE_SUBSYSTEM_WINDOWS_GUI},
		{name: "console", console: true, want: pe.IMAGE_SUBSYSTEM_WINDOWS_CUI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()
			err := buildNative("./native", out, nativeBuildOptions{
				Release: true, Mode: nativeModePortable, Console: tc.console,
			}, io.Discard, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if got := peSubsystem(t, filepath.Join(out, nativeBinaryName("./native"))); got != tc.want {
				t.Fatalf("subsystem = %d, want %d", got, tc.want)
			}
		})
	}
}

func peSubsystem(t *testing.T, path string) uint16 {
	t.Helper()
	file, err := pe.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		return header.Subsystem
	case *pe.OptionalHeader32:
		return header.Subsystem
	default:
		t.Fatalf("%s has no optional header", path)
		return 0
	}
}
