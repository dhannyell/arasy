package cli

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/internal/wasmserve"
)

func TestWasmBuildArgsNoDefaultFont(t *testing.T) {
	withFlag := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, true, false), " ")
	if !strings.Contains(withFlag, "-tags arr_no_default_font") {
		t.Fatalf("args = %q, want -tags arr_no_default_font", withFlag)
	}
	withoutFlag := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, false, false), " ")
	if strings.Contains(withoutFlag, "-tags") {
		t.Fatalf("args = %q, want no -tags when the flag is off", withoutFlag)
	}
}

func TestWasmBuildArgsDevTools(t *testing.T) {
	dev := strings.Join(wasmBuildArgs("pkg/wasm", "dist", false, false, true), " ")
	if !strings.Contains(dev, "-tags arr_dev") {
		t.Fatalf("dev args = %q, want -tags arr_dev", dev)
	}
	combined := strings.Join(wasmBuildArgs("pkg/wasm", "dist", false, true, true), " ")
	if !strings.Contains(combined, "-tags arr_no_default_font,arr_dev") {
		t.Fatalf("combined args = %q, want both tags in one -tags flag", combined)
	}
	release := strings.Join(wasmBuildArgs("pkg/wasm", "dist", true, false, false), " ")
	if strings.Contains(release, "arr_dev") {
		t.Fatalf("release args = %q, must never carry the dev panel tag", release)
	}
}

func TestBuildWASMProducesStaticArtifacts(t *testing.T) {
	out := filepath.Join(t.TempDir(), "build")
	pkg := filepath.Join(repoRoot(t), "examples", "breakout", "wasm")
	if err := buildWASM(pkg, out, "Breakout", wasmBuildOptions{Release: true}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"app.wasm", "index.html", "wasm_exec.js",
		wasmserve.PosterLandscapeFile, wasmserve.PosterStackedFile,
	} {
		info, err := os.Stat(filepath.Join(out, name))
		if err != nil || info.Size() == 0 {
			t.Fatalf("artifact %s missing or empty: %v", name, err)
		}
	}
	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(index)
	if !strings.Contains(html, "engine: true") || !strings.Contains(html, "game: true") {
		t.Fatal("static build does not use the default web splash policy")
	}
}

func TestSupportedPlatformRejectsUnknownPlatform(t *testing.T) {
	if err := supportedPlatform("playstation"); err == nil {
		t.Fatal("supportedPlatform(playstation) succeeded")
	}
	for _, platform := range []string{platformWASM, platformNative} {
		if err := supportedPlatform(platform); err != nil {
			t.Fatalf("supportedPlatform(%s) = %v, want nil", platform, err)
		}
	}
}

func TestNativeBuildArgsCarryTags(t *testing.T) {
	release := strings.Join(nativeBuildArgs("./native", "out", nativeBuildOptions{Release: true, Mode: nativeModePortable}), " ")
	if !strings.Contains(release, "-ldflags "+nativeLDFlags(false)) {
		t.Fatalf("release args = %q, want stripped ldflags", release)
	}
	if !strings.HasSuffix(nativeLDFlags(true), "-s -w") || strings.Contains(nativeLDFlags(true), "windowsgui") {
		t.Fatalf("--console ldflags = %q, want no GUI subsystem", nativeLDFlags(true))
	}
	if strings.Contains(release, "-tags") {
		t.Fatalf("release args = %q, want no build tags", release)
	}
	dev := strings.Join(nativeBuildArgs("./native", "out", nativeBuildOptions{DevTools: true, NoDefaultFont: true}), " ")
	if !strings.Contains(dev, "-tags arr_no_default_font,arr_dev") {
		t.Fatalf("dev args = %q, want both tags in one -tags flag", dev)
	}
	if strings.Contains(dev, "-ldflags") {
		t.Fatalf("dev args = %q, want no release ldflags", dev)
	}
}

