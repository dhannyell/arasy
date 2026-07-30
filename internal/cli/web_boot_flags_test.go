package cli

import "testing"

func TestWebBootFlagsKeepLoadingAndRuntimeSplashIndependent(t *testing.T) {
	loadingOnly, err := (webBootFlags{noSplash: true}).options()
	if err != nil {
		t.Fatal(err)
	}
	if !loadingOnly.DisableRuntimeSplash || loadingOnly.DisableLoadingIndicator {
		t.Fatalf("no-splash options = %+v, want runtime splash off and loading on", loadingOnly)
	}

	splashOnly, err := (webBootFlags{noLoading: true, noEngineSplash: true, bootBackground: "#123456"}).options()
	if err != nil {
		t.Fatal(err)
	}
	if !splashOnly.DisableLoadingIndicator || splashOnly.DisableRuntimeSplash ||
		!splashOnly.DisableEngineSplash || splashOnly.Background != "#123456" {
		t.Fatalf("independent options = %+v", splashOnly)
	}
}

func TestWebBootFlagsRejectInvalidBackground(t *testing.T) {
	if _, err := (webBootFlags{bootBackground: "navy"}).options(); err == nil {
		t.Fatal("non-hexadecimal boot background succeeded")
	}
}

func TestDevDisablesEngineSplashByDefault(t *testing.T) {
	flag := devCmd.Flags().Lookup("no-engine-splash")
	if flag == nil {
		t.Fatal("dev command has no --no-engine-splash flag")
	}
	if flag.DefValue != "true" {
		t.Fatalf("dev --no-engine-splash default = %q, want %q", flag.DefValue, "true")
	}
	if !devBoot.noEngineSplash {
		t.Fatal("dev boot flags default the engine splash to on, want off")
	}
}
