package raster

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/asset/pixel"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

const continuousAlphaSmokeManifest = `{
	"version": 1,
	"textures": [{
		"name": "soft_smoke",
		"file": "textures/soft_smoke.png"
	}],
	"effects": [{
		"name": "smoke",
		"texture": "soft_smoke",
		"blend": "alpha",
		"capacity": 160,
		"emit_rate": 16,
		"burst_size": 0,
		"lifetime": [3.4, 4.6],
		"speed": [10, 17],
		"direction": -90,
		"spread": 18,
		"shape": "disc",
		"emit_radius": 2,
		"velocity_mode": "cone",
		"gravity": [0, -0.8],
		"drag": 0.96,
		"size_start": [5, 8],
		"size_end": [20, 32],
		"color_start": [0.68, 0.72, 0.78, 0.48],
		"color_end": [0.24, 0.27, 0.31, 0],
		"spin": 25,
		"rotation": [0, 360],
		"wobble_amp": 1.8,
		"wobble_freq": 0.55,
		"fade_in": 0.15
	}]
}`

const additiveFireManifest = `{
	"version": 1,
	"textures": [{
		"name": "soft_fire",
		"file": "textures/soft_fire.png"
	}],
	"effects": [{
		"name": "fire",
		"texture": "soft_fire",
		"blend": "additive",
		"capacity": 128,
		"emit_rate": 30,
		"burst_size": 0,
		"lifetime": [0.65, 1.25],
		"speed": [8, 15],
		"direction": -90,
		"spread": 20,
		"shape": "disc",
		"emit_radius": 1.2,
		"velocity_mode": "cone",
		"gravity": [0, -5],
		"drag": 0.82,
		"size_start": [4, 7],
		"size_end": [11, 18],
		"color_start": [1, 0.72, 0.16, 0.9],
		"color_end": [1, 0.05, 0, 0],
		"spin": 60,
		"rotation": [0, 360],
		"wobble_amp": 0.5,
		"wobble_freq": 1.4,
		"fade_in": 0.08
	}]
}`

const singleBurstManifest = `{
	"version": 1,
	"textures": [{
		"name": "soft_spark",
		"file": "textures/soft_spark.png"
	}],
	"effects": [{
		"name": "spark_burst",
		"texture": "soft_spark",
		"blend": "additive",
		"capacity": 64,
		"emit_rate": 0,
		"burst_size": 48,
		"lifetime": [0.8, 1.4],
		"speed": [18, 34],
		"direction": -90,
		"spread": 180,
		"shape": "point",
		"velocity_mode": "cone",
		"gravity": [0, 14],
		"drag": 0.96,
		"size_start": [2, 3.5],
		"size_end": [0.8, 1.5],
		"color_start": [1, 0.78, 0.22, 1],
		"color_end": [1, 0.12, 0, 0],
		"spin": 180,
		"rotation": [0, 360],
		"fade_in": 0.02
	}]
}`

const untexturedParticlesManifest = `{
	"version": 1,
	"effects": [{
		"name": "untextured_fountain",
		"blend": "alpha",
		"capacity": 64,
		"emit_rate": 10,
		"burst_size": 0,
		"lifetime": [2.2, 3],
		"speed": [14, 24],
		"direction": -90,
		"spread": 38,
		"shape": "point",
		"velocity_mode": "cone",
		"gravity": [0, 10],
		"drag": 0.97,
		"size_start": [2.5, 4.5],
		"size_end": [1.5, 3],
		"color_start": [0.15, 0.85, 1, 0.9],
		"color_end": [0.1, 0.25, 1, 0.12],
		"spin": 120,
		"rotation": [0, 360],
		"fade_in": 0.08
	}]
}`

const rectWobbleSnowManifest = `{
	"version": 1,
	"textures": [{
		"name": "snowflake",
		"file": "textures/snowflake.png"
	}],
	"effects": [{
		"name": "snow",
		"texture": "snowflake",
		"blend": "alpha",
		"capacity": 256,
		"emit_rate": 20,
		"burst_size": 0,
		"lifetime": [6, 8],
		"speed": [8, 14],
		"direction": 90,
		"spread": 6,
		"shape": "rect",
		"emit_extent": [46, 1],
		"velocity_mode": "cone",
		"gravity": [0, 2],
		"drag": 0.99,
		"size_start": [1.5, 3.2],
		"size_end": [1.2, 2.8],
		"color_start": [0.92, 0.96, 1, 0.92],
		"color_end": [0.72, 0.84, 1, 0.68],
		"spin": 40,
		"rotation": [0, 360],
		"wobble_amp": 2.2,
		"wobble_freq": 0.45,
		"fade_in": 0.08
	}]
}`

