package scene

import "slices"

const maxStoreIndex EntityID = EntityID(1<<31 - 1)

// noIndex marks a EntityID with no entry in a store's dense array.
const noIndex int32 = -1

// store is a sparse set keeping one component type in dense, cache-friendly
// storage. The dense arrays are always sorted by EntityID, which makes
// iteration deterministic without sorting and keeps the checksum byte
// stream identical for worlds holding identical state.
//
// Concurrency: clone does not copy sparse, so the first get/has/set/remove on
// a cloned store rebuilds it lazily, mutating the receiver. That makes even
// get and has writes, not pure reads. Callers must not access a freshly cloned
// store from multiple goroutines until it has been touched once on a single
// goroutine (or the access must be externally synchronized). Iterating ids and
// dense directly and Checksum never touch sparse, so those stay read-only.
type store[T any] struct {
	dense  []T
	ids    []EntityID
	sparse []int32
}

func (s *store[T]) get(id EntityID) (T, bool) {
	idx := int(id)
	if idx < len(s.sparse) {
		if sparseIdx := s.sparse[idx]; sparseIdx != noIndex {
			return s.dense[int(sparseIdx)], true
		}
	} else if len(s.sparse) == 0 && len(s.ids) > 0 {
		s.rebuildSparse()
		if idx < len(s.sparse) {
			if sparseIdx := s.sparse[idx]; sparseIdx != noIndex {
				return s.dense[int(sparseIdx)], true
			}
		}
	}

	var zero T
	return zero, false
}

func (s *store[T]) set(id EntityID, value T) {
	s.growSparse(id)
	sparseIdx := int(id)
	if idx := s.sparse[sparseIdx]; idx != noIndex {
		s.dense[int(idx)] = value
		return
	}
	// Common case: ids are monotonically increasing, so new components
	// append in sorted position. Late sets on old ids insert with a shift.
	pos := len(s.ids)
	if pos > 0 && s.ids[pos-1] > id {
		pos, _ = slices.BinarySearch(s.ids, id)
	}
	s.dense = slices.Insert(s.dense, pos, value)
	s.ids = slices.Insert(s.ids, pos, id)
	for i := pos; i < len(s.ids); i++ {
		s.sparse[int(s.ids[i])] = int32(i)
	}
}

func (s *store[T]) remove(id EntityID) {
	if len(s.sparse) == 0 && len(s.ids) > 0 {
		s.rebuildSparse()
	}
	sparseIdx := int(id)
	if sparseIdx >= len(s.sparse) || s.sparse[sparseIdx] == noIndex {
		return
	}
	pos := s.sparse[sparseIdx]
	s.dense = slices.Delete(s.dense, int(pos), int(pos+1))
	s.ids = slices.Delete(s.ids, int(pos), int(pos+1))
	s.sparse[sparseIdx] = noIndex
	for i := int(pos); i < len(s.ids); i++ {
		s.sparse[int(s.ids[i])] = int32(i)
	}
}

func (s *store[T]) has(id EntityID) bool {
	idx := int(id)
	if len(s.sparse) == 0 && len(s.ids) > 0 {
		s.rebuildSparse()
	}
	return idx < len(s.sparse) && s.sparse[idx] != noIndex
}

func (s *store[T]) len() int { return len(s.ids) }

func (s *store[T]) clone() store[T] {
	return store[T]{
		dense: slices.Clone(s.dense),
		ids:   slices.Clone(s.ids),
	}
}

func (s *store[T]) growSparse(id EntityID) {
	if id > maxStoreIndex {
		panic("scene store EntityID exceeds int32 sparse index capacity")
	}
	if len(s.sparse) == 0 && len(s.ids) > 0 {
		s.rebuildSparse()
	}
	for int(id) >= len(s.sparse) {
		s.sparse = append(s.sparse, noIndex)
	}
}

func (s *store[T]) rebuildSparse() {
	maxID := s.ids[len(s.ids)-1]
	s.sparse = make([]int32, int(maxID)+1)
	for i := range s.sparse {
		s.sparse[i] = noIndex
	}
	for i, id := range s.ids {
		s.sparse[int(id)] = int32(i)
	}
}
