package svggen

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestParseCSSTransform(t *testing.T) {
	transform, err := ParseCSSTransform("translate(10px, 20px) scale(2) rotate(90deg)")
	if err != nil {
		t.Fatal(err)
	}
	got := transform.Apply(math2d.Vec2{X: 1})
	if got.X != 10 || got.Y != 22 {
		t.Fatalf("transformed point = %v, want (10,22)", got)
	}
}

func FuzzParseCSSTransform(f *testing.F) {
	f.Add("translate(10px,20px) scale(2)")
	f.Add("rotate(90deg)")
	f.Add("matrix(1,0,0,1,2,3)")
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = ParseCSSTransform(source)
	})
}
