package asset_test

import (
	"math"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
)

const effectsManifestJSON = `{
	"version": 1,
	"textures": [{"name": "soft_disc", "file": "textures/soft_disc.png"}],
	"effects": [
		{
			"name": "ember",
			"texture": "soft_disc",
			"blend": "additive",
			"capacity": 500,
			"emit_rate": 20,
			"burst_size": 5,
			"lifetime": [1.0, 2.5],
			"speed": [10, 30],
			"direction": 90,
			"spread": 45,
			"shape": "disc",
			"emit_radius": 3,
			"velocity_mode": "radial",
			"gravity": [0, -5],
			"drag": 0.9,
			"size_start": [4, 6],
			"size_end": [1, 2],
			"color_start": [1, 0.6, 0.2, 1],
			"color_end": [1, 0.2, 0, 0],
			"spin": 2.0,
			"rotation": [0, 180],
			"wobble_amp": 0.5,
			"wobble_freq": 1.5,
			"fade_in": 0.2
		},
		{
			"name": "flame",
			"capacity": 200,
			"lifetime": [0.5, 0.5]
		}
	]
}`

func TestLoadEffectsResolvesByNameAssignsAlphabeticalIDsAndConvertsUnits(t *testing.T) {
	layer := fstest.MapFS{
		"manifest.json":          {Data: []byte(effectsManifestJSON)},
		"textures/soft_disc.png": {Data: []byte("soft-disc-bytes")},
	}

	m, err := asset.Load(layer)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// IDs assigned by alphabetical order of name: "ember" < "flame".
	if got := m.Effect("ember"); got != 1 {
		t.Fatalf("Effect(ember) = %d, want 1", got)
	}
	if got := m.Effect("flame"); got != 2 {
		t.Fatalf("Effect(flame) = %d, want 2", got)
	}
	if got := m.Effect("nonexistent"); got != 0 {
		t.Fatalf("Effect(nonexistent) = %d, want 0", got)
	}

	all := m.Effects()
	if len(all) != 2 {
		t.Fatalf("Effects() len = %d, want 2", len(all))
	}

	ember, ok := m.EffectDecl(m.Effect("ember"))
	if !ok {
		t.Fatal("EffectDecl(ember) ok = false, want true")
	}
	wantEmber := particle.EffectDecl{
		Name:       "ember",
		Texture:    "soft_disc",
		Blend:      particle.BlendAdditive,
		Capacity:   500,
		EmitRate:   20,
		BurstSize:  5,
		Lifetime:   particle.Range{Min: 1.0, Max: 2.5},
		Speed:      particle.Range{Min: 10, Max: 30},
		Direction:  math.Pi / 2,
		Spread:     math.Pi / 4,
		Shape:      particle.EmitDisc,
		EmitRadius: 3,
		VelMode:    particle.VelRadial,
		Gravity:    math2d.Vec2{X: 0, Y: -5},
		Drag:       0.9,
		SizeStart:  particle.Range{Min: 4, Max: 6},
		SizeEnd:    particle.Range{Min: 1, Max: 2},
		ColorStart: [4]float64{1, 0.6, 0.2, 1},
		ColorEnd:   [4]float64{1, 0.2, 0, 0},
		Spin:       2.0,
		Rotation:   particle.Range{Min: 0, Max: math.Pi},
		WobbleAmp:  0.5,
		WobbleFreq: 1.5,
		FadeIn:     0.2,
	}
	assertEffectDeclNear(t, ember, wantEmber)

	flame, ok := m.EffectDecl(m.Effect("flame"))
	if !ok {
		t.Fatal("EffectDecl(flame) ok = false, want true")
	}
	wantFlame := particle.EffectDecl{
		Name:     "flame",
		Blend:    particle.BlendAlpha, // omitted blend defaults to alpha
		Capacity: 200,
		Lifetime: particle.Range{Min: 0.5, Max: 0.5},
		Shape:    particle.EmitPoint, // omitted shape defaults to point
		VelMode:  particle.VelCone,   // omitted velocity_mode defaults to cone
	}
	assertEffectDeclNear(t, flame, wantFlame)

	if _, ok := m.EffectDecl(99); ok {
		t.Fatal("EffectDecl(99) ok = true, want false for unknown ID")
	}
	if _, ok := m.EffectDecl(0); ok {
		t.Fatal("EffectDecl(0) ok = true, want false")
	}
}

func TestLoadEffectsRejectsInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{
			name:     "capacity zero",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":0,"lifetime":[1,1]}]}`,
			want:     "capacity",
		},
		{
			name:     "capacity above MaxCapacity",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":70000,"lifetime":[1,1]}]}`,
			want:     "capacity",
		},
		{
			name:     "lifetime min zero",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[0,1]}]}`,
			want:     "lifetime.min",
		},
		{
			name:     "unknown texture",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"texture":"missing"}]}`,
			want:     "unknown texture",
		},
		{
			name:     "disc without emit_radius",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"shape":"disc"}]}`,
			want:     "emit_radius",
		},
		{
			name:     "ring without emit_radius",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"shape":"ring"}]}`,
			want:     "emit_radius",
		},
		{
			name:     "rect without emit_extent",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"shape":"rect"}]}`,
			want:     "emit_extent",
		},
		{
			name:     "radial velocity mode with point shape",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"velocity_mode":"radial"}]}`,
			want:     "non-point shape",
		},
		{
			name:     "tangential velocity mode with point shape",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"velocity_mode":"tangential"}]}`,
			want:     "non-point shape",
		},
		{
			name:     "fade_in equal to one",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"fade_in":1}]}`,
			want:     "fade_in",
		},
		{
			name:     "negative wobble_freq",
			manifest: `{"version":1,"effects":[{"name":"e","capacity":10,"lifetime":[1,1],"wobble_freq":-0.5}]}`,
			want:     "wobble_freq",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := asset.Load(fstest.MapFS{"manifest.json": {Data: []byte(tt.manifest)}})
			if err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadEffectsUsesLastLayerForRedeclaredEffect(t *testing.T) {
	base := fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"effects":[{"name":"glow","capacity":100,"lifetime":[1,1]}]}`)},
	}
	override := fstest.MapFS{
		"manifest.json": {Data: []byte(`{"version":1,"effects":[{"name":"glow","capacity":300,"lifetime":[2,2]}]}`)},
	}

	m, err := asset.Load(base, override)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	decl, ok := m.EffectDecl(m.Effect("glow"))
	if !ok {
		t.Fatal("EffectDecl(glow) ok = false, want true")
	}
	if decl.Capacity != 300 {
		t.Fatalf("Capacity = %d, want 300 (override layer wins)", decl.Capacity)
	}
	if decl.Lifetime != (particle.Range{Min: 2, Max: 2}) {
		t.Fatalf("Lifetime = %+v, want {2 2} (override layer wins)", decl.Lifetime)
	}
}

func TestManifestEffectZeroValueAndUnknownNamesAreSafe(t *testing.T) {
	var m asset.Manifest

	if got := m.Effect("x"); got != 0 {
		t.Fatalf("Effect(x) = %d, want 0", got)
	}
	if got := m.Effects(); len(got) != 0 {
		t.Fatalf("Effects() len = %d, want 0", len(got))
	}
	if _, ok := m.EffectDecl(1); ok {
		t.Fatal("EffectDecl(1) ok = true, want false on zero-value Manifest")
	}
}

// assertEffectDeclNear compares every particle.EffectDecl field with a
// float-tolerant check for the fields converted from degrees, so a tiny
// floating point difference in the degree-to-radian conversion doesn't
// produce a flaky failure.
func assertEffectDeclNear(t *testing.T, got, want particle.EffectDecl) {
	t.Helper()
	const eps = 1e-9

	if got.Name != want.Name {
		t.Errorf("Name = %q, want %q", got.Name, want.Name)
	}
	if got.Texture != want.Texture {
		t.Errorf("Texture = %q, want %q", got.Texture, want.Texture)
	}
	if got.Blend != want.Blend {
		t.Errorf("Blend = %v, want %v", got.Blend, want.Blend)
	}
	if got.Capacity != want.Capacity {
		t.Errorf("Capacity = %d, want %d", got.Capacity, want.Capacity)
	}
	if got.EmitRate != want.EmitRate {
		t.Errorf("EmitRate = %v, want %v", got.EmitRate, want.EmitRate)
	}
	if got.BurstSize != want.BurstSize {
		t.Errorf("BurstSize = %d, want %d", got.BurstSize, want.BurstSize)
	}
	if got.Lifetime != want.Lifetime {
		t.Errorf("Lifetime = %+v, want %+v", got.Lifetime, want.Lifetime)
	}
	if got.Speed != want.Speed {
		t.Errorf("Speed = %+v, want %+v", got.Speed, want.Speed)
	}
	if math.Abs(got.Direction-want.Direction) > eps {
		t.Errorf("Direction = %v, want %v (degrees converted to radians)", got.Direction, want.Direction)
	}
	if math.Abs(got.Spread-want.Spread) > eps {
		t.Errorf("Spread = %v, want %v (degrees converted to radians)", got.Spread, want.Spread)
	}
	if got.Shape != want.Shape {
		t.Errorf("Shape = %v, want %v", got.Shape, want.Shape)
	}
	if got.EmitRadius != want.EmitRadius {
		t.Errorf("EmitRadius = %v, want %v", got.EmitRadius, want.EmitRadius)
	}
	if got.EmitExtent != want.EmitExtent {
		t.Errorf("EmitExtent = %+v, want %+v", got.EmitExtent, want.EmitExtent)
	}
	if got.VelMode != want.VelMode {
		t.Errorf("VelMode = %v, want %v", got.VelMode, want.VelMode)
	}
	if got.Gravity != want.Gravity {
		t.Errorf("Gravity = %+v, want %+v", got.Gravity, want.Gravity)
	}
	if got.Drag != want.Drag {
		t.Errorf("Drag = %v, want %v", got.Drag, want.Drag)
	}
	if got.SizeStart != want.SizeStart {
		t.Errorf("SizeStart = %+v, want %+v", got.SizeStart, want.SizeStart)
	}
	if got.SizeEnd != want.SizeEnd {
		t.Errorf("SizeEnd = %+v, want %+v", got.SizeEnd, want.SizeEnd)
	}
	if got.ColorStart != want.ColorStart {
		t.Errorf("ColorStart = %v, want %v", got.ColorStart, want.ColorStart)
	}
	if got.ColorEnd != want.ColorEnd {
		t.Errorf("ColorEnd = %v, want %v", got.ColorEnd, want.ColorEnd)
	}
	if got.Spin != want.Spin {
		t.Errorf("Spin = %v, want %v", got.Spin, want.Spin)
	}
	if math.Abs(got.Rotation.Min-want.Rotation.Min) > eps || math.Abs(got.Rotation.Max-want.Rotation.Max) > eps {
		t.Errorf("Rotation = %+v, want %+v (degrees converted to radians)", got.Rotation, want.Rotation)
	}
	if got.WobbleAmp != want.WobbleAmp {
		t.Errorf("WobbleAmp = %v, want %v", got.WobbleAmp, want.WobbleAmp)
	}
	if got.WobbleFreq != want.WobbleFreq {
		t.Errorf("WobbleFreq = %v, want %v", got.WobbleFreq, want.WobbleFreq)
	}
	if got.FadeIn != want.FadeIn {
		t.Errorf("FadeIn = %v, want %v", got.FadeIn, want.FadeIn)
	}
}
