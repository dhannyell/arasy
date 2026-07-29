package wasmserve

import (
	"strings"
	"testing"
)

func TestShellPosterIsInlineSoItsViewportMatchesBootSurface(t *testing.T) {
	body, err := ShellHTML("Arasy", "app.wasm")
	if err != nil {
		t.Fatal(err)
	}

	html := string(body)
	if !strings.Contains(html, `<svg xmlns="http://www.w3.org/2000/svg" id="ar-poster"`) {
		t.Fatal("poster SVG must be embedded inline")
	}
	if strings.Contains(html, `<img id="ar-poster"`) ||
		strings.Contains(html, "data:image/svg+xml;base64,") {
		t.Fatal("poster must not be rendered as a replaced img element")
	}
	if !strings.Contains(html, "#ar-poster {") ||
		!strings.Contains(html, "display: block;") {
		t.Fatal("inline poster must fill the boot surface")
	}
}

// TestShellRemovesBootOverlayWithoutItsOwnFade asserts the poster overlay
// is a hard cut, not a competing CSS opacity transition: the revealed
// SplashSegment already fades itself in (runtime/splash.go's
// splashFadeAlpha), so the shell must not also animate #ar-boot's opacity,
// or the two fades stack into a visible dip (see the doc comment there).
func TestShellRemovesBootOverlayWithoutItsOwnFade(t *testing.T) {
	body, err := ShellHTML("Arasy", "app.wasm")
	if err != nil {
		t.Fatal(err)
	}

	html := string(body)
	if strings.Contains(html, "transition: opacity") {
		t.Fatal("#ar-boot must not run its own opacity transition alongside the segment's own fade-in")
	}
	if !strings.Contains(html, "boot.remove();") {
		t.Fatal("reveal() must remove the boot overlay directly, with no transitionend wait")
	}
	if !strings.Contains(html, "window.arasyBootFadeDone = true;") {
		t.Fatal("reveal() must still set arasyBootFadeDone for tests that wait on it")
	}
}
