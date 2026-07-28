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
	for _, expected := range []string{
		"type SampleMetrics struct",
		"SampleAssetMetrics = SampleMetrics",
		"type SampleBudget struct",
		"type SampleWorkspace struct",
		"func (w *SampleWorkspace) Append(",
		"func (w *SampleWorkspace) FrameMetrics()",
		"func AppendSample(",
		"AppendFillWithWorkspace",
		"AppendStrokeWithWorkspace",
		"svgLODWithHysteresis",
		"svgTransformRect",
		"hasBounds: true",
		"anim.Timeline",
		"render.Builder",
		"ease.MustCubicBezier",
	} {
		if !strings.Contains(string(first), expected) {
			t.Fatalf("generated source does not contain %q", expected)
		}
	}
}

func TestGeneratePreparesStaticTextMetrics(t *testing.T) {
	source := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">
<text x="50" y="25" text-anchor="middle" font-size="10" letter-spacing="2">áBC</text>
</svg>`)
	document, err := Parse(source, Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := Generate(document, GenerateOptions{Package: "sample", Name: "Sample"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	for _, expected := range []string{
		"TextNodes: 1, TextRunes: 3",
		"NaturalWidth: 18",
		"RuneCount: 3",
		"Metrics: node.textMetrics",
	} {
		if !strings.Contains(text, expected) {
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

func TestGenerateAnimatedOpacityOverridesPresentationAttribute(t *testing.T) {
	source := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<style>.visible{animation:show 1s linear both}@keyframes show{from{opacity:0}to{opacity:1}}</style>
<circle class="visible" cx="5" cy="5" r="4" opacity="0"/>
</svg>`)
	document, err := Parse(source, Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := Generate(document, GenerateOptions{Package: "sample", Name: "Sample"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	if !strings.Contains(text, "opacity = svgScalar(timeline, node.animation, node.animation.opacity, node.opacity)") {
		t.Fatal("generated animation does not override the presentation opacity")
	}
	if strings.Contains(text, "node.opacity * svgScalar") {
		t.Fatal("generated animation still multiplies the presentation opacity")
	}
	opacityGuard := strings.Index(text, "if opacity <= 0")
	transformEvaluation := strings.Index(text, "transform := svgAffine")
	if opacityGuard < 0 || transformEvaluation < 0 || opacityGuard > transformEvaluation {
		t.Fatal("generated runtime does not skip transparent nodes before geometry evaluation")
	}
}
