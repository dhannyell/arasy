// Package simcheck finds determinism hazards in simulation code:
//
//   - architecture-dependent math calls: the standard library delegates
//     transcendentals to platform libm implementations, so their bits can
//     vary by architecture; simulation code must use dmath instead;
//   - map iteration: Go deliberately randomizes map order, so a loop whose
//     result depends on it cannot reproduce reliably;
//   - randomness outside ctx.Rand (math/rand, math/rand/v2, crypto/rand);
//   - wall-clock reads (time.Now, time.Since, time.Until): simulation time
//     is the fixed tick.
//
// Every hazard breaks replay, rewind, and checksums.
//
// A package is treated as simulation code when it imports scene or runtime,
// directly or transitively. A file opts out with the //arasy:render directive
// before its package clause. Render-only code is exempt because presentation
// state never enters snapshots, checksums, or replay. A single map loop that
// is provably independent of iteration order opts out with the
// //arasy:order-insensitive directive on the comment line above the for
// statement; the directive is per loop, not per file, because order
// insensitivity is a property of one loop body.
package simcheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

// RenderDirective marks a file as render-only, exempting it from the
// check. It must appear before the package clause, like a build tag.
const RenderDirective = "//arasy:render"

// OrderInsensitiveDirective suppresses the map-iteration check for the range
// statement directly below it. Add the reason after the directive.
const OrderInsensitiveDirective = "//arasy:order-insensitive"

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

// randPackages lists random sources that bypass the recorded stream.
// ctx.Rand is the only randomness that replay can reproduce.
var randPackages = map[string]bool{
	"math/rand":    true,
	"math/rand/v2": true,
	"crypto/rand":  true,
}

// clockFuncs lists the wall-clock reads. The time package remains allowed:
// time.Duration in an API is legitimate.
var clockFuncs = map[string]bool{
	"Now": true, "Since": true, "Until": true,
}

// checkSet selects which hazard checks a pass runs. The math check is
// separate because forbidigo owns it inside this repository. The analyzer
// keeps the remaining checks because import rules cannot express all of them.
type checkSet struct {
	math  bool
	order bool
	rand  bool
	clock bool
}

var allChecks = checkSet{math: true, order: true, rand: true, clock: true}

// New returns the analyzer with every check enabled. When roots are
// omitted, it uses engineRoots. Tests can pass alternate roots to build a
// smaller import graph.
func New(roots ...string) *analysis.Analyzer {
	return newAnalyzer(allChecks, roots...)
}

func newAnalyzer(set checkSet, roots ...string) *analysis.Analyzer {
	if len(roots) == 0 {
		roots = engineRoots
	}
	return &analysis.Analyzer{
		Name: "simcheck",
		Doc:  "reports determinism hazards in simulation code",
		Run: func(pass *analysis.Pass) (any, error) {
			return run(pass, roots, set)
		},
	}
}

func run(pass *analysis.Pass, roots []string, set checkSet) (any, error) {
	if !isSimulationPackage(pass.Pkg, roots) {
		return nil, nil
	}
	for _, file := range pass.Files {
		if isRenderFile(file) {
			continue
		}
		checkFile(pass, file, set)
	}
	return nil, nil
}

func checkFile(pass *analysis.Pass, file *ast.File, set checkSet) {
	if set.rand {
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if !randPackages[path] {
				continue
			}
			pass.Reportf(spec.Pos(),
				"%s bypasses the recorded random stream and breaks replay and rollback. Use ctx.Rand in simulation code. If this file only runs in Render and never touches simulation state, mark it with the %s directive.",
				path, RenderDirective)
		}
	}
	var suppressed map[int]bool
	if set.order {
		suppressed = orderInsensitiveLines(pass.Fset, file)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.RangeStmt:
			if !set.order {
				return true
			}
			typ := pass.TypesInfo.TypeOf(node.X)
			if typ == nil {
				return true
			}
			if _, isMap := typ.Underlying().(*types.Map); !isMap {
				return true
			}
			if suppressed[pass.Fset.Position(node.Pos()).Line-1] {
				return true
			}
			pass.Reportf(node.Pos(),
				"map iteration order is deliberately random in Go and breaks replay and rollback when it reaches simulation results. Iterate an ordered slice of keys instead (see the component store pattern), or mark this statement with %s if the loop body is provably independent of iteration order (pure membership work, commutative writes).",
				OrderInsensitiveDirective)
		case *ast.SelectorExpr:
			obj := pass.TypesInfo.ObjectOf(node.Sel)
			if obj == nil || obj.Pkg() == nil {
				return true
			}
			name := obj.Name()
			switch obj.Pkg().Path() {
			case "math":
				if !set.math || !forbidden[name] {
					return true
				}
				substitute := "Build it from the dmath primitives or move it out of the simulation path."
				if inDmath[name] {
					substitute = fmt.Sprintf("Use dmath.%s instead.", name)
				}
				pass.Reportf(node.Pos(),
					"math.%s is not reproducible across architectures and breaks replay and rollback. %s If this code only runs in Render and never touches simulation state, mark the file with the %s directive.",
					name, substitute, RenderDirective)
			case "time":
				if !set.clock || !clockFuncs[name] {
					return true
				}
				pass.Reportf(node.Pos(),
					"time.%s reads the wall clock and breaks replay and rollback. Simulation time is the fixed tick; presentation time arrives through ctx.Time. If this code only runs in Render and never touches simulation state, mark the file with the %s directive.",
					name, RenderDirective)
			}
		}
		return true
	})
}

// orderInsensitiveLines maps the line number where a directive comment
// group ends to true. A range statement is suppressed when such a group
// ends on the line directly above it.
func orderInsensitiveLines(fset *token.FileSet, file *ast.File) map[int]bool {
	var lines map[int]bool
	for _, group := range file.Comments {
		for _, comment := range group.List {
			rest, ok := strings.CutPrefix(comment.Text, OrderInsensitiveDirective)
			if !ok || (rest != "" && rest[0] != ':' && rest[0] != ' ') {
				continue
			}
			if lines == nil {
				lines = make(map[int]bool)
			}
			lines[fset.Position(group.End()).Line] = true
		}
	}
	return lines
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

// isSimulationPackage reports whether pkg is on the simulation path: it is
// one of the roots, or it imports one of them. Root inclusion matters for
// the engine self-scan, where scene and runtime are scanned directly.
func isSimulationPackage(pkg *types.Package, roots []string) bool {
	if slices.Contains(roots, pkg.Path()) {
		return true
	}
	return importsAny(pkg, roots)
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

// CheckEngine checks the repository's simulation packages for map iteration,
// external randomness, and wall-clock reads, returning sorted findings. It
// leaves the transcendental check to forbidigo. Unlike Check, it fails when
// a package cannot be loaded, so the self-scan cannot silently skip code.
func CheckEngine(dir string, patterns ...string) ([]string, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedTypesInfo | packages.NeedSyntax,
		Dir: dir,
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load %v: %w", patterns, err)
	}
	analyzer := newAnalyzer(checkSet{order: true, rand: true, clock: true})
	var findings []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load %s: %v", pkg.PkgPath, pkg.Errors[0])
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
