package render_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestBaseCommandLayerReturnsConfiguredLayer(t *testing.T) {
	command := render.BaseCommand{LayerValue: 7}

	if got := command.Layer(); got != 7 {
		t.Fatalf("Layer() = %d, want 7", got)
	}
}

// WithLayer is a type switch with a silent fallthrough: a command type added
// without a case there keeps its own layer, and a CommandSprite group
// silently breaks. CommandSink has one Visit method per command type, so
// reflecting over it discovers every command this package can ever emit.
func TestWithLayerCoversEveryCommandType(t *testing.T) {
	sink := reflect.TypeOf((*render.CommandSink)(nil)).Elem()

	for i := range sink.NumMethod() {
		argument := sink.Method(i).Type.In(0)
		command, ok := reflect.New(argument).Elem().Interface().(render.Command)
		if !ok {
			t.Fatalf("%s takes %v, which is not a render.Command", sink.Method(i).Name, argument)
		}

		if got := render.WithLayer(command, 42); got.Layer() != 42 {
			t.Errorf("WithLayer(%s, 42) kept layer %d: add a case to the switch",
				argument.Name(), got.Layer())
		}
	}
}

// withLightExemptions lists the commands WithLight must return unchanged,
// each with its recorded reason. Every other command must have every
// reachable Color field modulated — the reflection walk below discovers the
// fields, so a forgotten field fails here by name, the way DrawRect.Stroke
// was once forgotten by the sprite lighting path.
var withLightExemptions = map[reflect.Type]string{
	reflect.TypeOf(render.DrawParticles{}):   "particles declare their own blend (153 decision)",
	reflect.TypeOf(render.DrawSpriteBatch{}): "retained content: lit via a per-batch uniform when that extension lands",
	reflect.TypeOf(render.DrawTextBatch{}):   "retained content: same as DrawSpriteBatch",
	reflect.TypeOf(render.PostChain{}):       "post effects own their image-space colors and are not scene lighting",
}

// WithLight has the same silent-fallthrough shape as WithLayer, plus a
// second way to rot: a command handled for one color field but not another.
// CommandSink enumerates the types; reflection enumerates the fields.
func TestWithLightCoversEveryCommandColorField(t *testing.T) {
	sink := reflect.TypeOf((*render.CommandSink)(nil)).Elem()
	sentinel := render.RGBA(0.123, 0.456, 0.789, 0.5)
	half := render.RGBA(0.5, 0.5, 0.5, 1)

	for i := range sink.NumMethod() {
		argument := sink.Method(i).Type.In(0)

		if reason, exempt := withLightExemptions[argument]; exempt {
			original, ok := reflect.New(argument).Elem().Interface().(render.Command)
			if !ok {
				t.Fatalf("%s is not a render.Command", argument.Name())
			}
			if got := render.WithLight(original, half); !reflect.DeepEqual(got, original) {
				t.Errorf("WithLight(%s) must pass through unchanged: %s", argument.Name(), reason)
			}
			continue
		}

		value := reflect.New(argument).Elem()
		found := setColorFields(value, sentinel)
		if found == 0 {
			t.Errorf("%s has no Color fields: modulate it or add it to withLightExemptions with a reason", argument.Name())
			continue
		}

		command, ok := value.Interface().(render.Command)
		if !ok {
			t.Fatalf("%s is not a render.Command", argument.Name())
		}
		lit := reflect.ValueOf(render.WithLight(command, half))
		var missed []string
		findColorFields(lit, sentinel, argument.Name(), &missed)
		for _, path := range missed {
			t.Errorf("WithLight left %s unmodulated: add it to the %s case", path, argument.Name())
		}
	}
}

// setColorFields writes the sentinel into every reachable render.Color in v
// and reports how many it found. Nested structs, arrays and populated slices
// are walked; a future command carrying colors in a slice must be given a
// populated prototype here to be seen.
func setColorFields(v reflect.Value, sentinel render.Color) int {
	colorType := reflect.TypeOf(render.Color{})
	found := 0
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == colorType {
			v.Set(reflect.ValueOf(sentinel))
			return 1
		}
		for i := range v.NumField() {
			if field := v.Field(i); field.CanSet() {
				found += setColorFields(field, sentinel)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			found += setColorFields(v.Index(i), sentinel)
		}
	}
	return found
}

