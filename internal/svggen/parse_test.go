package svggen

import (
	"strings"
	"testing"
)

const minimalSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">
<style>.shape { fill:#ff0000; animation: fade 1s ease both }
@keyframes fade { from { opacity:0 } to { opacity:1 } }</style>
<path class="shape" d="M 0 0 L 10 0 L 10 10 Z"/>
</svg>`

func TestParseSecureAnimatedSVG(t *testing.T) {
	document, err := Parse([]byte(minimalSVG), Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if document.ViewBox.Width() != 100 || len(document.Styles.Keyframes["fade"]) != 2 {
		t.Fatalf("document = %+v", document)
	}
	path := document.Root.Children[1]
	if path.Attributes["fill"] != "#ff0000" {
		t.Fatalf("resolved fill = %q", path.Attributes["fill"])
	}
}

func TestParseRejectsSecurityAndSubsetViolations(t *testing.T) {
	tests := []string{
		`<!DOCTYPE svg><svg viewBox="0 0 1 1"/>`,
		`<svg viewBox="0 0 1 1"><script/></svg>`,
		`<svg viewBox="0 0 1 1"><image href="https://example.com/a.png"/></svg>`,
		`<svg viewBox="0 0 1 1"><style>@import "x";</style></svg>`,
		`<svg viewBox="0 0 1 1"><g onclick="x()"/></svg>`,
		`<svg viewBox="0 0 1 1"><style>.a > .b{fill:red}</style></svg>`,
	}
	for _, source := range tests {
		if _, err := Parse([]byte(source), Options{Strict: true}); err == nil {
			t.Fatalf("Parse(%q) returned nil error", source)
		}
	}
}

func TestParseEnforcesInputAndDepthLimits(t *testing.T) {
	if _, err := Parse(make([]byte, MaxInputBytes+1), Options{Strict: true}); err == nil {
		t.Fatal("oversized input returned nil error")
	}
	source := `<svg viewBox="0 0 1 1">` + strings.Repeat("<g>", MaxDepth) +
		strings.Repeat("</g>", MaxDepth) + `</svg>`
	if _, err := Parse([]byte(source), Options{Strict: true}); err == nil {
		t.Fatal("excessive depth returned nil error")
	}
}

func TestStyleElementMustBeDirectSVGChild(t *testing.T) {
	source := `<svg viewBox="0 0 1 1"><g><style>.a{fill:red}</style></g></svg>`
	if _, err := Parse([]byte(source), Options{Strict: true}); err == nil {
		t.Fatal("nested style returned nil error")
	}
}

func FuzzParseSVG(f *testing.F) {
	f.Add([]byte(minimalSVG))
	f.Add([]byte(`<svg viewBox="0 0 1 1"><path d="M0 0 L"/></svg>`))
	f.Add([]byte(`<!ENTITY x "boom">`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse(data, Options{Strict: true})
	})
}