func TestNativeNameSkipsTheHostDirectory(t *testing.T) {
	for pkg, want := range map[string]string{
		"./examples/breakout/native": "breakout",
		"examples/breakout":          "breakout",
	} {
		if got := nativeName(pkg); got != want {
			t.Fatalf("nativeName(%q) = %q, want %q", pkg, got, want)
		}
	}

	// Run from a project root: "./native" names the project, not the host
	// directory it lives in.
	project := filepath.Join(t.TempDir(), "my-game")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	for _, pkg := range []string{"./native", "."} {
		if got := nativeName(pkg); got != "my-game" {
			t.Fatalf("nativeName(%q) = %q, want my-game", pkg, got)
		}
	}
}

func TestNativeBuildOptionsValidate(t *testing.T) {
	if err := (nativeBuildOptions{Mode: nativeModeBundle}).validate(); err == nil {
		t.Fatal("bundle without --assets was accepted")
	}
	if err := (nativeBuildOptions{Mode: nativeModePortable, AssetsDir: "assets"}).validate(); err == nil {
		t.Fatal("portable with --assets was accepted")
	}
	if err := (nativeBuildOptions{Mode: "installer"}).validate(); err == nil {
		t.Fatal("unknown mode was accepted")
	}
	if err := (nativeBuildOptions{Mode: nativeModePortable}).validate(); err != nil {
		t.Fatalf("portable = %v, want nil", err)
	}
}

