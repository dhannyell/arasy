package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeDoctorPackagePrefersTheNativeDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if got := nativeDoctorPackage(nil); got != "" {
		t.Fatalf("nativeDoctorPackage with nothing to check = %q, want empty", got)
	}
	if err := os.Mkdir(filepath.Join(dir, "native"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := nativeDoctorPackage(nil); got != "./native" {
		t.Fatalf("nativeDoctorPackage with ./native = %q, want ./native", got)
	}
	if got := nativeDoctorPackage([]string{"./elsewhere"}); got != "./elsewhere" {
		t.Fatalf("nativeDoctorPackage with an explicit package = %q, want ./elsewhere", got)
	}
}

// A machine missing several development packages should learn all of them
// from one run, so the probe collects rather than stops at the first.
func TestMissingPkgConfigModulesNamesEveryAbsentOne(t *testing.T) {
	absent := []string{
		"arasy-no-such-module-one",
		"arasy-no-such-module-two",
	}
	missing := missingPkgConfigModules(absent)
	if len(missing) != len(absent) {
		t.Fatalf("missing = %v, want all of %v", missing, absent)
	}
	if strings.Join(missing, ",") != strings.Join(absent, ",") {
		t.Fatalf("missing = %v, want the order they were asked in", missing)
	}
	if got := missingPkgConfigModules(nil); len(got) != 0 {
		t.Fatalf("missing for no modules = %v, want none", got)
	}
}

// An out-of-reach host is a skip, never a failure: a machine that only ever
// ships to the web has nothing wrong with it.
func TestCheckNativeEnvironmentSkipsWithoutAPackage(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out strings.Builder
	failed, err := checkNativeEnvironment(&out, nativeDoctorPackage(nil))
	if err != nil {
		t.Fatal(err)
	}
	if failed {
		t.Fatal("a project with no native entry point was reported as a failure")
	}
	if !strings.HasPrefix(out.String(), "skipped native:") {
		t.Fatalf("output = %q, want a skip", out.String())
	}
}
