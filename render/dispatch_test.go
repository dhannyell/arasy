package render_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/render"
)

// recordingSink records a label per Visit call, in call order, so tests can
// assert both dispatch order and which method was invoked.
type recordingSink struct {
	visits []string
}

func (s *recordingSink) VisitRect(render.DrawRect)       { s.visits = append(s.visits, "rect") }
func (s *recordingSink) VisitCircle(render.DrawCircle)   { s.visits = append(s.visits, "circle") }
func (s *recordingSink) VisitPoint(render.DrawPoint)     { s.visits = append(s.visits, "point") }
func (s *recordingSink) VisitLine(render.DrawLine)       { s.visits = append(s.visits, "line") }
func (s *recordingSink) VisitText(render.DrawText)       { s.visits = append(s.visits, "text") }
func (s *recordingSink) VisitArc(render.DrawArc)         { s.visits = append(s.visits, "arc") }
func (s *recordingSink) VisitPolygon(render.DrawPolygon) { s.visits = append(s.visits, "polygon") }
func (s *recordingSink) VisitImage(render.DrawImage)     { s.visits = append(s.visits, "image") }
func (s *recordingSink) VisitImageQuad(render.DrawImageQuad) {
	s.visits = append(s.visits, "imageQuad")
}
func (s *recordingSink) VisitParticles(render.DrawParticles) {
	s.visits = append(s.visits, "particles")
}
func (s *recordingSink) VisitSpriteBatch(render.DrawSpriteBatch) {
	s.visits = append(s.visits, "spriteBatch")
}
func (s *recordingSink) VisitTextBatch(render.DrawTextBatch) {
	s.visits = append(s.visits, "textBatch")
}

func layered(layer int) render.BaseCommand {
	return render.BaseCommand{LayerValue: layer}
}

func TestEachDispatchesInAscendingLayerOrder(t *testing.T) {
	commands := []render.Command{
		render.DrawCircle{BaseCommand: layered(3)},
		render.DrawRect{BaseCommand: layered(1)},
		render.DrawLine{BaseCommand: layered(2)},
	}

	sink := &recordingSink{}
	render.Each(commands, sink)

	want := []string{"rect", "line", "circle"}
	if !reflect.DeepEqual(sink.visits, want) {
		t.Fatalf("visits = %v, want %v", sink.visits, want)
	}
}

func TestEachDispatchIsStableForEqualLayers(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{BaseCommand: layered(1)},
		render.DrawCircle{BaseCommand: layered(1)},
		render.DrawLine{BaseCommand: layered(1)},
		render.DrawPoint{BaseCommand: layered(0)},
	}

	sink := &recordingSink{}
	render.Each(commands, sink)

	// Same layer (1) commands must keep their original relative order;
	// the layer-0 point sorts before them.
	want := []string{"point", "rect", "circle", "line"}
	if !reflect.DeepEqual(sink.visits, want) {
		t.Fatalf("visits = %v, want %v", sink.visits, want)
	}
}

// TestEachDispatchesEveryCommandKindExactlyOnce guards Each's type switch
// against silently skipping a command kind: if a new Command implementation
// is added to render but not wired into Each's switch, this test must gain
// a case for it and will fail until the new Visit call is added.
func TestEachDispatchesEveryCommandKindExactlyOnce(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{},
		render.DrawCircle{},
		render.DrawPoint{},
		render.DrawLine{},
		render.DrawText{},
		render.DrawArc{},
		render.DrawPolygon{},
		render.DrawImage{},
		render.DrawImageQuad{},
		render.DrawParticles{},
		render.DrawSpriteBatch{},
		render.DrawTextBatch{},
	}

	sink := &recordingSink{}
	render.Each(commands, sink)

	want := []string{
		"rect", "circle", "point", "line", "text", "arc", "polygon", "image", "imageQuad", "particles",
		"spriteBatch", "textBatch",
	}
	if !reflect.DeepEqual(sink.visits, want) {
		t.Fatalf("visits = %v, want %v (each of the 12 command kinds must dispatch exactly once)", sink.visits, want)
	}
}

func TestEachDoesNotReorderCallerSlice(t *testing.T) {
	commands := []render.Command{
		render.DrawCircle{BaseCommand: layered(3)},
		render.DrawRect{BaseCommand: layered(1)},
		render.DrawLine{BaseCommand: layered(2)},
	}

	before := append([]render.Command(nil), commands...)

	render.Each(commands, &recordingSink{})

	if !reflect.DeepEqual(commands, before) {
		t.Fatalf("caller slice reordered: got %v, want %v", commands, before)
	}
}

func TestEachOrderedDispatchesWithoutReordering(t *testing.T) {
	commands := []render.Command{
		render.DrawCircle{BaseCommand: layered(3)},
		render.DrawRect{BaseCommand: layered(1)},
		render.DrawLine{BaseCommand: layered(2)},
	}

	sink := &recordingSink{}
	render.EachOrdered(commands, sink)

	want := []string{"circle", "rect", "line"}
	if !reflect.DeepEqual(sink.visits, want) {
		t.Fatalf("visits = %v, want input order %v", sink.visits, want)
	}
}

// TestEachKeepsBatchCommandsAtTheirPainterPosition pins batch commands as
// painter-order boundaries: a batch sorts by its own layer and stays stable
// against streaming commands on the same layer.
func TestEachKeepsBatchCommandsAtTheirPainterPosition(t *testing.T) {
	commands := []render.Command{
		render.DrawImage{BaseCommand: layered(2)},
		render.DrawTextBatch{BaseCommand: layered(3), Batch: 9},
		render.DrawSpriteBatch{BaseCommand: layered(1), Batch: 7},
		render.DrawRect{BaseCommand: layered(1)},
	}

	sink := &recordingSink{}
	render.Each(commands, sink)

	want := []string{"spriteBatch", "rect", "image", "textBatch"}
	if !reflect.DeepEqual(sink.visits, want) {
		t.Fatalf("visits = %v, want %v", sink.visits, want)
	}
}
