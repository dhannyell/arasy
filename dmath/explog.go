package dmath

import "math"

// ln2Hi and ln2Lo are the Cody-Waite split of ln(2). The high part keeps 33
// mantissa bits, so its product with the reduction multiple is exact within
// Exp's supported range.
const (
	ln2Hi = 6.93147180369123816490e-01
	ln2Lo = 1.90821492927058770002e-10
	log2E = 1.4426950408889634 // 1/ln(2)
)

// minNormal is the smallest positive normal float64, 2^-1022.
const minNormal = 2.2250738585072014e-308

// Exp returns the reproducible e**x. It uses Cody-Waite range reduction, a
// Taylor polynomial for the reduced interval, and direct exponent-bit scaling.
//
// Subnormal results are flushed to zero. The effective underflow threshold is
// therefore -1022*ln(2), instead of the standard library's -745. Gradual
// underflow is intentionally omitted from the simulation path.
func Exp(x float64) float64 {
	if math.IsNaN(x) {
		return math.NaN()
	}
	if x > 709.782712893384 { // largest x with finite e**x
		return math.Inf(1)
	}
	if x < -708.396418532264 { // e**x is subnormal below this value
		return 0.0
	}

	// Reduce x to x = k*ln(2) + r with |r| <= ln(2)/2.
	kr := float64(x * log2E) // rounded so the +0.5 below cannot fuse
	var k int64
	if kr >= 0 {
		k = int64(kr + 0.5)
	} else {
		k = int64(kr - 0.5)
	}
	kf := float64(k)
	r := (x - float64(kf*ln2Hi)) - float64(kf*ln2Lo)

	// Evaluate the Taylor polynomial for e**r through r^10.
	p := 1.0 / 3628800
	p = 1.0/362880 + float64(r*p)
	p = 1.0/40320 + float64(r*p)
	p = 1.0/5040 + float64(r*p)
	p = 1.0/720 + float64(r*p)
	p = 1.0/120 + float64(r*p)
	p = 1.0/24 + float64(r*p)
	p = 1.0/6 + float64(r*p)
	p = 0.5 + float64(r*p)
	rr := r * r
	expR := (1.0 + r) + float64(rr*p)

	// Scale by 2^k by editing the exponent bits instead of calling math.Ldexp.
	bits := math.Float64bits(expR)
	e := int64((bits>>52)&0x7FF) + k
	if e <= 0 {
		return 0.0 // subnormal result; see the Exp contract
	}
	if e >= 0x7FF {
		return math.Inf(1)
	}
	bits = (bits &^ (0x7FF << 52)) | (uint64(e) << 52)
	return math.Float64frombits(bits)
}

// Log returns the reproducible natural logarithm of x. Log(0) is -Inf;
// negative inputs and NaN return NaN.
func Log(x float64) float64 {
	switch {
	case math.IsNaN(x) || x < 0:
		return math.NaN()
	case x == 0:
		return math.Inf(-1)
	case math.IsInf(x, 1):
		return math.Inf(1)
	}

	// Extract the exponent and mantissa from the bits. Scale subnormals by 2^54
	// first; the operation is exact and gives them a usable exponent field.
	exp := int64(0)
	if x < minNormal {
		x *= 1 << 54
		exp = -54
	}
	bits := math.Float64bits(x)
	exp += int64((bits>>52)&0x7FF) - 1023
	// Normalize the mantissa to f in [1, 2).
	f := math.Float64frombits((bits &^ (0x7FF << 52)) | (1023 << 52))
	if f > 1.4142135623730951 { // balance f around 1 for the series
		f *= 0.5
		exp++
	}

	// Evaluate log(f) with the series for log((1+z)/(1-z)), where
	// z = (f-1)/(f+1) and |z| <= 0.1716.
	z := (f - 1.0) / (f + 1.0)
	z2 := z * z
	p := 2.0 / 13
	p = 2.0/11 + float64(z2*p)
	p = 2.0/9 + float64(z2*p)
	p = 2.0/7 + float64(z2*p)
	p = 2.0/5 + float64(z2*p)
	p = 2.0/3 + float64(z2*p)
	// The conversion is load-bearing: assignment alone does not round, and
	// arm64 would fuse this product into the sum on the return line.
	logF := float64(z * (2.0 + float64(z2*p)))

	kf := float64(exp)
	return float64(kf*ln2Hi) + (logF + float64(kf*ln2Lo))
}

// Pow returns the reproducible x**y as Exp(y * Log(x)). Negative bases
// require an integer y; other negative-base inputs return NaN. Special cases
// follow simplified simulation conventions rather than the full math table.
func Pow(x, y float64) float64 {
	switch {
	case y == 0:
		return 1.0
	case math.IsNaN(x) || math.IsNaN(y):
		return math.NaN()
	case x == 0:
		if y > 0 {
			return 0.0
		}
		return math.Inf(1)
	}
	if x < 0 {
		isInt, odd := isOddInt(y)
		if !isInt {
			return math.NaN()
		}
		r := Exp(float64(y * Log(-x)))
		if odd {
			return -r
		}
		return r
	}
	return Exp(float64(y * Log(x)))
}

// isOddInt reports whether y is an integer and whether it is odd.
func isOddInt(y float64) (isInt, odd bool) {
	// At 2^53 and above (including Inf) every float64 is an even integer,
	// and converting it to int64 would be implementation-defined.
	if y >= 9007199254740992.0 || y <= -9007199254740992.0 {
		return true, false
	}
	i := int64(y)
	if float64(i) != y {
		return false, false
	}
	return true, i&1 != 0
}
