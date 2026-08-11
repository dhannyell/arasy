package particlegpu

import (
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

func TestPackUniformsLayoutGolden(t *testing.T) {
	decl := particle.EffectDecl{
		ColorStart: [4]float64{.1, .2, .3, .4},
		ColorEnd:   [4]float64{.5, .6, .7, .8},
		Gravity:    math2d.Vec2{X: 3, Y: -4},
		EmitExtent: math2d.Vec2{X: 5, Y: 6},
		Direction:  .25,
		Spread:     .5,
		Speed:      particle.Range{Min: 7, Max: 8},
		Lifetime:   particle.Range{Min: 9, Max: 10},
		SizeStart:  particle.Range{Min: 11, Max: 12},
		SizeEnd:    particle.Range{Min: 13, Max: 14},
		Spin:       15,
		Rotation:   particle.Range{Min: 16, Max: 17},
		WobbleAmp:  18,
		WobbleFreq: 19,
		FadeIn:     .2,
		FlipXFreq:  21,
		Shape:      particle.EmitRing,
		VelMode:    particle.VelTangential,
		EmitRadius: 22,
	}

	cmd := render.DrawParticles{
		Pos:       math2d.Vec2{X: 23, Y: 24},
		Dir:       .75,
		Dt:        1.0 / 60,
		EmitScale: math2d.Vec2{X: 2, Y: 3},
	}
	var got [UniformSize]byte
	PackUniforms(
		got[:],
		decl,
		cmd,
		particle.EmissionFrame{
			Base:    25,
			PrevPos: math2d.Vec2{X: 26, Y: 27},
		},
		28,
		0x0102030405060708,
		29,
		render.Camera{
			World:    math2d.NewRect(math2d.Vec2{X: 30, Y: 31}, math2d.Vec2{X: 190, Y: 151}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
		},
		320,
		240,
	)
	const want = "cdcccc3dcdcc4c3e9a99993ecdcccc3e0000003f9a99193f3333333fcdcc4c3f92762c3c0000008000000000b20159bcca6ba8bf698eb43f" +
		"3694d73f9771cb3fcdcccc3b898808bc0000b8410000c0410000d0410000d84100004040000080c00000a0400000c040000000400" +
		"0004040a46a57bf40510a3f8988883c000000001c0000001900000008070605040302010000803f0000003f0000e0400000004100" +
		"00104100002041000030410000404100005041000060410000704100008041000088410000904100009841cdcc4c3e0000a8410300" +
		"0000020000000000b0411d000000000000000000000000000000"
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("particle uniform layout changed:\n%s", hex.EncodeToString(got[:]))
	}
}

func TestPackedParticlePositionMatchesWorldToScreen(t *testing.T) {
	const width, height = 100, 100
	point := math2d.Vec2{X: 10, Y: 10}
	cameras := []struct {
		name   string
		camera render.Camera
	}{
		{name: "identity", camera: particleTestCamera()},
		{name: "isometric view", camera: particleViewCamera()},
		{name: "depth scale", camera: particleDepthCamera()},
	}

	for _, tc := range cameras {
		t.Run(tc.name, func(t *testing.T) {
			var packed [UniformSize]byte
			PackUniforms(packed[:], particle.EffectDecl{}, render.DrawParticles{}, particle.EmissionFrame{}, 0, 0, 1, tc.camera, width, height)

			worldToClip := [2]math2d.Vec2{
				{X: float64(readUniformF32(packed[:], 32)), Y: float64(readUniformF32(packed[:], 36))},
				{X: float64(readUniformF32(packed[:], 40)), Y: float64(readUniformF32(packed[:], 44))},
			}
			offset := math2d.Vec2{X: float64(readUniformF32(packed[:], 48)), Y: float64(readUniformF32(packed[:], 52))}
			screenToClip := math2d.Vec2{X: float64(readUniformF32(packed[:], 64)), Y: float64(readUniformF32(packed[:], 68))}
			clip := math2d.Vec2{
				X: point.X*worldToClip[0].X + point.Y*worldToClip[1].X + offset.X,
				Y: point.X*worldToClip[0].Y + point.Y*worldToClip[1].Y + offset.Y,
			}
			got := math2d.Vec2{
				X: (clip.X + 1) / screenToClip.X,
				Y: (clip.Y - 1) / screenToClip.Y,
			}
			want := tc.camera.WorldToScreen(point)
			if math.Abs(got.X-want.X) > 1e-6 || math.Abs(got.Y-want.Y) > 1e-6 {
				t.Fatalf("particle center = %v, want Camera.WorldToScreen %v", got, want)
			}
		})
	}
}

func particleTestCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
}

