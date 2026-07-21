package renderbatch_test

import (
	"math"
	"testing"

	"github.com/dhannyell/frameforge/internal/renderbatch"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestWrapTime(t *testing.T) {
	cases := []struct {
		time float64
		want float64
	}{
		{0, 0},
		{12.5, 12.5},
		{4095.75, 4095.75},
		{4096, 0},
		{4097.25, 1.25},
		{8192 + 3.5, 3.5},
		{-1, 4095},
	}
	for _, tc := range cases {
		assertNear(t, "WrapTime", float64(renderbatch.WrapTime(tc.time)), tc.want, 1e-3)
	}
}

func TestFractPhase(t *testing.T) {
	cases := []struct {
		phase float32
		want  float64
	}{
		{0, 0},
		{0.25, 0.25},
		{1.25, 0.25},
		{-0.25, 0.75},
		{3.75, 0.75},
	}
	for _, tc := range cases {
		assertNear(t, "FractPhase", float64(renderbatch.FractPhase(tc.phase)), tc.want, 1e-6)
	}
}

// sheetInstance returns an instance on a 128x64 texture of 32x32 cells
// (4 cells per row, 2 rows, capacity 8), with the base cell at (0, 0).
func sheetInstance() render.SpriteInstance {
	return render.SpriteInstance{
		Dest: rect(0, 0, 32, 32),
		Src:  rect(0, 0, 32, 32),
		Animation: render.SpriteAnimation{
			Frames: 4,
			Rate:   2,
		},
	}
}

func TestAnimatedSrcZeroValueIsStatic(t *testing.T) {
	inst := render.SpriteInstance{Src: rect(8, 8, 24, 24)}
	if got := renderbatch.AnimatedSrc(inst, 3.5, 128, 64); got != inst.Src {
		t.Fatalf("AnimatedSrc = %v, want static Src %v", got, inst.Src)
	}
}

// TestAnimatedSrcDegeneratesToStatic covers every invalid-input case that
// must keep the static Src.
func TestAnimatedSrcDegeneratesToStatic(t *testing.T) {
	base := rect(0, 0, 32, 32)
	cases := []struct {
		name       string
		src        math2d.Rect
		anim       render.SpriteAnimation
		texW, texH int
	}{
		{"one frame", base, render.SpriteAnimation{Frames: 1, Rate: 2}, 128, 64},
		{"zero rate", base, render.SpriteAnimation{Frames: 4, Rate: 0}, 128, 64},
		{"negative rate", base, render.SpriteAnimation{Frames: 4, Rate: -1}, 128, 64},
		{"zero src", math2d.Rect{}, render.SpriteAnimation{Frames: 4, Rate: 2}, 128, 64},
		{"negative src origin", rect(-1, 0, 31, 32), render.SpriteAnimation{Frames: 4, Rate: 2}, 128, 64},
		{"src outside texture", rect(0, 0, 32, 96), render.SpriteAnimation{Frames: 2, Rate: 2}, 128, 64},
		{"cell wider than texture", base, render.SpriteAnimation{Frames: 4, Rate: 2}, 16, 64},
		{"zero texture size", base, render.SpriteAnimation{Frames: 4, Rate: 2}, 0, 64},
		{"sequence runs past the texture", base, render.SpriteAnimation{Frames: 9, Rate: 2}, 128, 64},
		{"sequence runs past from a late base cell", rect(64, 32, 96, 64), render.SpriteAnimation{Frames: 3, Rate: 2}, 128, 64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst := render.SpriteInstance{Src: tc.src, Animation: tc.anim}
			// Sample several times: no time may leave the static Src.
			for _, time := range []float64{0, 0.5, 1.3, 7.9} {
				if got := renderbatch.AnimatedSrc(inst, time, tc.texW, tc.texH); got != tc.src {
					t.Fatalf("time %v: AnimatedSrc = %v, want static Src %v", time, got, tc.src)
				}
			}
		})
	}
}

