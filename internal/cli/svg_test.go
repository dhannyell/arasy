package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileSVG(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.svg")
	output := filepath.Join(dir, "generated", "scene.gen.go")
	data := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><text x="1" y="5" font-family="Demo" font-size="4">OK</text></svg>`
	if err := os.WriteFile(source, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	options := svgFlags{
		output: output, packageName: "fixture", name: "Fixture",
		fonts: []string{"Demo=demo-font"}, strict: true,
	}
	if err := compileSVG(source, options); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `Demo render.FontID // manifest font "demo-font"`) {
		t.Fatalf("generated source did not expose mapped font:\n%s", generated)
	}
}

func TestParseFontMappingsRejectsDuplicates(t *testing.T) {
	if _, err := parseFontMappings([]string{"Demo=first", "Demo=second"}); err == nil {
		t.Fatal("parseFontMappings accepted a duplicate family")
	}
}
