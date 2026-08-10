package particle

// pcg3d hashes three 32-bit lanes (Jarzynski & Olano, JCGT 2020). The WGSL
// shader must implement the identical sequence so CPU and GPU spawn
// identical particles for the same seed.
func pcg3d(v [3]uint32) [3]uint32 {
	// 1. LCG multiplication by magic constants (Hull-Dobell)
	v[0] = v[0]*1664525 + 1013904223
	v[1] = v[1]*1664525 + 1013904223
	v[2] = v[2]*1664525 + 1013904223

	// 2. First cross-feed (cross-mixing between channels)
	v[0] += v[1] * v[2]
	v[1] += v[2] * v[0]
	v[2] += v[0] * v[1]

	// 3. XOR-Shift permutation. Prevent Marsaglia's theorem problem
	// This 'injects' chaos into the predictable values that the LCG generates on the right.
	v[0] ^= v[0] >> 16
	v[1] ^= v[1] >> 16
	v[2] ^= v[2] >> 16

	//4. Second cross-feed
	v[0] += v[1] * v[2]
	v[1] += v[2] * v[0]
	v[2] += v[0] * v[1]

	return v
}

// Hash3 exposes the canonical three-lane hash to other render-only systems.
// The particle simulator and post-processing effects therefore share one
// deterministic sequence instead of carrying duplicate implementations.
func Hash3(v [3]uint32) [3]uint32 { return pcg3d(v) }

// Float01 maps one hashed lane to [0,1). Divide by 2^32, not 2^32-1, so 1.0
// is never produced; the WGSL shader must use the same constant.
func Float01(x uint32) float64 { return float64(x) / (1 << 32) }
