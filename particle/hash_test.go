package particle

import (
	"math"
	"testing"
)

// Expected outputs generated once from the exact algorithm in hash.go (LCG,
// cross-feed, xorshift-16, cross-feed) via a throwaway program, then pasted
// here as fixed fixtures. If pcg3d's arithmetic ever changes, the mirrored
// WGSL shader must change identically and these fixtures must be
// regenerated together.
func TestPcg3dMatchesFixedFixtures(t *testing.T) {
	tests := []struct {
		name string
		in   [3]uint32
		want [3]uint32
	}{
		{
			name: "all zero lanes",
			in:   [3]uint32{0, 0, 0},
			want: [3]uint32{2611992518, 2833812075, 1058359340},
		},
		{
			name: "single set lane x",
			in:   [3]uint32{1, 0, 0},
			want: [3]uint32{2717828630, 1424287207, 2065551680},
		},
		{
			name: "single set lane y",
			in:   [3]uint32{0, 1, 0},
			want: [3]uint32{450040476, 1322785807, 1953739034},
		},
		{
			name: "arbitrary spawn-like lanes",
			in:   [3]uint32{42, 1337, 7},
			want: [3]uint32{3019059356, 3459087993, 2468311016},
		},
		{
			name: "all lanes at max uint32",
			in:   [3]uint32{0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF},
			want: [3]uint32{2784268096, 2756918915, 1364953698},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pcg3d(tt.in)
			if got != tt.want {
				t.Fatalf("pcg3d(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestPcg3dIsDeterministicAcrossCalls(t *testing.T) {
	in := [3]uint32{7, 11, 13}

	first := pcg3d(in)
	second := pcg3d(in)

	if first != second {
		t.Fatalf("pcg3d(%v) not deterministic: first=%v second=%v", in, first, second)
	}
}

func TestHash3MatchesCanonicalParticleHash(t *testing.T) {
	in := [3]uint32{42, 1337, 7}

	if got, want := Hash3(in), pcg3d(in); got != want {
		t.Fatalf("Hash3(%v) = %v, want %v", in, got, want)
	}
}

func TestPcg3dDoesNotMutateItsInputArray(t *testing.T) {
	in := [3]uint32{5, 9, 1}
	original := in

	pcg3d(in)

	if in != original {
		t.Fatalf("pcg3d mutated caller's array: got %v, want unchanged %v", in, original)
	}
}

func TestFloat01MapsLaneIntoHalfOpenUnitRange(t *testing.T) {
	tests := []struct {
		name string
		in   uint32
		want float64
	}{
		{name: "zero maps to zero", in: 0, want: 0},
		{name: "half range", in: 1 << 31, want: 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Float01(tt.in); got != tt.want {
				t.Fatalf("Float01(%d) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestFloat01NeverReachesOne(t *testing.T) {
	got := Float01(math.MaxUint32)

	if got >= 1.0 {
		t.Fatalf("Float01(MaxUint32) = %v, want < 1.0", got)
	}
	if got <= 0 {
		t.Fatalf("Float01(MaxUint32) = %v, want > 0", got)
	}
}

func TestFloat01StaysWithinHalfOpenRangeAcrossSweep(t *testing.T) {
	// Sweep a spread of lanes (not exhaustive over 2^32) to confirm the
	// mapping never escapes [0, 1) at either extreme or mid-range values.
	for _, x := range []uint32{0, 1, 1 << 16, 1 << 31, math.MaxUint32 - 1, math.MaxUint32} {
		got := Float01(x)
		if got < 0 || got >= 1.0 {
			t.Fatalf("Float01(%d) = %v, want in [0, 1)", x, got)
		}
	}
}
