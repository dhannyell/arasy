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
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"strings"
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
	// Budget 2026-08-31: twice the 2⁻²⁰ kernel error.
	for x := -300.0; x <= 300.0; x += 0.037 {
		s, c := dmath.Sin(x), dmath.Cos(x)
		if d := math.Abs(s*s + c*c - 1); d > 2e-6 {
			t.Fatalf("Sin^2+Cos^2 at x=%v deviates from 1 by %g", x, d)
		}
	}
}

// TestAcosInvertsCos checks the round trip through the cosine and arccosine
// implementations. It stays away from 0 and pi, where the inverse amplifies
// small errors.
func TestAcosInvertsCos(t *testing.T) {
	// Budget 2026-08-31: the 2⁻²⁰ kernel error of Cos is
	// amplified by 1/sin(x), up to 10 at the interval ends, plus the
	// error of Acos itself.
	for x := 0.1; x <= math.Pi-0.1; x += 0.0137 {
		if d := math.Abs(dmath.Acos(dmath.Cos(x)) - x); d > 2e-5 {
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
	// The float bodies (Exp, Log, Pow) hold a relative tolerance. The
	// integer trig bodies hold an absolute one: the kernel promises
	// absolute error 2⁻²⁰, and near the zeros of sine a relative error
	// carries no information. (Re-calibrated 2026-08-31.)
	const tol = 1e-9
	const trigTol = 2e-6
	rel := func(got, want float64) float64 {
		if want == 0 {
			return math.Abs(got - want)
		}
		return math.Abs((got - want) / want)
	}
	abs := func(got, want float64) float64 {
		return math.Abs(got - want)
	}
	check := func(name string, worst, limit float64) {
		t.Helper()
		t.Logf("%s: max error %.3g (limit %.3g)", name, worst, limit)
		if worst > limit {
			t.Errorf("%s: max error %.3g exceeds %g", name, worst, limit)
		}
	}

	worstSin, worstCos := 0.0, 0.0
	sweep := func(from, to, step float64) {
		for x := from; x <= to; x += step {
			if e := abs(dmath.Sin(x), math.Sin(x)); e > worstSin {
				worstSin = e
			}
			if e := abs(dmath.Cos(x), math.Cos(x)); e > worstCos {
				worstCos = e
			}
		}
	}
	sweep(-1000, 1000, 0.037)
	sweep(1048000, 1048576, 0.37) // near the supported range limit
	check("Sin", worstSin, trigTol)
	check("Cos", worstCos, trigTol)

	worst := 0.0
	for y := -5.0; y <= 5.0; y += 0.31 {
		for x := -5.0; x <= 5.0; x += 0.31 {
			if e := abs(dmath.Atan2(y, x), math.Atan2(y, x)); e > worst {
				worst = e
			}
		}
	}
	check("Atan2", worst, trigTol)

	worst = 0.0
	for x := -700.0; x <= 700.0; x += 0.37 {
		if e := rel(dmath.Exp(x), math.Exp(x)); e > worst {
			worst = e
		}
	}
	check("Exp", worst, tol)

	worst = 0.0
	for x := 1e-6; x <= 1e6; x *= 1.003 {
		if e := rel(dmath.Log(x), math.Log(x)); e > worst {
			worst = e
		}
	}
	check("Log", worst, tol)

	worstAsin, worstAcos := 0.0, 0.0
	for x := -0.999; x <= 0.999; x += 0.0037 {
		if e := abs(dmath.Asin(x), math.Asin(x)); e > worstAsin {
			worstAsin = e
		}
		if e := abs(dmath.Acos(x), math.Acos(x)); e > worstAcos {
			worstAcos = e
		}
	}
	check("Asin", worstAsin, trigTol)
	check("Acos", worstAcos, trigTol)

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
	check("Pow", worst, tol)
}

// Golden tables recorded from the reference implementation on x86-64. They
// are shared by every supported architecture.
//
// 2026-08-31: trigonometry bits re-pinned — the bodies now delegate to the
// integer kernel of the fixed module. The Exp/Log/Pow goldens are untouched.

var sinGolden = []golden1{
	{0, 0x0000000000000000},
	{1e-09, 0x0000000000000000},
	{-1e-09, 0x0000000000000000},
	{0.1, 0x3FB98EAE7A000000},
	{-0.1, 0xBFB98EAE7A000000},
	{0.5, 0x3FDEAEE856000000},
	{1, 0x3FEAED545E600000},
	{-1, 0xBFEAED545E600000},
	{0.5235987755982989, 0x3FDFFFFF73000000},
	{0.7853981633974483, 0x3FE6A09E66800000},
	{1.0471975511965979, 0x3FEBB67A6E800000},
	{1.5707963267948966, 0x3FF0000000000000},
	{2.0943951023931957, 0x3FEBB67A6EE00000},
	{3.141592653589793, 0x0000000000000000},
	{-3.141592653589793, 0x0000000000000000},
	{4.71238898038469, 0xBFF0000000000000},
	{6.283185307179586, 0x0000000000000000},
	{-6.283185307179586, 0x0000000000000000},
	{2, 0x3FED18F68E000000},
	{-2, 0xBFED18F68E000000},
	{5, 0xBFEEAF815EA00000},
	{10, 0xBFE1689EF0C00000},
	{-10, 0x3FE1689EF0C00000},
	{100, 0xBFE0342592600000},
	{-100, 0x3FE0342592600000},
	{1234.5678, 0x3FB3F9FFF1000000},
	{100000, 0x3FA24DAA50000000},
	{-100000, 0xBFA24DAA50000000},
	{1.0485755e+06, 0xBFC4CB2FF9000000},
	{1.048576e+06, 0x3FD526CC4C800000},
	{-1.048576e+06, 0xBFD526CC4C800000},
}

var cosGolden = []golden1{
	{0, 0x3FF0000000000000},
	{1e-09, 0x3FF0000000000000},
	{-1e-09, 0x3FF0000000000000},
	{0.1, 0x3FEFD71299000000},
	{-0.1, 0x3FEFD71299000000},
	{0.5, 0x3FEC1527EBE00000},
	{1, 0x3FE14A27F1000000},
	{-1, 0x3FE14A27F1000000},
	{0.5235987755982989, 0x3FEBB67A6EE00000},
	{0.7853981633974483, 0x3FE6A09E66800000},
	{1.0471975511965979, 0x3FDFFFFF74400000},
	{1.5707963267948966, 0x0000000000000000},
	{2.0943951023931957, 0xBFDFFFFF73000000},
	{3.141592653589793, 0xBFF0000000000000},
	{-3.141592653589793, 0xBFF0000000000000},
	{4.71238898038469, 0x0000000000000000},
	{6.283185307179586, 0x3FF0000000000000},
	{-6.283185307179586, 0x3FF0000000000000},
	{2, 0xBFDAA22602400000},
	{-2, 0xBFDAA22602400000},
	{5, 0x3FD2278515800000},
	{10, 0xBFEAD9AC82400000},
	{-10, 0xBFEAD9AC82400000},
	{100, 0x3FEB981D7F600000},
	{-100, 0x3FEB981D7F600000},
	{1234.5678, 0xBFEFE70510400000},
	{100000, 0xBFEFFAC2FD200000},
	{-100000, 0xBFEFFAC2FD200000},
	{1.0485755e+06, 0x3FEF932E34800000},
	{1.048576e+06, 0x3FEE33AD17A00000},
	{-1.048576e+06, 0x3FEE33AD17A00000},
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
	{2, 1, 0x3FF1B6E192C53D24},
	{1, 2, 0x3FDDAC6705FBBFD1},
	{-3, 4, 0xBFE4978FA355AA3A},
	{3, -4, 0x4003FC176B6EC28A},
	{5, -12, 0x4005F9731DE3BD70},
	{-5, -12, 0xC005F9731DE3BD70},
	{0.5, 0.8660254037844386, 0x3FE0C1521C6D4338},
	{1e-09, 1, 0x0000000000000000},
	{1, 1e-09, 0x3FF921FB54442D18},
	{123.456, -78.9, 0x40011DA09D9D4D0A},
	{1e+300, 1e+300, 0x3FE921FB54442D18},
	{1e-300, 1e-300, 0x3FE921FB54442D18},
	{0.41421356237309503, 1, 0x3FD921FB2B6CF4AF},
	{1, 0.41421356237309503, 0x3FF2D97C8968EFEC},
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
	{-0.99, 0xBFF6DE3C71FC6436},
	{-0.75, 0xBFEB235320FCAED9},
	{-0.5, 0xBFE0C1521C6D4338},
	{-0.1, 0xBFB9A492578A22C4},
	{0, 0x0000000000000000},
	{0.1, 0x3FB9A492578A22C4},
	{0.5, 0x3FE0C1521C6D4338},
	{0.7071067811865476, 0x3FE921FB54442D18},
	{0.75, 0x3FEB235320FCAED9},
	{0.99, 0x3FF6DE3C71FC6436},
	{1, 0x3FF921FB54442D18},
}

var acosGolden = []golden1{
	{-1, 0x400921FB54442D18},
	{-0.99, 0x4008001BE32048A7},
	{-0.75, 0x400359D272614242},
	{-0.5, 0x4000C152313D675A},
	{-0.1, 0x3FFABC4479BCCF44},
	{0, 0x3FF921FB54442D18},
	{0.1, 0x3FF787B22ECB8AEC},
	{0.5, 0x3FF0C152460D8B7C},
	{0.7071067811865476, 0x3FE921FB54442D18},
	{0.75, 0x3FE720A3878BAB57},
	{0.99, 0x3FC21DF7123E4713},
	{1, 0x0000000000000000},
}

// TestImplementationUsesOnlyAllowedMathHelpers gives the helper whitelist
// from doc.go a machine guard. The forbidigo pattern only bans
// transcendentals, so a future platform-dependent math helper would land
// in dmath without tripping any gate. Every math reference in the
// implementation files must be on this list.
func TestImplementationUsesOnlyAllowedMathHelpers(t *testing.T) {
	allowed := map[string]bool{
		"Sqrt": true, "Pi": true,
		"Abs": true, "IsNaN": true, "IsInf": true, "Signbit": true,
		"Float64bits": true, "Float64frombits": true,
		"NaN": true, "Inf": true,
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "math" {
				return true
			}
			if !allowed[sel.Sel.Name] {
				t.Errorf("%s: math.%s is not on the allowed helper list in doc.go",
					fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}
