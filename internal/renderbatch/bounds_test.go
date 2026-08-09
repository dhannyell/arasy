package renderbatch_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func rect(minX, minY, maxX, maxY float64) math2d.Rect {
	return math2d.Rect{Min: math2d.Vec2{X: minX, Y: minY}, Max: math2d.Vec2{X: maxX, Y: maxY}}
}

func TestIsFiniteRect(t *testing.T) {
	cases := []struct {
		name string
		r    math2d.Rect
		want bool
	}{
		{"zero value", math2d.Rect{}, true},
		{"ordinary", rect(1, 2, 3, 4), true},
		{"NaN Min", rect(math.NaN(), 0, 1, 1), false},
		{"NaN Max", rect(0, 0, 1, math.NaN()), false},
		{"+Inf", rect(0, math.Inf(1), 1, 2), false},
		{"-Inf", rect(math.Inf(-1), 0, 1, 2), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderbatch.IsFiniteRect(tc.r); got != tc.want {
				t.Fatalf("IsFiniteRect(%v) = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

func TestSpriteBoundsAxisAligned(t *testing.T) {
	inst := render.SpriteInstance{Dest: rect(10, 20, 30, 40)}
	bounds, ok := renderbatch.SpriteBounds(inst)
	if !ok {
		t.Fatal("SpriteBounds reported non-finite for an ordinary rect")
	}
	assertRectNear(t, bounds, inst.Dest, 1e-12)
}

func TestSpriteBoundsRotatedQuarterTurn(t *testing.T) {
	// A 10x4 rect centered at (5,2) rotated by pi/2 swaps its extents.
	inst := render.SpriteInstance{Dest: rect(0, 0, 10, 4), Rotation: math.Pi / 2}
	bounds, ok := renderbatch.SpriteBounds(inst)
	if !ok {
		t.Fatal("SpriteBounds reported non-finite for an ordinary rect")
	}
	assertRectNear(t, bounds, rect(3, -3, 7, 7), 1e-9)
}

func TestSpriteBoundsRotated45Degrees(t *testing.T) {
	// A 2x2 square centered at the origin rotated by pi/4 has half-extent
	// sqrt(2) on both axes.
	inst := render.SpriteInstance{Dest: rect(-1, -1, 1, 1), Rotation: math.Pi / 4}
	bounds, ok := renderbatch.SpriteBounds(inst)
	if !ok {
		t.Fatal("SpriteBounds reported non-finite for an ordinary rect")
	}
	want := math.Sqrt(2)
	assertRectNear(t, bounds, rect(-want, -want, want, want), 1e-9)
}

func TestSpriteBoundsDegenerateDest(t *testing.T) {
	// A zero-size Dest bounds to its own point; adapters drop it later when
	// the texture lookup rejects it.
	inst := render.SpriteInstance{Dest: rect(4, 4, 4, 4), Rotation: 1}
	bounds, ok := renderbatch.SpriteBounds(inst)
	if !ok {
		t.Fatal("SpriteBounds reported non-finite for a degenerate rect")
	}
	assertRectNear(t, bounds, rect(4, 4, 4, 4), 1e-12)
}

func TestSpriteBoundsNonFiniteDestIsRejected(t *testing.T) {
	cases := []math2d.Rect{
		rect(math.NaN(), 0, 1, 1),
		rect(0, 0, math.Inf(1), 1),
		rect(0, math.Inf(-1), 1, 1),
	}
	for _, dest := range cases {
		if _, ok := renderbatch.SpriteBounds(render.SpriteInstance{Dest: dest}); ok {
			t.Fatalf("SpriteBounds(%v) ok = true, want false so the adapter drops it", dest)
		}
	}
}

func TestSpriteBoundsNonFiniteRotationIsRejected(t *testing.T) {
	inst := render.SpriteInstance{Dest: rect(0, 0, 1, 1), Rotation: math.NaN()}
	if _, ok := renderbatch.SpriteBounds(inst); ok {
		t.Fatal("SpriteBounds accepted a non-finite rotation")
	}
}

func TestSpriteBoundsForCameraMatchesScaleThenRotateUnderAspectStretch(t *testing.T) {
	camera := render.Camera{
		World:    rect(0, 0, 100, 100),
		Viewport: rect(0, 0, 200, 100),
		Aspect:   render.AspectStretch,
	}
	inst := render.SpriteInstance{Dest: rect(49, 40, 51, 60), Rotation: math.Pi / 2}
	got, ok := renderbatch.SpriteBoundsForCamera(inst, camera)
	if !ok {
		t.Fatal("SpriteBoundsForCamera rejected finite geometry")
	}
	assertRectNear(t, got, rect(45, 48, 55, 52), 1e-9)
	if renderbatch.WorldBoundsPreserveScreenRotation(camera) {
		t.Fatal("anisotropic AspectStretch reported world-rotation bounds as conservative")
	}
}

func TestWorldBoundsPreserveScreenRotationWithUniformScale(t *testing.T) {
	camera := render.Camera{World: rect(0, 0, 100, 50), Viewport: rect(0, 0, 200, 100)}
	if !renderbatch.WorldBoundsPreserveScreenRotation(camera) {
		t.Fatal("uniform camera scale must preserve world-rotation bounds")
	}
}

func TestBoundsForViewMakesVisibleIsometricBatchReachable(t *testing.T) {
	camera := render.Camera{
		World:    rect(0, 0, 100, 100),
		Viewport: rect(0, 0, 100, 100),
		View: math2d.Affine{
			A: 1, B: 0.5, C: -1, D: 0.5, E: -50,
		},
	}
	view := camera.Resolve().World
	outsideWorld := rect(110, 10, 112, 12)
	if renderbatch.IntersectsView(outsideWorld, view) {
		t.Fatal("test premise: the untransformed batch must be outside the resolved view")
	}

	transformed := renderbatch.BoundsForView(outsideWorld, camera.View)
	if !renderbatch.IntersectsView(transformed, view) {
		t.Fatalf("transformed batch = %v must remain visible in %v", transformed, view)
	}
}

func TestBoundsForViewKeepsGenuinelyOutsideBatchCulled(t *testing.T) {
	view := rect(0, 0, 100, 100)
	transform := math2d.Affine{A: 1, B: 0.5, C: -1, D: 0.5, E: -50}
	outside := renderbatch.BoundsForView(rect(200, 200, 202, 202), transform)
	if renderbatch.IntersectsView(outside, view) {
		t.Fatalf("transformed genuinely outside batch = %v intersects %v", outside, view)
	}
}

func TestBoundsForViewIdentityPreservesBounds(t *testing.T) {
	want := rect(-3.25, 1.5, 7.75, 9.25)
	if got := renderbatch.BoundsForView(want, math2d.Affine{}); got != want {
		t.Fatalf("identity transform changed bounds: got %v, want %v", got, want)
	}
}

func TestBatchBoundsUnionsFiniteInstances(t *testing.T) {
	instances := []render.SpriteInstance{
		{Dest: rect(0, 0, 10, 10)},
		{Dest: rect(-1, -1, 1, 1), Rotation: math.Pi / 4},
		{Dest: rect(math.NaN(), 0, 1, 1)}, // skipped, not fatal
		{Dest: rect(20, 5, 30, 8)},
	}

	bounds, ok := renderbatch.BatchBounds(instances)
	if !ok {
		t.Fatal("BatchBounds ok = false, want true")
	}
	want := math.Sqrt(2)
	assertRectNear(t, bounds, rect(-want, -want, 30, 10), 1e-9)
}

func TestBatchBoundsWithoutFiniteInstance(t *testing.T) {
	if _, ok := renderbatch.BatchBounds(nil); ok {
		t.Fatal("BatchBounds(nil) ok = true, want false")
	}
	invalid := []render.SpriteInstance{{Dest: rect(math.NaN(), 0, 1, 1)}}
	if _, ok := renderbatch.BatchBounds(invalid); ok {
		t.Fatal("BatchBounds(all non-finite) ok = true, want false")
	}
}

func TestVerticalInflationFormula(t *testing.T) {
	anim := render.SpriteBatchAnimation{
		BounceHz:  1,
		Intensity: 1.5,
		Pulse:     render.SpritePulse{Width: 4, Boost: 3},
	}
	inst := render.SpriteInstance{Animation: render.SpriteAnimation{Bounce: -8}}

	// abs(Bounce*Intensity) * (1 + abs(Pulse.Boost)) = 8*1.5 * 4 = 48.
	assertNear(t, "inflation", renderbatch.VerticalInflation(inst, anim), 48, 1e-9)
}

func TestVerticalInflationZeroWhenBounceInactive(t *testing.T) {
	inst := render.SpriteInstance{Animation: render.SpriteAnimation{Bounce: 8}}
	cases := []render.SpriteBatchAnimation{
		{},                           // zero value
		{BounceHz: 0, Intensity: 1},  // no frequency
		{BounceHz: -1, Intensity: 1}, // negative frequency
		{BounceHz: 1, Intensity: 0},  // no intensity
		{BounceHz: 1, Intensity: 2},  // active: not zero
	}
	for i, anim := range cases[:4] {
		if got := renderbatch.VerticalInflation(inst, anim); got != 0 {
			t.Fatalf("case %d: VerticalInflation = %v, want 0", i, got)
		}
	}
	if got := renderbatch.VerticalInflation(inst, cases[4]); got != 16 {
		t.Fatalf("active case: VerticalInflation = %v, want 16", got)
	}
}

// TestVerticalInflationNeverUnderestimates samples the real bounce offset
// across time and asserts the conservative inflation always covers it, for
// negative bounce, negative intensity and an off-center pulse.
func TestVerticalInflationNeverUnderestimates(t *testing.T) {
	inst := render.SpriteInstance{
		Dest:      rect(10, 10, 20, 20),
		Animation: render.SpriteAnimation{Bounce: -8, Phase: 0.3},
	}
	anim := render.SpriteBatchAnimation{
		BounceHz:  0.7,
		Intensity: -1.5,
		Pulse: render.SpritePulse{
			Center: math2d.Vec2{X: 40, Y: 25},
			Radius: 12,
			Width:  6,
			Boost:  3,
		},
	}
	inflation := renderbatch.VerticalInflation(inst, anim)

	for time := 0.0; time <= 12.0; time += 0.013 {
		anim.Time = time
		if got := math.Abs(renderbatch.BounceOffsetY(inst, anim)); got > inflation+1e-4 {
			t.Fatalf("time %v: |offsetY| = %v exceeds inflation %v", time, got, inflation)
		}
	}
}

func TestInflatedBatchBoundsExpandsVertically(t *testing.T) {
	instances := []render.SpriteInstance{
		{Dest: rect(0, 0, 10, 10), Animation: render.SpriteAnimation{Bounce: 2}},
		{Dest: rect(20, 20, 30, 30), Animation: render.SpriteAnimation{Bounce: -5}},
	}
	anim := render.SpriteBatchAnimation{BounceHz: 1, Intensity: 2}

	bounds, ok := renderbatch.InflatedBatchBounds(instances, anim)
	if !ok {
		t.Fatal("InflatedBatchBounds ok = false, want true")
	}
	// Max inflation = abs(-5*2) = 10, expanded on both vertical sides.
	assertRectNear(t, bounds, rect(0, -10, 30, 40), 1e-9)
}

func TestIntersectsView(t *testing.T) {
	view := rect(0, 0, 100, 100)
	cases := []struct {
		name   string
		bounds math2d.Rect
		want   bool
	}{
		{"inside", rect(10, 10, 20, 20), true},
		{"straddling edge", rect(90, 90, 120, 120), true},
		{"containing view", rect(-10, -10, 200, 200), true},
		{"fully left", rect(-50, 10, -10, 20), false},
		{"fully right", rect(110, 10, 120, 20), false},
		{"fully above", rect(10, -50, 20, -10), false},
		{"fully below", rect(10, 110, 20, 120), false},
		// Touching without area is not visible; the sprite covers no pixel.
		{"touching corner", rect(100, 100, 120, 120), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderbatch.IntersectsView(tc.bounds, view); got != tc.want {
				t.Fatalf("IntersectsView(%v) = %v, want %v", tc.bounds, got, tc.want)
			}
		})
	}
}