const ringRadialShockwaveManifest = `{
	"version": 1,
	"textures": [{
		"name": "soft_wave",
		"file": "textures/soft_wave.png"
	}],
	"effects": [{
		"name": "shockwave",
		"texture": "soft_wave",
		"blend": "additive",
		"capacity": 128,
		"emit_rate": 0,
		"burst_size": 96,
		"lifetime": [0.75, 1],
		"speed": [18, 22],
		"direction": 0,
		"spread": 0,
		"shape": "ring",
		"emit_radius": 6,
		"velocity_mode": "radial",
		"gravity": [0, 0],
		"drag": 1,
		"size_start": [2.2, 3.4],
		"size_end": [4.5, 6.5],
		"color_start": [0.25, 0.85, 1, 0.85],
		"color_end": [0.05, 0.2, 1, 0],
		"spin": 0,
		"rotation": [0, 360],
		"fade_in": 0.03
	}]
}`

func TestGoldenParticlesContinuousAlphaSmoke(t *testing.T) {
	renderer, manifest := newGoldenParticleRenderer(
		t,
		continuousAlphaSmokeManifest,
		"textures/soft_smoke.png",
		softParticlePNG(t),
	)

	effect := manifest.Effect("smoke")
	const dt = 1.0 / 60.0
	var frame *image.RGBA
	for step := range 150 {
		frame = renderer.Render([]render.Command{render.DrawParticles{
			Effect:    effect,
			Emitter:   1,
			Pos:       math2d.Vec2{X: 50, Y: 68},
			Intensity: 1,
			Time:      float64(step+1) * dt,
			Dt:        dt,
			Seed:      0x5eed,
		}})
	}

	assertGolden(t, "particles_continuous_alpha_smoke", frame)
}

func TestGoldenParticlesAdditiveFire(t *testing.T) {
	renderer, manifest := newGoldenParticleRenderer(
		t,
		additiveFireManifest,
		"textures/soft_fire.png",
		softParticlePNG(t),
	)

	effect := manifest.Effect("fire")
	const dt = 1.0 / 60.0
	var frame *image.RGBA
	for step := range 75 {
		frame = renderer.Render([]render.Command{render.DrawParticles{
			Effect:    effect,
			Emitter:   1,
			Pos:       math2d.Vec2{X: 50, Y: 66},
			Intensity: 1,
			Time:      float64(step+1) * dt,
			Dt:        dt,
			Seed:      0xf1ae,
		}})
	}

	assertGolden(t, "particles_additive_fire", frame)
}

func TestGoldenParticlesSingleBurst(t *testing.T) {
	renderer, manifest := newGoldenParticleRenderer(
		t,
		singleBurstManifest,
		"textures/soft_spark.png",
		softParticlePNG(t),
	)

	effect := manifest.Effect("spark_burst")
	const dt = 1.0 / 60.0
	position := math2d.Vec2{X: 50, Y: 37.5}
	renderer.Render([]render.Command{render.DrawParticles{
		Effect:  effect,
		Emitter: 1,
		Pos:     position,
		Dt:      dt,
		Seed:    0xb17,
	}})

	var frame *image.RGBA
	for step := range 22 {
		frame = renderer.Render([]render.Command{render.DrawParticles{
			Effect:  effect,
			Emitter: 1,
			Pos:     position,
			Burst:   1,
			Time:    float64(step+2) * dt,
			Dt:      dt,
			Seed:    0xb17,
		}})
	}

	assertGolden(t, "particles_single_burst", frame)
}

func TestGoldenParticlesUntextured(t *testing.T) {
	renderer, manifest := newGoldenParticleRenderer(
		t,
		untexturedParticlesManifest,
		"",
		nil,
	)

	effect := manifest.Effect("untextured_fountain")
	const dt = 1.0 / 60.0
	var frame *image.RGBA
	for step := range 110 {
		frame = renderer.Render([]render.Command{render.DrawParticles{
			Effect:    effect,
			Emitter:   1,
			Pos:       math2d.Vec2{X: 50, Y: 68},
			Intensity: 1,
			Time:      float64(step+1) * dt,
			Dt:        dt,
			Seed:      0x5011d,
		}})
	}

	assertGolden(t, "particles_untextured", frame)
}

