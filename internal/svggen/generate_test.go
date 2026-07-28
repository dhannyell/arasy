package svggen

import (
	"bytes"
	"strings"
	"testing"
)

func TestGenerateIsDeterministicAndFormatted(t *testing.T) {
	source := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">
<style>.moving{animation:move 2s cubic-bezier(.2,0,.8,1) both}@keyframes move{from{opacity:0;transform:translate(0px,0px)}to{opacity:1;transform:translate(10px,0px)}}</style>
<rect class="moving" x="5" y="5" width="20" height="10" fill="#123456"/>
</svg>`)
	document, err := Parse(source, Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	first, err := Generate(document, GenerateOptions{Package: "sample", Name: "Sample"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(document, GenerateOptions{Package: "sample", Name: "Sample"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generation is not deterministic")
	}
	for _, expected := range []string{"func AppendSample(", "anim.Timeline", "render.Builder", "ease.MustCubicBezier"} {
		if !strings.Contains(string(first), expected) {
			t.Fatalf("generated source does not contain %q", expected)
		}
	}
}

func TestGenerateRejectsInvalidPublicNames(t *testing.T) {
	document, err := Parse([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`), Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(document, GenerateOptions{Package: "bad-name", Name: "sample"}); err == nil {
		t.Fatal("Generate accepted invalid names")
	}
}
