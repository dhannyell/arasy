package scene

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// componentSetters returns every exported *World method matching the
// component-setter convention: SetX(EntityID, T) with no results. Sorted by
// name so population order is deterministic (sprites/transforms sort before
// the tweens they would cancel, e.g. SetCircleSprite before
// SetCircleSpriteTween, SetTransform before SetTransformTween).
func componentSetters(t *testing.T) []reflect.Method {
	t.Helper()
	wt := reflect.TypeFor[*World]()
	var setters []reflect.Method
	for i := 0; i < wt.NumMethod(); i++ {
		m := wt.Method(i)
		if !strings.HasPrefix(m.Name, "Set") {
			continue
		}
		mt := m.Type // func(*World, EntityID, T)
		if mt.NumIn() != 3 || mt.NumOut() != 0 || mt.In(1) != reflect.TypeFor[EntityID]() {
			continue
		}
		setters = append(setters, m)
	}
	sort.Slice(setters, func(i, j int) bool { return setters[i].Name < setters[j].Name })
	return setters
}

// fillNonZero writes deterministic non-zero values into every field of v
// (recursing into structs, including embedded ones), so each population
// seed produces distinct component payloads. Floats get the running seed
// directly; ints/uints get a small positive value derived from it (kept
// within [1,100] so it never overflows a narrow named type like
// ease.Kind/LoopMode, both uint8); bools are set true.
func fillNonZero(v reflect.Value, seed float64) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fillNonZero(v.Field(i), seed+float64(i)*1.7+1)
		}
	case reflect.Float64, reflect.Float32:
		v.SetFloat(seed)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(smallPositive(seed))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(smallPositive(seed)))
	}
}

func smallPositive(seed float64) int64 {
	n := int64(seed * 3)
	if n < 0 {
		n = -n
	}
	return n%100 + 1
}

// populateAll sets every discovered component on id with values derived from
// seed, then asserts via the paired getter X(EntityID) (T, bool) that every
// component is actually present — this catches setter side effects (e.g.
// SetTransform cancels the transform tween) instead of silently testing
// less. It returns the getter-observed values keyed by component name.
func populateAll(t *testing.T, w *World, id EntityID, seed float64) map[string]any {
	t.Helper()
	wv := reflect.ValueOf(w)
	result := make(map[string]any)

	for i, setter := range componentSetters(t) {
		compType := setter.Type.In(2)
		val := reflect.New(compType).Elem()
		fillNonZero(val, seed+float64(i)*7+1)

		wv.MethodByName(setter.Name).Call([]reflect.Value{reflect.ValueOf(id), val})

		name := strings.TrimPrefix(setter.Name, "Set")
		getter := wv.MethodByName(name)
		if !getter.IsValid() {
			t.Fatalf("component %s: no paired getter %s(EntityID) (T, bool)", name, name)
		}
		out := getter.Call([]reflect.Value{reflect.ValueOf(id)})
		if len(out) != 2 || !out[1].Bool() {
			t.Fatalf("component %s: getter reports ok=false right after Set%s", name, name)
		}
		result[name] = out[0].Interface()
	}
	return result
}

// TestComponentSetterConventionCoversAllStores closes the triangle between
// component stores, ChecksumComponentStores and the Set convention: a store
// added without a Set/getter pair in the convention, or a setter that
// doesn't follow it, changes this count without anyone noticing otherwise.
// renderOnlySetters follow the Set convention but write to presentation
// caches excluded from Checksum — mirroring the store exclusion in
// TestChecksumCoversAllComponentStores — so they must not count against
// ChecksumComponentStores.
var renderOnlySetters = map[string]bool{
	"SetSpriteTransform": true,
	"SetCommandSprite":   true,
	"SetAttachment":      true,
}

func TestComponentSetterConventionCoversAllStores(t *testing.T) {
	got := 0
	for _, setter := range componentSetters(t) {
		if renderOnlySetters[setter.Name] {
			continue
		}
		got++
	}
	if got != ChecksumComponentStores {
		t.Fatalf("component setters discovered by convention = %d, want %d (ChecksumComponentStores)",
			got, ChecksumComponentStores)
	}
}

