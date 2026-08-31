package fixedprobe

import (
	"math/bits"

	"github.com/dhannyell/fixed"
)

// preparedDivider divides many numerators by one invariant denominator
// through a precomputed reciprocal (Möller–Granlund). It reproduces
// fixed.Q.Div bit for bit over Raw values; only the package saturation
// counter is out of reach from here.
type preparedDivider struct {
	d   uint64 // divisor magnitude
	dn  uint64 // divisor magnitude, normalized to the top bit
	s   uint   // normalization shift
	v   uint64 // reciprocal word of dn
	neg bool   // divisor sign
}

func prepareDivider(o fixed.Q) preparedDivider {
	raw := o.Raw()
	if raw == 0 {
		panic("fixedprobe: division by zero")
	}
	d := rawMagnitude(raw)
	s := uint(bits.LeadingZeros64(d))
	dn := d << s
	// v = floor((2¹²⁸-1)/dn) - 2⁶⁴; the Div64 precondition holds because
	// dn is normalized.
	v, _ := bits.Div64(^dn, ^uint64(0), dn)
	return preparedDivider{d: d, dn: dn, s: s, v: v, neg: raw < 0}
}

// div returns q divided by the prepared denominator, equal to
// q.Div(denom) bit for bit.
func (p preparedDivider) div(q fixed.Q) fixed.Q {
	raw := q.Raw()
	n := rawMagnitude(raw)
	neg := (raw < 0) != p.neg
	hi, lo := n>>32, n<<32
	if hi >= p.d {
		if neg {
			return fixed.MinValue()
		}
		return fixed.MaxValue()
	}
	// Shift the 96-bit numerator into the normalized frame. The quotient
	// keeps its value: both sides scale by 2^s.
	u1 := hi<<p.s | lo>>(64-p.s)
	u0 := lo << p.s
	// 2-by-1 division by the invariant word: one high multiply and at
	// most two unit corrections replace the hardware divide.
	q1, q0 := bits.Mul64(p.v, u1)
	var c uint64
	q0, c = bits.Add64(q0, u0, 0)
	q1, _ = bits.Add64(q1, u1, c)
	q1++
	r := u0 - q1*p.dn
	if r > q0 {
		q1--
		r += p.dn
	}
	if r >= p.dn {
		q1++
	}
	if neg {
		if q1 > 1<<63 {
			return fixed.MinValue()
		}
		return fixed.FromRaw(-int64(q1))
	}
	if q1 > 1<<63-1 {
		return fixed.MaxValue()
	}
	return fixed.FromRaw(int64(q1))
}

func rawMagnitude(v int64) uint64 {
	m := uint64(v >> 63)
	return (uint64(v) ^ m) - m
}
