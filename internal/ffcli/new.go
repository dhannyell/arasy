package ffcli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
)

//go:embed all:template
var projectTemplates embed.FS

type projectTemplateData struct {
	Module        string
	Name          string
	GoVersion     string
	EngineModule  string
	EngineVersion string
	EngineReplace string
}

var (
	newDir           string
	newName          string
	newEngineReplace string
)

var newCmd = &cobra.Command{
	Use:   "new <module-path>",
	Short: "create a minimal frameforge game project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		module := args[0]
		dir := newDir
		if dir == "" {
			dir = filepath.Base(module)
		}
		name := newName
		if name == "" {
			name = packageName(filepath.Base(module))
		}
		if name == "" {
			return fmt.Errorf("cannot derive a Go package name from %q", module)
		}
		if err := ensureDestinationEmpty(dir); err != nil {
			return err
		}
		data := projectTemplateData{
			Module: module, Name: name, GoVersion: strings.TrimPrefix(runtime.Version(), "go"),
			EngineModule: "github.com/dhannyell/arasy", EngineVersion: "v0.0.0", EngineReplace: newEngineReplace,
		}
		if err := renderProject(dir, data); err != nil {
			return err
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "created %s\nnext steps:\n  cd %s\n  go mod tidy\n  frameforge dev ./wasm\n", module, dir)
		return err
	},
}

func ensureDestinationEmpty(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination %q already exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("destination %q already exists and is not empty", dir)
	}
	return nil
}

func init() {
	newCmd.Flags().StringVarP(&newDir, "dir", "d", "", "destination directory (defaults to the module name)")
	newCmd.Flags().StringVar(&newName, "name", "", "Go package name (defaults to the sanitized module name)")
	newCmd.Flags().StringVar(&newEngineReplace, "engine-replace", "", "local frameforge repository path; required to compile while the engine is private")
	rootCmd.AddCommand(newCmd)
}

func renderProject(dir string, data projectTemplateData) error {
	files := map[string]string{
		"go.mod.tmpl":               "go.mod",
		"game.go.tmpl":              "game.go",
		"wasm/main.go.tmpl":         "wasm/main.go",
		"raster/main.go.tmpl":       "raster/main.go",
		"assets/manifest.json.tmpl": "assets/manifest.json",
		"gitignore.tmpl":            ".gitignore",
		"README.md.tmpl":            "README.md",
	}
	for source, destination := range files {
		body, err := projectTemplates.ReadFile("template/" + source)
		if err != nil {
			return err
		}
		tmpl, err := template.New(source).Parse(string(body))
		if err != nil {
			return fmt.Errorf("parse %s: %w", source, err)
		}
		path := filepath.Join(dir, filepath.FromSlash(destination))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		err = tmpl.Execute(file, data)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func packageName(value string) string {
	var out strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			out.WriteRune(unicode.ToLower(r))
		}
	}
	name := out.String()
	if name == "" {
		return ""
	}
	if first := rune(name[0]); unicode.IsDigit(first) {
		return "game" + name
	}
	return name
}
