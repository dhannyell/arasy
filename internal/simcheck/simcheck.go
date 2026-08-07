// Package simcheck finds architecture-dependent math calls in simulation code.
// The standard library delegates transcendentals to platform libm
// implementations, so their bits can vary by architecture and break replay,
// rewind, and checksums. Simulation code must use dmath instead.
//
// A package is treated as simulation code when it imports scene or runtime,
// directly or transitively. A file opts out with the //arasy:render directive
// before its package clause. Render-only code may use math because presentation
// state never enters snapshots, checksums, or replay.
package simcheck

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"sort"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// RenderDirective marks a file as render-only, exempting it from the
// check. It must appear before the package clause, like a build tag.
const RenderDirective = "//arasy:render"

// engineModule is the engine's own module path. Check skips its packages:
// the repository guards itself with forbidigo at file granularity, with
// deliberate exemptions for render-only files and platform backends, and
// a second exemption map here would drift from the first. In a user
// module the engine is a dependency, never the main module, so the skip
// changes nothing for the audience this check exists for.
const engineModule = "github.com/dhannyell/arasy"

// engineRoots lists the engine packages that identify simulation code.
var engineRoots = []string{
	engineModule + "/scene",
	engineModule + "/runtime",
}

// forbidden lists math functions whose results can vary by architecture. It
// mirrors the forbidigo pattern in .golangci.yml. Mod and Modf are allowed by
// the project's deterministic remainder policy.
var forbidden = map[string]bool{
	"Sin": true, "Cos": true, "Tan": true,
	"Asin": true, "Acos": true, "Atan": true, "Atan2": true,
	"Exp": true, "Pow": true,
	"Log": true, "Log2": true, "Log10": true,
}

// inDmath marks the forbidden functions that have a dmath replacement,
// so the diagnostic can point at the exact substitute.
var inDmath = map[string]bool{
	"Sin": true, "Cos": true, "Asin": true, "Acos": true,
	"Atan2": true, "Exp": true, "Pow": true, "Log": true,
}

// New returns the analyzer. When roots are omitted, it uses engineRoots.
// Tests can pass alternate roots to build a smaller import graph.
func New(roots ...string) *analysis.Analyzer {
	if len(roots) == 0 {
		roots = engineRoots
	}
	return &analysis.Analyzer{
		Name: "simcheck",
		Doc:  "reports architecture-dependent math calls in simulation code",
		Run: func(pass *analysis.Pass) (any, error) {
			return run(pass, roots)
		},
	}
}

func run(pass *analysis.Pass, roots []string) (any, error) {
	if !importsAny(pass.Pkg, roots) {
		return nil, nil
	}
	for _, file := range pass.Files {
		if isRenderFile(file) {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			obj := pass.TypesInfo.ObjectOf(sel.Sel)
			if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "math" {
				return true
			}
			name := obj.Name()
			if !forbidden[name] {
				return true
			}
			substitute := "Build it from the dmath primitives or move it out of the simulation path."
			if inDmath[name] {
				substitute = fmt.Sprintf("Use dmath.%s instead.", name)
			}
			pass.Reportf(sel.Pos(),
				"math.%s is not reproducible across architectures and breaks replay and rollback. %s If this code only runs in Render and never touches simulation state, mark the file with the %s directive.",
				name, substitute, RenderDirective)
			return true
		})
	}
	return nil, nil
}

// isRenderFile reports whether the file carries the render directive
// before its package clause.
func isRenderFile(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, comment := range group.List {
			if comment.Text == RenderDirective {
				return true
			}
		}
	}
	return false
}

// importsAny reports whether pkg imports one of the roots, directly or
// indirectly.
func importsAny(pkg *types.Package, roots []string) bool {
	rootSet := make(map[string]bool, len(roots))
	for _, r := range roots {
		rootSet[r] = true
	}
	seen := make(map[*types.Package]bool)
	var walk func(p *types.Package) bool
	walk = func(p *types.Package) bool {
		if seen[p] {
			return false
		}
		seen[p] = true
		if rootSet[p.Path()] {
			return true
		}
		return slices.ContainsFunc(p.Imports(), walk)
	}
	return slices.ContainsFunc(pkg.Imports(), walk)
}

// Check loads the target and every package in its main module, runs the
// analyzer, and returns sorted diagnostics. Packages with load errors produce
// no findings so the caller can report the compiler's own error.
func Check(dir, pattern string) ([]string, error) {
	list, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:  dir,
	}, pattern)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", pattern, err)
	}
	paths := modulePackagePaths(list)
	if len(paths) == 0 {
		return nil, nil
	}

	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedTypesInfo | packages.NeedSyntax,
		Dir: dir,
	}, paths...)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", pattern, err)
	}

	analyzer := New()
	var findings []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 || pkg.Types == nil {
			return nil, nil
		}
		pass := &analysis.Pass{
			Analyzer:   analyzer,
			Fset:       pkg.Fset,
			Files:      pkg.Syntax,
			Pkg:        pkg.Types,
			TypesInfo:  pkg.TypesInfo,
			TypesSizes: pkg.TypesSizes,
			Report: func(d analysis.Diagnostic) {
				findings = append(findings, fmt.Sprintf("%s: %s", pkg.Fset.Position(d.Pos), d.Message))
			},
			ResultOf: map[*analysis.Analyzer]any{},
		}
		if _, err := analyzer.Run(pass); err != nil {
			return nil, fmt.Errorf("analyze %s: %w", pkg.PkgPath, err)
		}
	}
	sort.Strings(findings)
	return findings, nil
}

// modulePackagePaths collects main-module packages from the complete import
// graph of the loaded roots. The build target is usually a thin main package;
// the game code it guards lives in the packages that target imports.
func modulePackagePaths(roots []*packages.Package) []string {
	seen := make(map[string]bool)
	var paths []string
	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		if seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		if len(p.Errors) > 0 {
			return
		}
		if p.Module != nil && p.Module.Main && p.Module.Path != engineModule {
			paths = append(paths, p.PkgPath)
		}
		for _, imp := range p.Imports {
			walk(imp)
		}
	}
	for _, p := range roots {
		walk(p)
	}
	sort.Strings(paths)
	return paths
}
