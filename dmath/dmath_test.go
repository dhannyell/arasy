package dmath_test

// These tests cover three layers:
//
// 1. Golden tables check the exact output bits recorded on the reference
//    platform. Other architectures run the same tables, so any divergence is
//    visible. A mismatch must be investigated rather than hidden by updating
//    the table.
// 2. Property tests check the mathematical contracts on values not covered by
//    the tables.
// 3. Informational comparisons check the distance from the platform math
//    library. This is not an exactness gate: the package promises
//    reproducibility, not libm's last bit.

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/dmath"
)

type golden1 struct {
	in   float64
	bits uint64
}

type golden2 struct {
	a, b float64
	bits uint64
}

func checkGolden1(t *testing.T, name string, fn func(float64) float64, cases []golden1) {
	t.Helper()
	for _, c := range cases {
		if got := math.Float64bits(fn(c.in)); got != c.bits {
			t.Errorf("%s(%v) = %#016x, want %#016x", name, c.in, got, c.bits)
		}
	}
}

func checkGolden2(t *testing.T, name string, fn func(a, b float64) float64, cases []golden2) {
	t.Helper()
	for _, c := range cases {
		if got := math.Float64bits(fn(c.a, c.b)); got != c.bits {
			t.Errorf("%s(%v, %v) = %#016x, want %#016x", name, c.a, c.b, got, c.bits)
		}
	}
}

func TestGoldenBits(t *testing.T) {
	t.Run("Sin", func(t *testing.T) { checkGolden1(t, "Sin", dmath.Sin, sinGolden) })
	t.Run("Cos", func(t *testing.T) { checkGolden1(t, "Cos", dmath.Cos, cosGolden) })
	t.Run("Atan2", func(t *testing.T) { checkGolden2(t, "Atan2", dmath.Atan2, atan2Golden) })
	t.Run("Exp", func(t *testing.T) { checkGolden1(t, "Exp", dmath.Exp, expGolden) })
	t.Run("Log", func(t *testing.T) { checkGolden1(t, "Log", dmath.Log, logGolden) })
	t.Run("Pow", func(t *testing.T) { checkGolden2(t, "Pow", dmath.Pow, powGolden) })
	t.Run("Asin", func(t *testing.T) { checkGolden1(t, "Asin", dmath.Asin, asinGolden) })
	t.Run("Acos", func(t *testing.T) { checkGolden1(t, "Acos", dmath.Acos, acosGolden) })
}

// TestInvalidInputsReturnNaN checks that invalid inputs return NaN instead of
// producing a misleading result.
func TestInvalidInputsReturnNaN(t *testing.T) {
	cases := []struct {
		name string
		got  float64
	}{
		{"Sin above range", dmath.Sin(1048577)},
		{"Sin below range", dmath.Sin(-1048577)},
		{"Sin of +Inf", dmath.Sin(math.Inf(1))},
		{"Cos of -Inf", dmath.Cos(math.Inf(-1))},
		{"Cos of NaN", dmath.Cos(math.NaN())},
		{"Exp of NaN", dmath.Exp(math.NaN())},
		{"Asin above 1", dmath.Asin(1.0000001)},
		{"Acos below -1", dmath.Acos(-1.0000001)},
		{"Log of negative", dmath.Log(-1)},
		{"Atan2 of NaN", dmath.Atan2(math.NaN(), 1)},
		{"Pow of negative base and fractional exponent", dmath.Pow(-2, 0.5)},
	}
	for _, c := range cases {
		if !math.IsNaN(c.got) {
			t.Errorf("%s: got %v, want NaN", c.name, c.got)
		}
	}
}

// TestSinCosPythagoreanIdentity checks the relationship between the sine and
// cosine implementations across many reduction quadrants.
func TestSinCosPythagoreanIdentity(t *testing.T) {
	for x := -300.0; x <= 300.0; x += 0.037 {
		s, c := dmath.Sin(x), dmath.Cos(x)
		if d := math.Abs(s*s + c*c - 1); d > 1e-12 {
			t.Fatalf("Sin^2+Cos^2 at x=%v deviates from 1 by %g", x, d)
		}
	}
}

