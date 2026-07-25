package simfuzz

import (
	"testing"
)

func TestDecodeScriptIsTotal(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"single byte", []byte{42}},
		{"two bytes", []byte{0, 255}},
		{"three bytes", []byte{0, 1, 10}},
		{"four bytes header only", []byte{0, 0, 0, 30}},
		{"five bytes partial mask", []byte{0, 0, 1, 10, 0x01}},
		{"all 0xFF", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := DecodeScript(c.data) // must not panic
			if s.MaxTicks < 1 {
				t.Fatalf("MaxTicks = %d, want >= 1", s.MaxTicks)
			}
			if len(s.Entities) > MaxEntities {
				t.Fatalf("entity count %d exceeds MaxEntities", len(s.Entities))
			}
		})
	}
}

func TestDecodeScriptIsDeterministic(t *testing.T) {
	data := seedBytes(20, CompTransform|CompDamping, CompTransform|CompVelocity)
	s1 := DecodeScript(data)
	s2 := DecodeScript(data)
	if len(s1.Entities) != len(s2.Entities) {
		t.Fatalf("entity count differs: %d vs %d", len(s1.Entities), len(s2.Entities))
	}
	for i := range s1.Entities {
		if s1.Entities[i].ComponentMask != s2.Entities[i].ComponentMask {
			t.Fatalf("entity %d mask differs", i)
		}
	}
	if s1.MaxTicks != s2.MaxTicks || s1.Seed != s2.Seed {
		t.Fatalf("header differs: %+v vs %+v", s1, s2)
	}
}

func TestDecodeScriptClampsLargeMasks(t *testing.T) {
	s := DecodeScript([]byte{0, 0, 255, 255})
	if len(s.Entities) > MaxEntities {
		t.Fatalf("entity count %d exceeds MaxEntities", len(s.Entities))
	}
	if s.MaxTicks > MaxTicks {
		t.Fatalf("MaxTicks %d exceeds limit", s.MaxTicks)
	}
}

func TestDecodeScriptZeroTickDefaultsToOne(t *testing.T) {
	s := DecodeScript([]byte{0, 0, 1, 0})
	if s.MaxTicks < 1 {
		t.Fatalf("MaxTicks = %d, want >= 1", s.MaxTicks)
	}
}

func TestDecodeScriptVersionOneReachesTickLimit(t *testing.T) {
	s := DecodeScript(seedBytes(MaxTicks))
	if s.MaxTicks != MaxTicks {
		t.Fatalf("MaxTicks = %d, want %d", s.MaxTicks, MaxTicks)
	}
}

func TestDecodeScriptTruncatedEntityData(t *testing.T) {
	s := DecodeScript([]byte{0, 0, 2, 10, 0xFF, 0xFF, 0xFF, 0xFF, 0x01})
	_ = s // must not panic
}

func TestDecodeEventsMonotonicAndBounded(t *testing.T) {
	data := seedBytes(60, 0)                               // v1 header + 4-byte zero mask
	data = append(data, 3)                                 // event count
	data = append(data, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)   // tick 0
	data = append(data, 1, 1, 5, 0, 0, 0, 0, 0, 0, 0, 0)   // tick 5
	data = append(data, 2, 0, 200, 0, 0, 0, 0, 0, 0, 0, 0) // clamp to 59
	s := DecodeScript(data)
	if len(s.Events) != 3 {
		t.Fatalf("events = %d, want 3", len(s.Events))
	}
	wantTicks := []uint64{0, 5, 59}
	for i, w := range wantTicks {
		if s.Events[i].Tick != w {
			t.Fatalf("event %d tick = %d, want %d", i, s.Events[i].Tick, w)
		}
		if !s.Events[i].Valid() {
			t.Fatalf("event %d not valid: %+v", i, s.Events[i])
		}
	}
}
