package ui

import "github.com/dhannyell/frameforge/math2d"

// Interactor is the interface for Update: deterministic queries that read the
// tick's PointerFrame and can mutate State.activeID. Implemented by *UI
// (constructed via NewUpdate).
type Interactor interface {
	Clicked(id string, rect math2d.Rect) bool
}

// Drawer is the rendering interface: pure functions of World + cursor position that
// accumulate render.Command. Implemented by *UI (constructed via New).
type Drawer interface {
	Button(id, label string, rect math2d.Rect)
	Panel(rect math2d.Rect)
	Label(text string, pos math2d.Vec2)
	ProgressBar(rect math2d.Rect, value float64)
}
