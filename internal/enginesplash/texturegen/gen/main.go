// Command gen writes the boot splash's particle sprites to
// internal/enginesplash/assets, once, so internal/enginesplash can embed
// them directly instead of rasterizing on every process start. Run it with:
//
//	go run ./internal/enginesplash/texturegen/gen
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dhannyell/arasy/internal/enginesplash/texturegen"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dir := filepath.Join("internal", "enginesplash", "assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	files := map[string][]byte{
		"mote.png":  texturegen.Mote(),
		"ember.png": texturegen.Ember(),
	}
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Println("wrote", path)
	}
	return nil
}
