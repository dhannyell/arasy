package input

// KeyFrame is the coalesced keyboard input for one tick, the counterpart of
// PointerFrame. It exists because a text field cannot be expressed as actions:
// Mapper turns bound keys into what the game means by them, while an editor
// needs the characters themselves and the raw editing keys, whichever bindings
// the game happens to declare.
//
// Text collects every character typed during the tick in order, so a fast
// typist loses nothing when several land in one batch. Pressed holds the down
// edges of the tick and stays free of auto-repeat, matching the button stream
// the Mapper sees; widgets that want a key to repeat while held read Down and
// run their own clock, which repeats identically live and in replay.
type KeyFrame struct {
	Text    []rune
	Pressed []SourceCode
	down    [keyCodeCount]bool
}

// Down reports whether code is held at the end of the tick's input batch.
func (f KeyFrame) Down(code SourceCode) bool {
	if int(code) >= len(f.down) {
		return false
	}
	return f.down[code]
}

// JustPressed reports whether code reported a down edge during this tick.
func (f KeyFrame) JustPressed(code SourceCode) bool {
	for _, pressed := range f.Pressed {
		if pressed == code {
			return true
		}
	}
	return false
}

// KeyboardState is the held-key state that survives across ticks, mirroring
// PointerState. The driver keeps one and stamps it into each KeyFrame.
type KeyboardState struct {
	down [keyCodeCount]bool
}

func (s *KeyboardState) SetDown(code SourceCode, down bool) {
	if int(code) >= len(s.down) {
		return
	}
	s.down[code] = down
}

func (s *KeyboardState) Down(code SourceCode) bool {
	if int(code) >= len(s.down) {
		return false
	}
	return s.down[code]
}

// BeginFrame opens a tick's frame over the keys still held from the last one.
func (s *KeyboardState) BeginFrame() KeyFrame { return KeyFrame{down: s.down} }

func (s *KeyboardState) Clone() KeyboardState { return KeyboardState{down: s.down} }

// AddEvent folds one keyboard event into the frame, returning the frame so it
// can be threaded through a batch. Non-keyboard events are ignored.
func (f KeyFrame) AddEvent(event Event) KeyFrame {
	if event.Source.Device != DeviceKeyboard {
		return f
	}
	switch event.Kind {
	case EventText:
		if r, ok := event.Rune(); ok {
			f.Text = append(f.Text, r)
		}
	case EventButtonDown:
		if int(event.Source.Code) < len(f.down) {
			f.down[event.Source.Code] = true
		}
		f.Pressed = append(f.Pressed, event.Source.Code)
	case EventButtonUp:
		if int(event.Source.Code) < len(f.down) {
			f.down[event.Source.Code] = false
		}
	}
	return f
}