// TestBuildNativeBundleProducesExecutableAndAssets exercises the build
// machinery against a throwaway module: it must run on every CI host,
// including the ones without the native host's C toolchain.
func TestBuildNativeBundleProducesExecutableAndAssets(t *testing.T) {
	module := writeThrowawayModule(t, "game")
	assets := filepath.Join(t.TempDir(), "assets")
	if err := os.MkdirAll(filepath.Join(assets, "audio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "audio", "theme.wav"), []byte("bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "dist")
	t.Chdir(module)
	err := buildNative("./native", out, nativeBuildOptions{
		Release: true, Mode: nativeModeBundle, AssetsDir: assets,
	}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{nativeBinaryName("./native"), filepath.Join("assets", "audio", "theme.wav")} {
		info, statErr := os.Stat(filepath.Join(out, name))
		if statErr != nil || info.Size() == 0 {
			t.Fatalf("artifact %s missing or empty: %v", name, statErr)
		}
	}
}

// writeThrowawayModule creates a dependency-free module whose native/ entry
// point echoes its arguments and exits 3 when the first one is "fail". It
// stands in for a game wherever a test needs the build-and-exec machinery
// rather than the native host itself.
func writeThrowawayModule(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/"+name+"\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n")
	write("native/main.go", `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	fmt.Println("args:", strings.Join(args, " "))
	if len(args) > 0 && args[0] == "fail" {
		os.Exit(3)
	}
}
`)
	return dir
}

// TestCheckDeterminismGuardsTheBlessedPath checks that simulation calls are
// refused, render-only files can opt out, and the explicit skip flag bypasses
// the check.
func TestCheckDeterminismGuardsTheBlessedPath(t *testing.T) {
	module := writeSimViolationModule(t)
	t.Chdir(module)

	err := checkDeterminism("./native", io.Discard)
	if err == nil {
		t.Fatal("checkDeterminism accepted math.Sin in simulation code")
	}
	if !strings.Contains(err.Error(), "--skip-determinism-check") {
		t.Fatalf("refusal %q does not mention the escape flag", err)
	}

	buildSkipDeterminism = true
	skipErr := checkDeterminism("./native", io.Discard)
	buildSkipDeterminism = false
	if skipErr != nil {
		t.Fatalf("with --skip-determinism-check: %v", skipErr)
	}

	// The render directive is the legitimate opt-out for presentation code.
	game := filepath.Join(module, "game", "game.go")
	body, readErr := os.ReadFile(game)
	if readErr != nil {
		t.Fatal(readErr)
	}
	directive := append([]byte("//arasy:render\n\n"), body...)
	if err := os.WriteFile(game, directive, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkDeterminism("./native", io.Discard); err != nil {
		t.Fatalf("with //arasy:render directive: %v", err)
	}
}

// TestCheckDeterminismCoversOrderRandAndClock checks that the blessed path
// refuses the three non-math hazards, names each one in the report, and
// honors the per-loop order directive end to end.
func TestCheckDeterminismCoversOrderRandAndClock(t *testing.T) {
	module := writeHazardModule(t)
	t.Chdir(module)

	var report strings.Builder
	err := checkDeterminism("./native", &report)
	if err == nil {
		t.Fatal("checkDeterminism accepted map iteration, math/rand, and time.Now in simulation code")
	}
	for _, fragment := range []string{
		"map iteration order",
		"math/rand bypasses the recorded random stream",
		"time.Now reads the wall clock",
	} {
		if !strings.Contains(report.String(), fragment) {
			t.Errorf("refusal report misses %q:\n%s", fragment, report.String())
		}
	}

	buildSkipDeterminism = true
	skipErr := checkDeterminism("./native", io.Discard)
	buildSkipDeterminism = false
	if skipErr != nil {
		t.Fatalf("with --skip-determinism-check: %v", skipErr)
	}

	// With the RNG and the clock gone, the order directive is the
	// legitimate opt-out for the one provably commutative loop.
	game := filepath.Join(module, "game", "game.go")
	if err := os.WriteFile(game, []byte(hazardModuleFixedGame), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkDeterminism("./native", io.Discard); err != nil {
		t.Fatalf("with //arasy:order-insensitive directive: %v", err)
	}
}

// writeHazardModule creates a module whose simulation package carries the
// three non-math hazards: a map range, a math/rand import, and a wall-clock
// read.
func writeHazardModule(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "hazards")
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/hazards\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+
		"\n\nrequire github.com/dhannyell/arasy v0.0.0\n\nreplace github.com/dhannyell/arasy => "+
		filepath.ToSlash(repoRoot(t))+"\n")
	write("game/game.go", `package game

import (
	"math/rand"
	"time"

	"github.com/dhannyell/arasy/runtime"
)

// Tally represents simulation code with the three hazards.
func Tally(r *runtime.Rand, m map[int]int) int {
	_ = r
	total := rand.Int() + int(time.Now().Unix())
	for _, v := range m {
		total += v
	}
	return total
}
`)
	write("native/main.go", `package main

import (
	"fmt"

	"example.com/hazards/game"
)

func main() {
	fmt.Println(game.Tally(nil, nil))
}
`)
	return dir
}

// hazardModuleFixedGame is the same package after the fix: RNG and clock
// removed, and the loop marked order-insensitive with its reason.
const hazardModuleFixedGame = `package game

import (
	"github.com/dhannyell/arasy/runtime"
)

// Tally represents simulation code whose loop is order-independent.
func Tally(r *runtime.Rand, m map[int]int) int {
	_ = r
	total := 0
	//arasy:order-insensitive: integer sum is commutative
	for _, v := range m {
		total += v
	}
	return total
}
`

// writeSimViolationModule creates a module that depends on the real engine
// through replace and calls math.Sin from a package reachable by the native
// target. This is the situation the build check must refuse.
func writeSimViolationModule(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "simviolation")
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/simviolation\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+
		"\n\nrequire github.com/dhannyell/arasy v0.0.0\n\nreplace github.com/dhannyell/arasy => "+
		filepath.ToSlash(repoRoot(t))+"\n")
	write("game/game.go", `package game

import (
	"math"

	"github.com/dhannyell/arasy/runtime"
)

// Advance represents simulation code.
func Advance(r *runtime.Rand, x float64) float64 {
	_ = r
	return math.Sin(x)
}
`)
	write("native/main.go", `package main

import (
	"fmt"

	"example.com/simviolation/game"
)

func main() {
	fmt.Println(game.Advance(nil, 1))
}
`)
	return dir
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