func TestAnimatedSrcSelectsFrameByRate(t *testing.T) {
	inst := sheetInstance() // Frames 4, Rate 2 on a 4x2 grid.
	cases := []struct {
		time float64
		want math2d.Rect
	}{
		{0, rect(0, 0, 32, 32)},
		{0.49, rect(0, 0, 32, 32)},
		{0.5, rect(32, 0, 64, 32)},
		{1.0, rect(64, 0, 96, 32)},
		{1.5, rect(96, 0, 128, 32)},
		{2.0, rect(0, 0, 32, 32)}, // step wraps mod Frames
		{2.5, rect(32, 0, 64, 32)},
	}
	for _, tc := range cases {
		if got := renderbatch.AnimatedSrc(inst, tc.time, 128, 64); got != tc.want {
			t.Fatalf("time %v: AnimatedSrc = %v, want %v", tc.time, got, tc.want)
		}
	}
}

func TestAnimatedSrcAdvancesRowMajor(t *testing.T) {
	inst := sheetInstance()
	inst.Animation.Frames = 6
	inst.Animation.Rate = 1
	cases := []struct {
		time float64
		want math2d.Rect
	}{
		{3, rect(96, 0, 128, 32)}, // frame 3: last cell of row 0
		{4, rect(0, 32, 32, 64)},  // frame 4: first cell of row 1
		{5, rect(32, 32, 64, 64)}, // frame 5
		{6, rect(0, 0, 32, 32)},   // step 6 wraps mod 6
	}
	for _, tc := range cases {
		if got := renderbatch.AnimatedSrc(inst, tc.time, 128, 64); got != tc.want {
			t.Fatalf("time %v: AnimatedSrc = %v, want %v", tc.time, got, tc.want)
		}
	}
}

func TestAnimatedSrcAppliesPhase(t *testing.T) {
	inst := sheetInstance()
	inst.Animation.Rate = 1
	for _, phase := range []float32{0.5, 1.5, -0.5} {
		inst.Animation.Phase = phase
		// phase 0.5 * 4 frames starts the sheet at frame 2.
		if got := renderbatch.AnimatedSrc(inst, 0, 128, 64); got != rect(64, 0, 96, 32) {
			t.Fatalf("phase %v: AnimatedSrc = %v, want frame 2 at (64,0)", phase, got)
		}
	}
}

func TestAnimatedSrcAdvancesFromBaseCell(t *testing.T) {
	inst := sheetInstance()
	inst.Src = rect(32, 32, 64, 64) // row 1, col 1 (grid index 5)
	inst.Animation.Frames = 2
	inst.Animation.Rate = 1

	if got := renderbatch.AnimatedSrc(inst, 0, 128, 64); got != rect(32, 32, 64, 64) {
		t.Fatalf("frame 0: AnimatedSrc = %v, want the base cell", got)
	}
	if got := renderbatch.AnimatedSrc(inst, 1, 128, 64); got != rect(64, 32, 96, 64) {
		t.Fatalf("frame 1: AnimatedSrc = %v, want the next cell row-major", got)
	}
}

func TestAnimatedSrcWrapsTimeAt4096(t *testing.T) {
	inst := sheetInstance()
	inst.Animation.Rate = 1.5
	inst.Animation.Phase = 0.3
	for _, time := range []float64{0, 3.7, 123.25, 4095.9} {
		got := renderbatch.AnimatedSrc(inst, time+4096, 128, 64)
		want := renderbatch.AnimatedSrc(inst, time, 128, 64)
		if got != want {
			t.Fatalf("time %v: AnimatedSrc(t+4096) = %v, want %v", time, got, want)
		}
	}
}

func TestBounceOffsetYZeroWhenInactive(t *testing.T) {
	bouncing := render.SpriteInstance{
		Dest:      rect(0, 0, 10, 10),
		Animation: render.SpriteAnimation{Bounce: 10},
	}
	cases := []struct {
		name string
		inst render.SpriteInstance
		anim render.SpriteBatchAnimation
	}{
		{"zero value", render.SpriteInstance{}, render.SpriteBatchAnimation{}},
		{"no frequency", bouncing, render.SpriteBatchAnimation{BounceHz: 0, Intensity: 1}},
		{"no intensity", bouncing, render.SpriteBatchAnimation{BounceHz: 1, Intensity: 0}},
		{"no bounce", render.SpriteInstance{}, render.SpriteBatchAnimation{BounceHz: 1, Intensity: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, time := range []float64{0, 0.25, 1.3} {
				tc.anim.Time = time
				if got := renderbatch.BounceOffsetY(tc.inst, tc.anim); got != 0 {
					t.Fatalf("time %v: BounceOffsetY = %v, want 0", time, got)
				}
			}
		})
	}
}