// TestAcosInvertsCos checks the round trip through the cosine and arccosine
// implementations. It stays away from 0 and pi, where the inverse amplifies
// small errors.
func TestAcosInvertsCos(t *testing.T) {
	for x := 0.1; x <= math.Pi-0.1; x += 0.0137 {
		if d := math.Abs(dmath.Acos(dmath.Cos(x)) - x); d > 1e-9 {
			t.Fatalf("Acos(Cos(%v)) deviates by %g", x, d)
		}
	}
}

// TestAtan2QuadrantContract checks the signed-zero and quadrant conventions
// that are difficult to express in a table of ordinary literals.
func TestAtan2QuadrantContract(t *testing.T) {
	negZero := math.Copysign(0, -1)
	if got := dmath.Atan2(0, 1); got != 0 || math.Signbit(got) {
		t.Errorf("Atan2(0, 1) = %v, want +0", got)
	}
	if got := dmath.Atan2(negZero, 1); got != 0 || !math.Signbit(got) {
		t.Errorf("Atan2(-0, 1) = %v, want -0", got)
	}
	if got := dmath.Atan2(0, -1); got != math.Pi {
		t.Errorf("Atan2(0, -1) = %v, want pi", got)
	}
	if got := dmath.Atan2(negZero, -1); got != -math.Pi {
		t.Errorf("Atan2(-0, -1) = %v, want -pi", got)
	}
	quads := []struct{ y, x, lo, hi float64 }{
		{1, 1, 0, math.Pi / 2},
		{1, -1, math.Pi / 2, math.Pi},
		{-1, -1, -math.Pi, -math.Pi / 2},
		{-1, 1, -math.Pi / 2, 0},
	}
	for _, q := range quads {
		if a := dmath.Atan2(q.y, q.x); a <= q.lo || a >= q.hi {
			t.Errorf("Atan2(%v, %v) = %v, want inside (%v, %v)", q.y, q.x, a, q.lo, q.hi)
		}
	}
	if got := dmath.Atan2(math.Inf(1), math.Inf(1)); got != math.Pi/4 {
		t.Errorf("Atan2(+Inf, +Inf) = %v, want pi/4", got)
	}
	if got := dmath.Atan2(math.Inf(-1), math.Inf(-1)); got != -3*math.Pi/4 {
		t.Errorf("Atan2(-Inf, -Inf) = %v, want -3pi/4", got)
	}
}

// TestExpLogExtremes checks overflow, underflow, and exact anchor values from
// the function contracts.
func TestExpLogExtremes(t *testing.T) {
	if got := dmath.Exp(0); got != 1 {
		t.Errorf("Exp(0) = %v, want exactly 1", got)
	}
	if got := dmath.Exp(710); !math.IsInf(got, 1) {
		t.Errorf("Exp(710) = %v, want +Inf", got)
	}
	if got := dmath.Exp(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("Exp(+Inf) = %v, want +Inf", got)
	}
	if got := dmath.Exp(-800); got != 0 {
		t.Errorf("Exp(-800) = %v, want 0", got)
	}
	if got := dmath.Exp(math.Inf(-1)); got != 0 {
		t.Errorf("Exp(-Inf) = %v, want 0", got)
	}
	if got := dmath.Log(1); got != 0 {
		t.Errorf("Log(1) = %v, want exactly 0", got)
	}
	if got := dmath.Log(0); !math.IsInf(got, -1) {
		t.Errorf("Log(0) = %v, want -Inf", got)
	}
	if got := dmath.Log(math.Inf(1)); !math.IsInf(got, 1) {
		t.Errorf("Log(+Inf) = %v, want +Inf", got)
	}
	if got := dmath.Pow(123.456, 0); got != 1 {
		t.Errorf("Pow(123.456, 0) = %v, want exactly 1", got)
	}
	if got := dmath.Pow(0, 2); got != 0 {
		t.Errorf("Pow(0, 2) = %v, want 0", got)
	}
	if got := dmath.Pow(0, -1); !math.IsInf(got, 1) {
		t.Errorf("Pow(0, -1) = %v, want +Inf", got)
	}
}

