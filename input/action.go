package input

type Action string

const (
	ActionUnknown Action = ""
	ActionMoveX   Action = "move_x"
	ActionMoveY   Action = "move_y"
	ActionConfirm Action = "confirm"
	ActionCancel  Action = "cancel"
	ActionPause   Action = "pause"
)

func (a Action) Valid() bool {
	switch a {
	case ActionMoveX, ActionMoveY, ActionConfirm, ActionCancel, ActionPause:
		return true
	default:
		return false
	}
}
