package input

type Action uint8

const (
	ActionUnknown Action = iota
	ActionMoveX
	ActionMoveY
	ActionConfirm
	ActionCancel
	ActionPause
	ActionJump
)

func (a Action) Valid() bool {
	return a != ActionUnknown
}