func particleViewCamera() render.Camera {
	camera := particleTestCamera()
	camera.View = math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
	return camera
}

func particleDepthCamera() render.Camera {
	camera := particleTestCamera()
	camera.DepthScale = 0.5
	return camera
}

func readUniformF32(data []byte, offset int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data[offset:]))
}

func TestScissorRect(t *testing.T) {
	camera := render.Camera{World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}), Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})}
	cases := []struct {
		name string
		clip math2d.Rect
		w, h int
		want [4]int
		ok   bool
	}{
		{"full", math2d.Rect{}, 100, 80, [4]int{0, 0, 100, 80}, true},
		{"inside", math2d.NewRectFromCorners(math2d.Vec2{X: 10.2, Y: 20.2}, math2d.Vec2{X: 30.1, Y: 40.1}), 100, 100, [4]int{10, 20, 21, 21}, true},
		{"outside", math2d.NewRectFromCorners(math2d.Vec2{X: 110, Y: 10}, math2d.Vec2{X: 120, Y: 20}), 100, 100, [4]int{0, 0, 100, 100}, false},
		{"empty target", math2d.Rect{}, 0, 100, [4]int{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, w, h, ok := ScissorRect(tc.clip, camera, 1, tc.w, tc.h)
			if got := [4]int{x, y, w, h}; got != tc.want || ok != tc.ok {
				t.Fatalf("ScissorRect() = %v, %v; want %v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestScissorRectWithin(t *testing.T) {
	camera := render.Camera{World: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}), Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100})}
	viewport := math2d.NewRectFromCorners(math2d.Vec2{X: 20, Y: 30}, math2d.Vec2{X: 60, Y: 70})
	cases := []struct {
		name string
		clip math2d.Rect
		want [4]int
		ok   bool
	}{
		{"viewport", math2d.Rect{}, [4]int{20, 30, 40, 40}, true},
		{"intersection", math2d.NewRectFromCorners(math2d.Vec2{X: 50, Y: 60}, math2d.Vec2{X: 80, Y: 90}), [4]int{50, 60, 10, 10}, true},
		{"outside", math2d.NewRectFromCorners(math2d.Vec2{X: 70, Y: 40}, math2d.Vec2{X: 80, Y: 50}), [4]int{70, 40, 0, 0}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, w, h, ok := ScissorRectWithin(tc.clip, camera, viewport, 1, 100, 100)
			if got := [4]int{x, y, w, h}; got != tc.want || ok != tc.ok {
				t.Fatalf("ScissorRectWithin() = %v, %v; want %v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestViewKeyIncludesCameraView(t *testing.T) {
	camera := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
	}
	projected := camera
	projected.View = math2d.Affine{A: 1, D: 1, E: 8}

	if NewViewKey(camera, 100, 100) == NewViewKey(projected, 100, 100) {
		t.Fatal("particle view key ignored Camera.View")
	}
	identity := camera
	identity.View = math2d.Identity()
	if NewViewKey(camera, 100, 100) != NewViewKey(identity, 100, 100) {
		t.Fatal("zero and explicit identity views produced different keys")
	}
}
