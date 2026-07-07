package runtime

import "testing"

func TestRandUint64KnownAnswers(t *testing.T) {
	tests := []struct {
		name string
		seed uint64
		want []uint64
	}{
		{
			name: "seed zero",
			seed: 0,
			want: []uint64{
				0xE220A8397B1DCDAF,
				0x6E789E6AA1B965F4,
				0x06C45D188009454F,
				0xF88BB8A8724C81EC,
			},
		},
		{
			name: "seed one",
			seed: 1,
			want: []uint64{
				0x910A2DEC89025CC1,
				0xBEEB8DA1658EEC67,
				0xF893A2EEFB32555E,
				0x71C18690EE42C90B,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rng := NewRand(tt.seed)
			for i, want := range tt.want {
				got := rng.Uint64()
				if got != want {
					t.Fatalf("draw %d = %#016x, want %#016x", i, got, want)
				}
			}
		})
	}
}

func TestRandUint64KnownAnswersAreReplayContract(t *testing.T) {
	// These values are a replay contract. If this test breaks, recorded
	// replays broke; never update the values, fix the regression.
	rng := NewRand(0)
	if got, want := rng.Uint64(), uint64(0xE220A8397B1DCDAF); got != want {
		t.Fatalf("first splitmix64 vector = %#016x, want %#016x", got, want)
	}
}

func TestRandFloat64Range(t *testing.T) {
	rng := NewRand(0)
	for i := 0; i < 1000; i++ {
		got := rng.Float64()
		if got < 0 || got >= 1 {
			t.Fatalf("draw %d = %v, want in [0, 1)", i, got)
		}
	}
}

func TestRandIntNRangeAndKnownAnswers(t *testing.T) {
	rng := NewRand(0)
	want := []int{5, 2, 0, 5}
	for i, want := range want {
		got := rng.IntN(6)
		if got != want {
			t.Fatalf("IntN(6) draw %d = %d, want %d", i, got, want)
		}
	}

	for i := 0; i < 1000; i++ {
		got := rng.IntN(6)
		if got < 0 || got >= 6 {
			t.Fatalf("draw %d = %d, want in [0, 6)", i, got)
		}
	}
}

func TestRandIntNPanicsOnNonPositiveN(t *testing.T) {
	for _, n := range []int{0, -1} {
		t.Run("n", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("IntN(%d) did not panic", n)
				}
			}()
			rng := NewRand(0)
			_ = rng.IntN(n)
		})
	}
}

func TestRandUint64NPanicsOnZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Uint64N(0) did not panic")
		}
	}()
	rng := NewRand(0)
	_ = rng.Uint64N(0)
}

func TestRandSameSeedProducesSameSequence(t *testing.T) {
	a := NewRand(42)
	b := NewRand(42)
	for i := 0; i < 100; i++ {
		gotA := a.Uint64()
		gotB := b.Uint64()
		if gotA != gotB {
			t.Fatalf("draw %d differs: %016x != %016x", i, gotA, gotB)
		}
	}
}

func TestRandDeriveDoesNotAdvanceParent(t *testing.T) {
	parent := NewRand(123)
	before := parent
	_ = parent.Derive("perception")
	if parent != before {
		t.Fatalf("parent changed after Derive: got %+v, want %+v", parent, before)
	}
}

func TestRandDeriveKnownAnswers(t *testing.T) {
	child := NewRand(123).Derive("perception")
	want := []uint64{
		0xCC56BC944396BAC0,
		0xDFF9C88F40FCDE5D,
	}

	for i, want := range want {
		got := child.Uint64()
		if got != want {
			t.Fatalf("derived draw %d = %#016x, want %#016x", i, got, want)
		}
	}
}

func TestRandDeriveLabelSeparatesStreams(t *testing.T) {
	perception := NewRand(123).Derive("perception")
	execution := NewRand(123).Derive("execution")
	for i := 0; i < 4; i++ {
		gotPerception := perception.Uint64()
		gotExecution := execution.Uint64()
		if gotPerception == gotExecution {
			t.Fatalf("derived labels produced same draw %d: %#016x", i, gotPerception)
		}
	}
}

func TestRandDeriveChildDiffersFromParent(t *testing.T) {
	parent := NewRand(123)
	child := parent.Derive("perception")
	for i := 0; i < 4; i++ {
		gotParent := parent.Uint64()
		gotChild := child.Uint64()
		if gotParent == gotChild {
			t.Fatalf("child matched parent draw %d: %#016x", i, gotParent)
		}
	}
}