// TestStaysNearTheStandardLibrary compares the results with the platform math
// library using a deliberately wide tolerance. It can catch a wrong
// implementation, but it is not an exactness gate: dmath promises
// reproducibility, not bit equality with libm.
func TestStaysNearTheStandardLibrary(t *testing.T) {
	const tol = 1e-9
	rel := func(got, want float64) float64 {
		if want == 0 {
			return math.Abs(got - want)
		}
		return math.Abs((got - want) / want)
	}
	check := func(name string, worst float64) {
		t.Helper()
		t.Logf("%s: max relative error %.3g", name, worst)
		if worst > tol {
			t.Errorf("%s: max relative error %.3g exceeds %g", name, worst, tol)
		}
	}

	worstSin, worstCos := 0.0, 0.0
	sweep := func(from, to, step float64) {
		for x := from; x <= to; x += step {
			if e := rel(dmath.Sin(x), math.Sin(x)); e > worstSin {
				worstSin = e
			}
			if e := rel(dmath.Cos(x), math.Cos(x)); e > worstCos {
				worstCos = e
			}
		}
	}
	sweep(-1000, 1000, 0.037)
	sweep(1048000, 1048576, 0.37) // near the supported range limit
	check("Sin", worstSin)
	check("Cos", worstCos)

	worst := 0.0
	for y := -5.0; y <= 5.0; y += 0.31 {
		for x := -5.0; x <= 5.0; x += 0.31 {
			if e := rel(dmath.Atan2(y, x), math.Atan2(y, x)); e > worst {
				worst = e
			}
		}
	}
	check("Atan2", worst)

	worst = 0.0
	for x := -700.0; x <= 700.0; x += 0.37 {
		if e := rel(dmath.Exp(x), math.Exp(x)); e > worst {
			worst = e
		}
	}
	check("Exp", worst)

	worst = 0.0
	for x := 1e-6; x <= 1e6; x *= 1.003 {
		if e := rel(dmath.Log(x), math.Log(x)); e > worst {
			worst = e
		}
	}
	check("Log", worst)

	worstAsin, worstAcos := 0.0, 0.0
	for x := -0.999; x <= 0.999; x += 0.0037 {
		if e := rel(dmath.Asin(x), math.Asin(x)); e > worstAsin {
			worstAsin = e
		}
		if e := rel(dmath.Acos(x), math.Acos(x)); e > worstAcos {
			worstAcos = e
		}
	}
	check("Asin", worstAsin)
	check("Acos", worstAcos)

	worst = 0.0
	for _, base := range []float64{0.07, 0.5, 1.5, 2, 3, 10} {
		for y := -20.0; y <= 20.0; y += 0.37 {
			if e := rel(dmath.Pow(base, y), math.Pow(base, y)); e > worst {
				worst = e
			}
		}
	}
	for y := -15.0; y <= 15.0; y++ { // negative bases need integer exponents
		if e := rel(dmath.Pow(-2, y), math.Pow(-2, y)); e > worst {
			worst = e
		}
	}
	check("Pow", worst)
}

// Golden tables recorded from the reference implementation on x86-64. They
// are shared by every supported architecture.

var sinGolden = []golden1{
	{0, 0x0000000000000000},
	{1e-09, 0x3E112E0BE826D695},
	{-1e-09, 0xBE112E0BE826D695},
	{0.1, 0x3FB98EAECB8BCB2C},
	{-0.1, 0xBFB98EAECB8BCB2C},
	{0.5, 0x3FDEAEE8744B05F0},
	{1, 0x3FEAED548F090CEE},
	{-1, 0xBFEAED548F090CEE},
	{0.5235987755982989, 0x3FE0000000000001},
	{0.7853981633974483, 0x3FE6A09E667F3BC4},
	{1.0471975511965979, 0x3FEBB67AE8584CAB},
	{1.5707963267948966, 0x3FF0000000000000},
	{2.0943951023931957, 0x3FEBB67AE8584CAA},
	{3.141592653589793, 0x3CA1A62633100000},
	{-3.141592653589793, 0xBCA1A62633100000},
	{4.71238898038469, 0xBFF0000000000000},
	{6.283185307179586, 0xBCB1A62633100000},
	{-6.283185307179586, 0x3CB1A62633100000},
	{2, 0x3FED18F6EAD1B446},
	{-2, 0xBFED18F6EAD1B446},
	{5, 0xBFEEAF81F5E09933},
	{10, 0xBFE1689EF5F34F54},
	{-10, 0x3FE1689EF5F34F54},
	{100, 0xBFE03425B78C4DB8},
	{-100, 0x3FE03425B78C4DB8},
	{1234.5678, 0x3FB3FA00084EE3DD},
	{100000, 0x3FA24DAA9C527E96},
	{-100000, 0xBFA24DAA9C527E96},
	{1.0485755e+06, 0xBFC4CB305757FA66},
	{1.048576e+06, 0x3FD526CCB2FC8655},
	{-1.048576e+06, 0xBFD526CCB2FC8655},
}

