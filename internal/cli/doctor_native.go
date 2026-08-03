package cli

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// linuxNativePkgConfig lists the pkg-config modules the native host links
// against on Linux: GLFW's X11 and Wayland backends, and ALSA for audio.
var linuxNativePkgConfig = []string{
	"x11", "xcursor", "xi", "xinerama", "xrandr", "xxf86vm",
	"wayland-client", "xkbcommon", "alsa",
}

// linuxNativeAptLine is the one command that installs what the probes above
// look for, phrased the way the distribution guide phrases it.
const linuxNativeAptLine = "sudo apt-get install -y libegl1-mesa-dev mesa-vulkan-drivers libasound2-dev " +
	"libxcursor-dev libxi-dev libxinerama-dev libxrandr-dev libxxf86vm-dev libxkbcommon-dev libwayland-dev"

// checkNativeEnvironment reports whether this machine can build a native game.
// A host outside the native host's reach is a skip, never a failure: a machine
// that only ever ships to the web has nothing wrong with it.
func checkNativeEnvironment(out io.Writer, pkg string) (failed bool, err error) {
	if pkg == "" {
		return false, doctorPrintf(out, "skipped native: pass a package or run from a project with ./native\n")
	}
	if hostErr := requireNativeHost(); hostErr != nil {
		return false, doctorPrintf(out, "skipped native: %v\n", hostErr)
	}
	if runtime.GOOS == "darwin" {
		// Tier 3: it compiles, nobody has played it. Probing further would
		// imply a promise the project does not make.
		return false, doctorPrintf(out, "skipped native: macOS is a best-effort platform and is not validated\n")
	}

	// Every check runs even after one fails: someone missing both a compiler
	// and the X11 headers should learn that in one pass, not two.
	checks := []func(io.Writer) (bool, error){checkCGOEnabled, checkCCompiler}
	if runtime.GOOS == "linux" {
		checks = append(checks, checkLinuxNativeLibraries)
	}
	checks = append(checks, func(w io.Writer) (bool, error) { return checkNativeBuild(w, pkg) })

	for _, check := range checks {
		checkFailed, printErr := check(out)
		if printErr != nil {
			return failed, printErr
		}
		failed = failed || checkFailed
	}
	return failed, nil
}

func checkCGOEnabled(out io.Writer) (bool, error) {
	value, err := exec.Command("go", "env", "CGO_ENABLED").Output()
	if err != nil {
		return true, doctorPrintf(out, "FAIL native cgo: %v\n", err)
	}
	if strings.TrimSpace(string(value)) != "1" {
		return true, doctorPrintf(out, "FAIL native cgo: CGO_ENABLED is %s; the native host needs cgo for GLFW (set CGO_ENABLED=1)\n", strings.TrimSpace(string(value)))
	}
	return false, doctorPrintf(out, "ok native cgo: enabled\n")
}

func checkCCompiler(out io.Writer) (bool, error) {
	for _, name := range []string{"gcc", "clang", "cc"} {
		if path, err := exec.LookPath(name); err == nil {
			return false, doctorPrintf(out, "ok native C compiler: %s\n", path)
		}
	}
	hint := "install build-essential"
	if runtime.GOOS == "windows" {
		hint = "install MinGW-w64, or the gcc from a toolchain such as w64devkit or MSYS2"
	}
	return true, doctorPrintf(out, "FAIL native C compiler: no gcc, clang or cc on PATH; %s\n", hint)
}

func checkLinuxNativeLibraries(out io.Writer) (bool, error) {
	if _, err := exec.LookPath("pkg-config"); err != nil {
		return true, doctorPrintf(out, "FAIL native libraries: pkg-config is not on PATH, so the development headers cannot be checked; %s\n", linuxNativeAptLine)
	}
	missing := missingPkgConfigModules(linuxNativePkgConfig)
	if len(missing) > 0 {
		return true, doctorPrintf(out, "FAIL native libraries: missing %s; %s\n", strings.Join(missing, ", "), linuxNativeAptLine)
	}
	return false, doctorPrintf(out, "ok native libraries: %d development packages present\n", len(linuxNativePkgConfig))
}

// missingPkgConfigModules reports which of the given modules pkg-config cannot
// find, in the order they were asked for, so the failure names all of them at
// once instead of one per run.
func missingPkgConfigModules(modules []string) []string {
	var missing []string
	for _, module := range modules {
		if err := exec.Command("pkg-config", "--exists", module).Run(); err != nil {
			missing = append(missing, module)
		}
	}
	return missing
}

func checkNativeBuild(out io.Writer, pkg string) (bool, error) {
	dir, err := os.MkdirTemp("", "arasy-doctor-native-*")
	if err != nil {
		return true, doctorPrintf(out, "FAIL native build: %v\n", err)
	}
	defer func() { _ = removeTempDir(dir) }()
	stopInterruptCleanup := watchTempDirInterrupt(dir)
	defer stopInterruptCleanup()

	var output strings.Builder
	if buildErr := buildNative(pkg, dir, nativeBuildOptions{Mode: nativeModePortable}, &output, &output); buildErr != nil {
		return true, doctorPrintf(out, "FAIL native build: %v\n%s", buildErr, output.String())
	}
	return false, doctorPrintf(out, "ok native build: %s\n", pkg)
}

// nativeDoctorPackage picks the package the native checks should build: the
// one the caller named, or the conventional ./native directory when it exists.
// An empty result means there is nothing to check.
func nativeDoctorPackage(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	if info, err := os.Stat(platformNative); err == nil && info.IsDir() {
		return "./native"
	}
	return ""
}
