package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
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
	Short: "create a minimal arasy game project",
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
		mainVersion := ""
		if info, ok := debug.ReadBuildInfo(); ok {
			mainVersion = info.Main.Version
		}
		version, err := engineVersion(mainVersion, newEngineReplace)
		if err != nil {
			return err
		}
		if err := ensureDestinationEmpty(dir); err != nil {
			return err
		}
		data := projectTemplateData{
			Module: module, Name: name, GoVersion: strings.TrimPrefix(runtime.Version(), "go"),
			EngineModule: "github.com/dhannyell/arasy", EngineVersion: version, EngineReplace: newEngineReplace,
		}
		if err := renderProject(dir, data); err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(),
			"created %s\nnext steps:\n  cd %s\n  go mod tidy\n  arasy dev ./wasm     # browser, rebuilds on reload\n  arasy run ./native   # native window\n",
			module, dir)
		return err
	},
}

// engineVersion picks the engine version that a generated go.mod requires.
// A CLI installed with `go install ...@version` carries that version in its
// build info. A local build carries "(devel)" or a "+dirty" pseudo-version,
// which no module proxy can resolve, so it needs a replace instead.
func engineVersion(mainVersion, replace string) (string, error) {
	if replace != "" {
		// The replace decides the source; the version only has to parse.
		return "v0.0.0", nil
	}
	if strings.HasPrefix(mainVersion, "v") && !strings.Contains(mainVersion, "+") {
		return mainVersion, nil
	}
	return "", fmt.Errorf("this arasy CLI is a development build (version %q); "+
		"pass --engine-replace <path to the arasy repository>, "+
		"or install a release with: go install github.com/dhannyell/arasy/cmd/arasy@latest", mainVersion)
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
	newCmd.Flags().StringVar(&newEngineReplace, "engine-replace", "", "local arasy repository path; required when this CLI is a development build")
	rootCmd.AddCommand(newCmd)
}

func renderProject(dir string, data projectTemplateData) error {
	files := map[string]string{
		"go.mod.tmpl":               "go.mod",
		"game.go.tmpl":              "game.go",
		"wasm/main.go.tmpl":         "wasm/main.go",
		"native/main.go.tmpl":       "native/main.go",
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