var cosGolden = []golden1{
	{0, 0x3FF0000000000000},
	{1e-09, 0x3FF0000000000000},
	{-1e-09, 0x3FF0000000000000},
	{0.1, 0x3FEFD712F9A817C0},
	{-0.1, 0x3FEFD712F9A817C0},
	{0.5, 0x3FEC1528065B7D50},
	{1, 0x3FE14A280FB5068E},
	{-1, 0x3FE14A280FB5068E},
	{0.5235987755982989, 0x3FEBB67AE8584CAA},
	{0.7853981633974483, 0x3FE6A09E667F3C85},
	{1.0471975511965979, 0x3FE0000000000000},
	{1.5707963267948966, 0x3C91A62633100000},
	{2.0943951023931957, 0xBFE0000000000002},
	{3.141592653589793, 0xBFF0000000000000},
	{-3.141592653589793, 0xBFF0000000000000},
	{4.71238898038469, 0xBCAA79394CA00000},
	{6.283185307179586, 0x3FF0000000000000},
	{-6.283185307179586, 0x3FF0000000000000},
	{2, 0xBFDAA22657537205},
	{-2, 0xBFDAA22657537205},
	{5, 0x3FD22785706B4ADA},
	{10, 0xBFEAD9AC890C6B1F},
	{-10, 0xBFEAD9AC890C6B1F},
	{100, 0x3FEB981DBF665FE0},
	{-100, 0x3FEB981DBF665FE0},
	{1234.5678, 0xBFEFE7053DC31387},
	{100000, 0xBFEFFAC3841B3DA7},
	{-100000, 0xBFEFFAC3841B3DA7},
	{1.0485755e+06, 0x3FEF932EC65B1D71},
	{1.048576e+06, 0x3FEE33ADA92FE2AE},
	{-1.048576e+06, 0x3FEE33ADA92FE2AE},
}

var atan2Golden = []golden2{
	{0, 1, 0x0000000000000000},
	{0, -1, 0x400921FB54442D18},
	{1, 0, 0x3FF921FB54442D18},
	{-1, 0, 0xBFF921FB54442D18},
	{1, 1, 0x3FE921FB54442D18},
	{1, -1, 0x4002D97C7F3321D2},
	{-1, 1, 0xBFE921FB54442D18},
	{-1, -1, 0xC002D97C7F3321D2},
	{2, 1, 0x3FF1B6E192EBC5A3},
	{1, 2, 0x3FDDAC6705619DD3},
	{-3, 4, 0xBFE4978FA3269EE1},
	{3, -4, 0x4003FC176B7A8560},
	{5, -12, 0x4005F97315270DDA},
	{-5, -12, 0xC005F97315270DDA},
	{0.5, 0.8660254037844386, 0x3FE0C152382D734C},
	{1e-09, 1, 0x3E112E0BE826D695},
	{1, 1e-09, 0x3FF921FB53FF74E8},
	{123.456, -78.9, 0x40011DA0A7C4A94E},
	{1e+300, 1e+300, 0x3FE921FB54442D18},
	{1e-300, 1e-300, 0x3FE921FB54442D18},
	{0.41421356237309503, 1, 0x3FD921FB545464AB},
	{1, 0.41421356237309503, 0x3FF2D97C7F2F13ED},
}