func TestBounceOffsetYSineShape(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:      rect(0, 0, 10, 10),
		Animation: render.SpriteAnimation{Bounce: 10},
	}
	anim := render.SpriteBatchAnimation{BounceHz: 1, Intensity: 2}

	// The bounce lifts sprites up (negative Y) only on the positive sine
	// half-wave.
	cases := []struct {
		time float64
		want float64
	}{
		{0, 0},
		{0.25, -20}, // sin(pi/2) = 1, full lift
		{0.5, 0},
		{0.75, 0}, // negative half-wave is clamped
		{1.25, -20},
	}
	for _, tc := range cases {
		anim.Time = tc.time
		assertNear(t, "offsetY", renderbatch.BounceOffsetY(inst, anim), tc.want, 1e-4)
	}
}

func TestBounceOffsetYAppliesPhase(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:      rect(0, 0, 10, 10),
		Animation: render.SpriteAnimation{Bounce: 10, Phase: 0.25},
	}
	anim := render.SpriteBatchAnimation{BounceHz: 1, Intensity: 1, Time: 0}

	// phase 0.25 starts the sine at its peak.
	assertNear(t, "offsetY", renderbatch.BounceOffsetY(inst, anim), -10, 1e-4)
}

func TestBounceOffsetYPulseBoost(t *testing.T) {
	anim := render.SpriteBatchAnimation{
		Time:      0.25, // sine peak
		BounceHz:  1,
		Intensity: 1,
		Pulse: render.SpritePulse{
			Center: math2d.Vec2{X: 100, Y: 100},
			Radius: 0,
			Width:  5,
			Boost:  2,
		},
	}

	// Instance centered exactly on the pulse center: d = 0, gaussian = 1,
	// boost = 1 + 2 = 3.
	near := render.SpriteInstance{
		Dest:      rect(95, 95, 105, 105),
		Animation: render.SpriteAnimation{Bounce: 10},
	}
	assertNear(t, "near pulse", renderbatch.BounceOffsetY(near, anim), -30, 1e-3)

	// Instance on the pulse ring: distance == Radius, again d = 0.
	anim.Pulse.Radius = 20
	ring := render.SpriteInstance{
		Dest:      rect(115, 95, 125, 105), // center (120,100), distance 20
		Animation: render.SpriteAnimation{Bounce: 10},
	}
	assertNear(t, "on ring", renderbatch.BounceOffsetY(ring, anim), -30, 1e-3)

	// Instance far off the ring: the gaussian decays to ~0 and boost ~1.
	far := render.SpriteInstance{
		Dest:      rect(0, 0, 10, 10),
		Animation: render.SpriteAnimation{Bounce: 10},
	}
	assertNear(t, "far from pulse", renderbatch.BounceOffsetY(far, anim), -10, 1e-3)
}

func TestBounceOffsetYPulseDisabled(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:      rect(95, 95, 105, 105),
		Animation: render.SpriteAnimation{Bounce: 10},
	}
	base := render.SpriteBatchAnimation{Time: 0.25, BounceHz: 1, Intensity: 1}

	for _, pulse := range []render.SpritePulse{
		{Center: math2d.Vec2{X: 100, Y: 100}, Width: 0, Boost: 5},  // no width
		{Center: math2d.Vec2{X: 100, Y: 100}, Width: -1, Boost: 5}, // negative width
		{Center: math2d.Vec2{X: 100, Y: 100}, Width: 5, Boost: 0},  // no boost
	} {
		anim := base
		anim.Pulse = pulse
		assertNear(t, "offsetY", renderbatch.BounceOffsetY(inst, anim), -10, 1e-4)
	}
}

