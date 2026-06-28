package scene

type DebugLayers uint32

const (
	DebugColliders DebugLayers = 1 << iota
	DebugContacts
	DebugGrid
	DebugActiveGrid
	DebugCandidateGrid
)

func (l DebugLayers) Has(flag DebugLayers) bool {
	return l&flag != 0
}
