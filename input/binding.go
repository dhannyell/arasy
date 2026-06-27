package input

type Binding struct {
	Source Source
	Action Action
	Scale  float64
}

func (b Binding) Valid() bool {
	return b.Source.Valid() && b.Action.Valid()
}