func TestBounceOffsetYWrapsTimeAt4096(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:      rect(0, 0, 10, 10),
		Animation: render.SpriteAnimation{Bounce: 7, Phase: 0.1},
	}
	anim := render.SpriteBatchAnimation{BounceHz: 0.8, Intensity: 1.3}
	for _, time := range []float64{0.2, 3.7, 123.25} {
		anim.Time = time
		want := renderbatch.BounceOffsetY(inst, anim)
		anim.Time = time + 4096
		assertNear(t, "offsetY", renderbatch.BounceOffsetY(inst, anim), want, 1e-4)
	}
}

func TestFlipBookCellsValidSequence(t *testing.T) {
	// 8x4 texture of 2x2 cells: 4 cells per row, 2 rows.
	inst := render.SpriteInstance{
		Src:       rect(2, 2, 4, 4), // base cell: col 1, row 1 -> index 5
		Animation: render.SpriteAnimation{Frames: 3, Rate: 1},
	}
	cellsPerRow, base, ok := renderbatch.FlipBookCells(inst, 8, 4)
	if !ok {
		t.Fatal("FlipBookCells rejected a valid sequence")
	}
	if cellsPerRow != 4 || base != 5 {
		t.Fatalf("FlipBookCells = (%d, %d), want (4, 5)", cellsPerRow, base)
	}
}

func TestFlipBookCellsDegradesLikeAnimatedSrc(t *testing.T) {
	cases := map[string]render.SpriteInstance{
		"one frame":      {Src: rect(0, 0, 2, 2), Animation: render.SpriteAnimation{Frames: 1, Rate: 1}},
		"no rate":        {Src: rect(0, 0, 2, 2), Animation: render.SpriteAnimation{Frames: 4}},
		"zero src":       {Animation: render.SpriteAnimation{Frames: 4, Rate: 1}},
		"src outside":    {Src: rect(8, 0, 10, 2), Animation: render.SpriteAnimation{Frames: 4, Rate: 1}},
		"sequence spills": {Src: rect(4, 2, 6, 4), Animation: render.SpriteAnimation{Frames: 4, Rate: 1}},
	}
	for name, inst := range cases {
		if _, _, ok := renderbatch.FlipBookCells(inst, 8, 4); ok {
			t.Fatalf("%s: FlipBookCells ok, want degradation", name)
		}
		// AnimatedSrc must agree: degradation there means the static Src.
		if got := renderbatch.AnimatedSrc(inst, 1.5, 8, 4); got != inst.Src {
			t.Fatalf("%s: AnimatedSrc = %+v, want static %+v", name, got, inst.Src)
		}
	}
	if _, _, ok := renderbatch.FlipBookCells(render.SpriteInstance{
		Src: rect(0, 0, 2, 2), Animation: render.SpriteAnimation{Frames: 4, Rate: 1},
	}, 0, 4); ok {
		t.Fatal("zero texture width must degrade")
	}
}

func TestMaxVerticalInflationMatchesPerInstance(t *testing.T) {
	instances := []render.SpriteInstance{
		{Animation: render.SpriteAnimation{Bounce: 2}},
		{Animation: render.SpriteAnimation{Bounce: -7}},
		{Animation: render.SpriteAnimation{Bounce: 3}},
	}
	anim := render.SpriteBatchAnimation{BounceHz: 1, Intensity: 1.5, Pulse: render.SpritePulse{Boost: 0.5}}

	maxBounce := 0.0
	maxPerInstance := 0.0
	for _, inst := range instances {
		if b := math.Abs(float64(inst.Animation.Bounce)); b > maxBounce {
			maxBounce = b
		}
		if v := renderbatch.VerticalInflation(inst, anim); v > maxPerInstance {
			maxPerInstance = v
		}
	}
	assertNear(t, "inflation", renderbatch.MaxVerticalInflation(maxBounce, anim), maxPerInstance, 1e-12)

	inactive := render.SpriteBatchAnimation{BounceHz: 0, Intensity: 1.5}
	if got := renderbatch.MaxVerticalInflation(maxBounce, inactive); got != 0 {
		t.Fatalf("inactive bounce inflation = %v, want 0", got)
	}
}
