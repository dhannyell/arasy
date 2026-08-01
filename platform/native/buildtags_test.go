//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The reach of the native host is one boolean expression repeated in every
// file of this package, and Go has no alias for a build tag. This test is
// what keeps the copies from drifting: it reads the package's own sources,
// insists they all carry the same expression, and insists unsupported.go
// covers exactly the platforms they do not. Without it, adding a file with
// the wrong tag produces a duplicate symbol or an empty package on a platform
// nobody compiles locally.
//
// A file may narrow the shared expression with the arr_dev term — the dev
// panel only exists in dev builds — but may not widen it or vary it any
// other way.

// unsupportedFile holds the fallback declarations for platforms outside the
// host's reach, and therefore carries the complement of the shared tag.
const unsupportedFile = "unsupported.go"

// hostTargets is the table the constraints are evaluated against: every pair
// the project cares about, in and out of reach.
var hostTargets = []struct{ goos, goarch string }{
	{"windows", "amd64"}, {"windows", "arm64"}, {"windows", "386"},
	{"linux", "amd64"}, {"linux", "arm64"}, {"linux", "arm"},
	{"darwin", "amd64"}, {"darwin", "arm64"},
	{"js", "wasm"}, {"freebsd", "amd64"}, {"android", "arm64"},
}

func TestBuildTagsAreIdenticalAcrossThePackage(t *testing.T) {
	files := packageConstraints(t)
	if len(files) == 0 {
		t.Fatal("no Go files found in the package directory")
	}

	shared := ""
	for _, name := range sortedNames(files) {
		if name == unsupportedFile {
			continue
		}
		line := files[name]
		if line == "" {
			t.Fatalf("%s has no //go:build line; every file in this package must declare the host's reach", name)
		}
		base := strings.TrimSuffix(strings.TrimSuffix(line, " && arr_dev"), " && !arr_dev")
		if shared == "" {
			shared = base
			continue
		}
		if base != shared {
			t.Fatalf("%s constrains %q, but the package constrains %q", name, base, shared)
		}
	}
}

func TestBuildTagsUnsupportedFileIsTheExactComplement(t *testing.T) {
	files := packageConstraints(t)
	unsupported, ok := files[unsupportedFile]
	if !ok {
		t.Skipf("%s does not exist yet; the package has no out-of-reach fallback", unsupportedFile)
	}

	var supported string
	for name, line := range files {
		if name != unsupportedFile && !strings.Contains(line, "arr_dev") {
			supported = line
			break
		}
	}
	if supported == "" {
		t.Fatal("no unconditional host file to compare against")
	}

	supportedExpr := parseConstraint(t, supported)
	unsupportedExpr := parseConstraint(t, unsupported)
	for _, target := range hostTargets {
		in := evalConstraint(supportedExpr, target.goos, target.goarch)
		out := evalConstraint(unsupportedExpr, target.goos, target.goarch)
		if in == out {
			t.Fatalf("%s/%s compiles %d sides of the package, want exactly one", target.goos, target.goarch, boolToInt(in)+boolToInt(out))
		}
	}
}

// packageConstraints maps every Go file in the package directory to its
// //go:build expression, or to the empty string when it has none.
func packageConstraints(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = buildLine(string(body))
	}
	return files
}

// buildLine returns the //go:build expression of a source file, without the
// directive prefix, or "" when the file has none.
func buildLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "//go:build "); ok {
			return strings.TrimSpace(after)
		}
		if strings.HasPrefix(line, "package ") {
			return ""
		}
	}
	return ""
}

func parseConstraint(t *testing.T, line string) constraint.Expr {
	t.Helper()
	expr, err := constraint.Parse("//go:build " + line)
	if err != nil {
		t.Fatalf("parse %q: %v", line, err)
	}
	return expr
}

// evalConstraint answers the expression for one GOOS/GOARCH pair, with cgo
// assumed on: the native host needs it, so a build without cgo is a build
// that never reaches this package.
func evalConstraint(expr constraint.Expr, goos, goarch string) bool {
	unix := goos == "linux" || goos == "darwin" || goos == "freebsd" || goos == "android"
	return expr.Eval(func(tag string) bool {
		switch tag {
		case goos, goarch, "cgo":
			return true
		case "unix":
			return unix
		default:
			return false
		}
	})
}

// sortedNames keeps failure messages deterministic.
func sortedNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