// findColorFields records the path of every render.Color in v still equal to
// the sentinel — a color the modulation never touched.
func findColorFields(v reflect.Value, sentinel render.Color, path string, missed *[]string) {
	colorType := reflect.TypeOf(render.Color{})
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == colorType {
			if v.Interface().(render.Color) == sentinel {
				*missed = append(*missed, path)
			}
			return
		}
		for i := range v.NumField() {
			findColorFields(v.Field(i), sentinel, path+"."+v.Type().Field(i).Name, missed)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			findColorFields(v.Index(i), sentinel, fmt.Sprintf("%s[%d]", path, i), missed)
		}
	}
}

// The zero tint means "draw as-is", so lighting has to promote it to white
// before multiplying — otherwise an untinted image would ignore the light.
func TestWithLightPromotesZeroTintToWhite(t *testing.T) {
	half := render.RGBA(0.5, 0.5, 0.5, 1)

	image, ok := render.WithLight(render.DrawImage{}, half).(render.DrawImage)
	if !ok {
		t.Fatal("WithLight changed the command type")
	}
	if want := render.RGBA(0.5, 0.5, 0.5, 1); image.Tint != want {
		t.Fatalf("zero tint under half light = %+v, want %+v", image.Tint, want)
	}
}

func TestDrawCommandsImplementCommand(t *testing.T) {
	commands := []render.Command{
		render.DrawRect{},
		render.DrawCircle{},
		render.DrawEllipse{},
		render.DrawPoint{},
		render.DrawLine{},
		render.DrawText{},
		render.DrawArc{},
		render.DrawPolygon{},
		render.DrawTriangles{},
		render.DrawImage{},
		render.DrawImageQuad{},
		render.PostChain{},
	}

	for _, command := range commands {
		if got := command.Layer(); got != 0 {
			t.Fatalf("%T default layer = %d, want 0", command, got)
		}
	}
}

func TestDrawTextHasExpectedZeroValue(t *testing.T) {
	command := render.DrawText{}

	if command.Font != 0 {
		t.Fatalf("Font = %d, want built-in font 0", command.Font)
	}
}

func TestDrawImageHasExpectedZeroValue(t *testing.T) {
	command := render.DrawImage{}

	if got := command.Src; got != (math2d.Rect{}) {
		t.Fatalf("Src = %#v, want zero value", got)
	}
	if got := command.Tint; got != (render.Color{}) {
		t.Fatalf("Tint = %#v, want zero value", got)
	}
}

func TestDrawImageQuadHasExpectedZeroValueAndLayer(t *testing.T) {
	command := render.DrawImageQuad{BaseCommand: render.BaseCommand{LayerValue: 9}}

	if got := command.Layer(); got != 9 {
		t.Fatalf("Layer() = %d, want 9", got)
	}
	if got := command.Src; got != (math2d.Rect{}) {
		t.Fatalf("Src = %#v, want zero value", got)
	}
	if got := command.Tint; got != (render.Color{}) {
		t.Fatalf("Tint = %#v, want zero value", got)
	}
}

func TestDrawImageUsesBaseCommandLayer(t *testing.T) {
	command := render.DrawImage{BaseCommand: render.BaseCommand{LayerValue: 9}}

	if got := command.Layer(); got != 9 {
		t.Fatalf("Layer() = %d, want 9", got)
	}
}

func TestBaseCommandTransformDefaultIsIdentity(t *testing.T) {
	bc := render.BaseCommand{}
	if !bc.Transform.IsIdentity() {
		t.Fatalf("default Transform should be identity, got %+v", bc.Transform)
	}
}

func TestBaseCommandTransformStoresMatrix(t *testing.T) {
	bc := render.BaseCommand{Transform: math2d.Rotate(1)}

	if bc.Transform.IsIdentity() {
		t.Fatal("Rotate(1) should not be identity")
	}

	p := math2d.Vec2{X: 1, Y: 0}
	got := bc.Transform.Apply(p)
	want := math2d.Rotate(1).Apply(p)
	if got != want {
		t.Fatalf("Transform.Apply(%v) = %v, want %v", p, got, want)
	}
}

func TestAllCommandsCanCarryTransform(t *testing.T) {
	transform := math2d.Translate(math2d.Vec2{X: 10, Y: 20})

	commands := []render.Command{
		render.DrawRect{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawCircle{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawEllipse{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawPoint{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawLine{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawText{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawArc{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawPolygon{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawTriangles{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawImage{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawImageQuad{BaseCommand: render.BaseCommand{Transform: transform}},
		render.DrawParticles{BaseCommand: render.BaseCommand{Transform: transform}},
	}

	for _, cmd := range commands {
		if cmd.Layer() != 0 {
			t.Fatalf("%T default layer should be 0", cmd)
		}
	}
}
