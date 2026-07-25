package particlegpu

import (
	"encoding/hex"
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
	const want = "cdcccc3dcdcc4c3e9a99993ecdcccc3e0000003f9a99193f3333333fcdcc4c3f92762c3cb20159bcca6ba8bf698eb43f3" +
		"694d73f9771cb3fcdcccc3b898808bc0000b8410000c0410000d0410000d84100004040000080c00000a0400000c040000000400" +
		"0004040a46a57bf40510a3f8988883c000000001c0000001900000008070605040302010000803f0000003f0000e0400000004100" +
		"00104100002041000030410000404100005041000060410000704100008041000088410000904100009841cdcc4c3e0000a8410300" +
		"0000020000000000b0411d00000000000000"
	if hex.EncodeToString(got[:]) != want {
		t.Fatalf("particle uniform layout changed:\n%s", hex.EncodeToString(got[:]))
	}
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