// TestCloneCoversAllComponentStores catches a component store forgotten in
// World.Clone(): either missing entirely (the clone reads back the zero
// value) or shallow-copied (the clone shares storage with the source and
// observes the source's later mutations).
func TestCloneCoversAllComponentStores(t *testing.T) {
	w := NewWorld()
	id := w.Spawn()
	before := populateAll(t, w, id, 1.5)
	wantRenderTransform := w.renderTransforms.dense[0]

	snap := w.Clone()
	snapValue := reflect.ValueOf(snap)

	for name, want := range before {
		getter := snapValue.MethodByName(name)
		out := getter.Call([]reflect.Value{reflect.ValueOf(id)})
		if !out[1].Bool() {
			t.Fatalf("component %s: missing from Clone (store forgotten in World.Clone)", name)
		}
		if got := out[0].Interface(); !reflect.DeepEqual(got, want) {
			t.Fatalf("component %s: value diverged right after Clone:\n got:  %+v\n want: %+v", name, got, want)
		}
	}
	if len(snap.renderTransforms.ids) != 1 || snap.renderTransforms.dense[0] != wantRenderTransform {
		t.Fatal("renderTransforms: not copied by Clone (no getter — checked via the internal store)")
	}

	// Isolation: mutate the source in place (same id => store.set overwrites
	// the dense slot instead of appending). A shallow-copying Clone (shared
	// backing slice) would observe this mutation on the snapshot too.
	populateAll(t, w, id, 99.5)

	for name, want := range before {
		getter := snapValue.MethodByName(name)
		out := getter.Call([]reflect.Value{reflect.ValueOf(id)})
		if got := out[0].Interface(); !reflect.DeepEqual(got, want) {
			t.Fatalf("component %s: Clone shares storage with the source (shallow copy) — snapshot changed after mutating the source", name)
		}
	}
	if snap.renderTransforms.dense[0] != wantRenderTransform {
		t.Fatal("renderTransforms: Clone shares storage with the source (shallow copy)")
	}

	if w.Checksum() == snap.Checksum() {
		t.Fatal("sanity: source and snapshot checksums match after mutating only the source")
	}
}

// TestDespawnCoversAllComponentStores catches a component store forgotten in
// World.Despawn(): the leaked component keeps a getter returning ok=true, or
// leaks into the checksum, after the entity was despawned. Twin worlds
// (matching EntityID from a fresh Spawn) isolate the "all zero" reference
// from the "everything set, then despawned" world.
func TestDespawnCoversAllComponentStores(t *testing.T) {
	w1 := NewWorld()
	w2 := NewWorld()
	id1 := w1.Spawn()
	id2 := w2.Spawn()

	populateAll(t, w2, id2, 1.5)
	// w1 stays with only the spawn: no components ever set on id1.

	w1.Despawn(id1)
	w2.Despawn(id2)

	w2Value := reflect.ValueOf(w2)
	for _, setter := range componentSetters(t) {
		name := strings.TrimPrefix(setter.Name, "Set")
		getter := w2Value.MethodByName(name)
		out := getter.Call([]reflect.Value{reflect.ValueOf(id2)})
		if out[1].Bool() {
			t.Fatalf("component %s: still present after Despawn (store forgotten in World.Despawn)", name)
		}
	}
	if len(w2.renderTransforms.ids) != 0 {
		t.Fatal("renderTransforms: not cleared by Despawn (no getter — checked via the internal store)")
	}

	if got, want := w2.Checksum(), w1.Checksum(); got != want {
		t.Fatalf("checksum mismatch after despawning twin worlds: despawned=%d, never-populated=%d — a component leaked into the checksum", got, want)
	}
}

// TestDespawnBatchCoversAllComponentStores applies the same structural guard
// to the compaction path. A new store must be removed by both APIs.
func TestDespawnBatchCoversAllComponentStores(t *testing.T) {
	w1 := NewWorld()
	w2 := NewWorld()
	id1 := w1.Spawn()
	id2 := w2.Spawn()

	populateAll(t, w2, id2, 1.5)
	w1.DespawnBatch([]EntityID{id1})
	w2.DespawnBatch([]EntityID{id2})

	w2Value := reflect.ValueOf(w2)
	for _, setter := range componentSetters(t) {
		name := strings.TrimPrefix(setter.Name, "Set")
		getter := w2Value.MethodByName(name)
		out := getter.Call([]reflect.Value{reflect.ValueOf(id2)})
		if out[1].Bool() {
			t.Fatalf("component %s: still present after DespawnBatch (store forgotten in World.DespawnBatch)", name)
		}
	}
	if len(w2.renderTransforms.ids) != 0 {
		t.Fatal("renderTransforms: not cleared by DespawnBatch (no getter — checked via the internal store)")
	}
	if got, want := w2.Checksum(), w1.Checksum(); got != want {
		t.Fatalf("checksum mismatch after batch despawning twin worlds: despawned=%d, never-populated=%d — a component leaked into the checksum", got, want)
	}
}
