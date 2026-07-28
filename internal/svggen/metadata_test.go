package svggen

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestPathControlBoundsConservativelyIncludesCurveControls(t *testing.T) {
	bounds, ok := pathControlBounds([]PathCommand{
		{Verb: PathMove, P0: math2d.Vec2{X: 10, Y: 20}},
		{
			Verb: PathCubic,
			P0:   math2d.Vec2{X: -5, Y: 40},
			P1:   math2d.Vec2{X: 70, Y: -10},
			P2:   math2d.Vec2{X: 50, Y: 30},
		},
	})
	if !ok {
		t.Fatal("pathControlBounds() did not report bounds")
	}
	want := math2d.Rect{
		Min: math2d.Vec2{X: -5, Y: -10},
		Max: math2d.Vec2{X: 70, Y: 40},
	}
	if bounds != want {
		t.Fatalf("bounds = %+v, want %+v", bounds, want)
	}
}

func TestAssetMetricsCountCommandsDepthAndText(t *testing.T) {
	document, err := Parse([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">
<g><path d="M0 0 L10 0 L10 10 Z" fill="#fff" stroke="#000"/><text font-size="10">áB</text></g>
</svg>`), Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	g := generator{document: document, options: GenerateOptions{Package: "sample", Name: "Sample"}}
	if err := g.collect(); err != nil {
		t.Fatal(err)
	}
	g.prepareNodeMetadata()

	commands, textNodes, textRunes, maxDepth := g.assetMetrics()
	if commands != 3 || textNodes != 1 || textRunes != 2 || maxDepth != 3 {
		t.Fatalf(
			"metrics = commands:%d textNodes:%d textRunes:%d maxDepth:%d, want 3,1,2,3",
			commands,
			textNodes,
			textRunes,
			maxDepth,
		)
	}
}