var expGolden = []golden1{
	{0, 0x3FF0000000000000},
	{1, 0x4005BF0A8B145662},
	{-1, 0x3FD78B56362CF12B},
	{0.5, 0x3FFA61298E1E069F},
	{-0.5, 0x3FE368B2FC6F9609},
	{0.001, 0x3FF0041919B7EE33},
	{-0.001, 0x3FEFF7CFE56F1A9E},
	{1e-10, 0x3FF000000006DF38},
	{2, 0x401D8E64B8D4DDAD},
	{-2, 0x3FC152AAA3BF81CC},
	{5.5, 0x406E96244F21BBF6},
	{-5.5, 0x3F70BD4A5ACA7729},
	{10, 0x40D5829DCF9504B0},
	{-10, 0x3F07CD79B5647DEA},
	{20, 0x41BCEB088B68E804},
	{-20, 0x3E21B48655F37266},
	{100, 0x48F3494A9B171BF4},
	{-100, 0x36EA8C1F14E2AF5F},
	{700, 0x7F0D945DF4F8EC8E},
	{-700, 0x00D14F2B0FB9307F},
	{709.7, 0x7FED75AE7A50EE14},
	{-708, 0x0017C8AB2288CB05},
	{0.34657359027997264, 0x3FF6A09E667F4338},
	{-0.34657359027997264, 0x3FE6A09E667F37DE},
}

var logGolden = []golden1{
	{1, 0x0000000000000000},
	{2, 0x3FE62E42FEFA39EF},
	{0.5, 0xBFE62E42FEFA39EF},
	{2.718281828459045, 0x3FEFFFFFFFFFFD61},
	{10, 0x40026BB1BBB55514},
	{100, 0x40126BB1BBB55519},
	{1000, 0x401BA18A998FFFA0},
	{1e+10, 0x4037069E2AA2AA5B},
	{1e-10, 0xC037069E2AA2AA5B},
	{0.1, 0xC0026BB1BBB55514},
	{3, 0x3FF193EA7AAD038C},
	{7, 0x3FFF2272AE325A57},
	{1.5, 0x3FD9F323ECBF9A4F},
	{0.75, 0xBFD269621134D98F},
	{1e+300, 0x4085963447F87FB6},
	{1e-300, 0xC085963447F87FB6},
	{2.2250738585072014e-308, 0xC086232BDD7ABCD2},
	{5e-324, 0xC0874385446D71C3},
	{1.4142135623730951, 0x3FD62E42FEFA1A49},
	{0.7071067811865476, 0xBFD62E42FEFA5995},
}

var powGolden = []golden2{
	{2, 10, 0x4090000000000000},
	{2, 0.5, 0x3FF6A09E667F4338},
	{2, -2, 0x3FD0000000000000},
	{10, 3, 0x408F3FFFFFFFFFEE},
	{0.5, 2, 0x3FD0000000000000},
	{1.5, 2.5, 0x40060B9FD68A4567},
	{9, 0.5, 0x40080000000000F7},
	{2, 100, 0x4630000000000008},
	{-2, 3, 0xC01FFFFFFFFFFFFE},
	{-2, 4, 0x402FFFFFFFFFFFFF},
	{-3, 5, 0xC06E6000000004CC},
	{-1, 101, 0xBFF0000000000000},
	{7, 0, 0x3FF0000000000000},
	{2.718281828459045, 1, 0x4005BF0A8B14549B},
}

var asinGolden = []golden1{
	{-1, 0xBFF921FB54442D18},
	{-0.99, 0xBFF6DE3C6F33D51D},
	{-0.75, 0xBFEB235315C680DC},
	{-0.5, 0xBFE0C152382D734C},
	{-0.1, 0xBFB9A49276037884},
	{0, 0x0000000000000000},
	{0.1, 0x3FB9A49276037884},
	{0.5, 0x3FE0C152382D734C},
	{0.7071067811865476, 0x3FE921FB54442D18},
	{0.75, 0x3FEB235315C680DC},
	{0.99, 0x3FF6DE3C6F33D51D},
	{1, 0x3FF921FB54442D18},
}

var acosGolden = []golden1{
	{-1, 0x400921FB54442D18},
	{-0.99, 0x4008001BE1BC011A},
	{-0.75, 0x400359D26F93B6C3},
	{-0.5, 0x4000C152382D735F},
	{-0.1, 0x3FFABC447BA464A0},
	{0, 0x3FF921FB54442D18},
	{0.1, 0x3FF787B22CE3F590},
	{0.5, 0x3FF0C152382D7372},
	{0.7071067811865476, 0x3FE921FB54442D18},
	{0.75, 0x3FE720A392C1D954},
	{0.99, 0x3FC21DF72882BFD8},
	{1, 0x0000000000000000},
}
