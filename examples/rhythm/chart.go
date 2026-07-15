package rhythm

type objectKind uint8

const (
	objectTap objectKind = iota
	objectHold
	objectSpecial
)

type chartObject struct {
	ID     int
	Kind   objectKind
	TimeMS int
	EndMS  int
	Lane   int
}