func TestGoldenParticlesRectWobbleSnow(t *testing.T) {
	renderer, manifest := newGoldenParticleRenderer(
		t,
		rectWobbleSnowManifest,
		"textures/snowflake.png",
		snowParticlePNG(t),
	)

	effect := manifest.Effect("snow")
	const dt = 1.0 / 60.0
	var frame *image.RGBA
	for step := range 240 {
		frame = renderer.Render([]render.Command{render.DrawParticles{
			Effect:    effect,
			Emitter:   1,
			Pos:       math2d.Vec2{X: 50, Y: 4},
			Intensity: 1,
			Time:      float64(step+1) * dt,
			Dt:        dt,
			Seed:      0x5a0f,
		}})
	}

	assertGolden(t, "particles_rect_wobble_snow", frame)
}

func TestGoldenParticlesRingRadialShockwave(t *testing.T) {
	renderer, manifest := newGoldenParticleRenderer(
		t,
		ringRadialShockwaveManifest,
		"textures/soft_wave.png",
		softParticlePNG(t),
	)

	effect := manifest.Effect("shockwave")
	const dt = 1.0 / 60.0
	position := math2d.Vec2{X: 50, Y: 37.5}
	renderer.Render([]render.Command{render.DrawParticles{
		Effect:  effect,
		Emitter: 1,
		Pos:     position,
		Dt:      dt,
		Seed:    0x51a7,
	}})

	var frame *image.RGBA
	for step := range 24 {
		frame = renderer.Render([]render.Command{render.DrawParticles{
			Effect:  effect,
			Emitter: 1,
			Pos:     position,
			Burst:   1,
			Time:    float64(step+2) * dt,
			Dt:      dt,
			Seed:    0x51a7,
		}})
	}

	assertGolden(t, "particles_ring_radial_shockwave", frame)
}

func newGoldenParticleRenderer(
	t *testing.T,
	manifestJSON string,
	texturePath string,
	textureData []byte,
) (*Renderer, asset.Manifest) {
	t.Helper()

	files := fstest.MapFS{
		"manifest.json": {Data: []byte(manifestJSON)},
	}
	if texturePath != "" {
		files[texturePath] = &fstest.MapFile{Data: textureData}
	}

	manifest, err := asset.Load(files)
	if err != nil {
		t.Fatalf("load particle golden manifest: %v", err)
	}

	textures, err := pixel.BuildRegistry(manifest)
	if err != nil {
		t.Fatalf("build particle golden textures: %v", err)
	}

	renderer := goldenRenderer()
	renderer.Assets = manifest
	renderer.Textures = textures
	return renderer, manifest
}

func softParticlePNG(t *testing.T) []byte {
	t.Helper()

	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	const radiusSquared = size * size
	for y := range size {
		for x := range size {
			dx := 2*x + 1 - size
			dy := 2*y + 1 - size
			distanceSquared := dx*dx + dy*dy
			if distanceSquared >= radiusSquared {
				continue
			}

			alpha := uint8((radiusSquared - distanceSquared) * 190 / radiusSquared)
			img.SetRGBA(x, y, color.RGBA{R: alpha, G: alpha, B: alpha, A: alpha})
		}
	}
	return encodeParticlePNG(t, img)
}

func snowParticlePNG(t *testing.T) []byte {
	t.Helper()

	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			dx := 2*x + 1 - size
			dy := 2*y + 1 - size
			adx := absInt(dx)
			ady := absInt(dy)
			distanceSquared := dx*dx + dy*dy

			alpha := 0
			if distanceSquared < 100 {
				alpha = 235 - distanceSquared
			}

			armLength := max(adx, ady)
			if armLength <= 22 && (adx <= 2 || ady <= 2 || absInt(dx-dy) <= 2 || absInt(dx+dy) <= 2) {
				alpha = max(alpha, 215-armLength*5)
			}
			if alpha <= 0 {
				continue
			}

			a := uint8(min(alpha, 255))
			img.SetRGBA(x, y, color.RGBA{R: a, G: a, B: a, A: a})
		}
	}
	return encodeParticlePNG(t, img)
}

func encodeParticlePNG(t *testing.T, img *image.RGBA) []byte {
	t.Helper()

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatalf("encode soft particle texture: %v", err)
	}
	return encoded.Bytes()
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
