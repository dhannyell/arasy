package render_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/render"
)

var (
	_ render.PostEffect = render.Vignette{}
	_ render.PostEffect = render.ColorGrade{}
	_ render.PostEffect = render.Pixelate{}
	_ render.PostEffect = render.Bloom{}
	_ render.PostEffect = render.ChromaticAberration{}
	_ render.PostEffect = render.Shockwave{}
	_ render.PostEffect = render.LensRain{}
)

func TestPostChainIsACommandWithTheConfiguredLayer(t *testing.T) {
	chain := render.PostChain{
		BaseCommand: render.BaseCommand{LayerValue: 12},
		Effects:     []render.PostEffect{render.ColorGrade{}, render.Vignette{}},
		BelowLayer:  7,
		Time:        1.25,
	}

	if got := chain.Layer(); got != 12 {
		t.Fatalf("PostChain layer = %d, want 12", got)
	}
	if got := len(chain.Effects); got != 2 {
		t.Fatalf("PostChain effects = %d, want 2", got)
	}
}

func TestPostEffectVocabularyKeepsEffectOrder(t *testing.T) {
	effects := []render.PostEffect{
		render.Shockwave{},
		render.Bloom{},
		render.ColorGrade{},
		render.Vignette{},
	}
	chain := render.PostChain{Effects: effects}

	for i, want := range effects {
		if got := reflect.TypeOf(chain.Effects[i]); got != reflect.TypeOf(want) {
			t.Fatalf("effect %d has type %v, want %v", i, got, reflect.TypeOf(want))
		}
	}
}

func TestPostCommandZeroValueIsOnTheDefaultLayer(t *testing.T) {
	var command render.Command = render.PostChain{}

	if got := command.Layer(); got != 0 {
		t.Fatalf("zero PostChain layer = %d, want 0", got)
	}
}
