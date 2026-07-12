package ui

import (
	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Interactor is the interface for Update: deterministic queries that read the
// tick's input.PointerFrame and can mutate ui.State. Implemented by *UI
// (constructed via ui.NewUpdate).
type Interactor interface {
	Clicked(id string, rect math2d.Rect) bool
	ClickedIn(id string, rect math2d.Rect, space Space) bool
	Activated(id string, rect math2d.Rect) bool
	ActivatedIn(id string, rect math2d.Rect, space Space) bool
	Focusable(id string, rect math2d.Rect)
	FocusableIn(id string, rect math2d.Rect, space Space)
	ScrollOffset(id string, view math2d.Rect, contentH float64) float64
	ScrollOffsetIn(id string, view math2d.Rect, contentH float64, space Space) float64
	TableHitIn(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, space Space) int
	ListHitIn(id string, rect math2d.Rect, rowH float64, rowCount int, space Space) int

	Hovered(rect math2d.Rect) bool
	HoveredIn(rect math2d.Rect, space Space) bool

	RowHeight() float64
	HeaderHeight() float64
	TableBodyRect(rect math2d.Rect) math2d.Rect
}

// Drawer emits UI render commands and exposes animation primitives.
type Drawer interface {
	Button(id, label string, rect math2d.Rect)
	Panel(rect math2d.Rect)
	Label(text string, pos math2d.Vec2)
	ProgressBar(rect math2d.Rect, value float64)
	Card(rect math2d.Rect, accent render.Color)
	Badge(text string, center math2d.Vec2, bg render.Color)
	Table(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, cell func(row, col int) string) int

	ButtonIn(id, label string, rect math2d.Rect, space Space)
	PanelIn(rect math2d.Rect, space Space)
	LabelIn(text string, pos math2d.Vec2, space Space)
	ProgressBarIn(rect math2d.Rect, value float64, space Space)
	CardIn(rect math2d.Rect, accent render.Color, space Space)
	BadgeIn(text string, center math2d.Vec2, bg render.Color, space Space)
	ScrollAreaIn(id string, view math2d.Rect, contentH float64, space Space) (offset float64, clip math2d.Rect)
	TableIn(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, cell func(row, col int) string, space Space) int
	TableStyledIn(id string, rect math2d.Rect, cols []TableColumn, rowCount int, selected int, cell func(row, col int) string, style func(row, col int) TableCellStyle, space Space) int
	List(id string, rect math2d.Rect, rowH float64, rowCount, selected int, draw ListDraw) int
	ListIn(id string, rect math2d.Rect, rowH float64, rowCount, selected int, draw ListDraw, space Space) int

	Hovered(rect math2d.Rect) bool
	HoveredIn(rect math2d.Rect, space Space) bool

	RowHeight() float64
	HeaderHeight() float64
	TableBodyRect(rect math2d.Rect) math2d.Rect

	// -- Raw draw primitives — explicit color/size, no theme ---

	FillRect(rect math2d.Rect, fill render.Color)
	FillRectIn(rect math2d.Rect, fill render.Color, space Space)
	DrawText(text string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline)
	DrawTextIn(text string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline, space Space)
	DrawTextRotIn(text string, pos math2d.Vec2, color render.Color, size, rotation float64, align render.TextAlign, baseline render.TextBaseline, space Space)
	SetFont(id render.FontID)
	DrawCircle(center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64)
	DrawCircleIn(center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64, space Space)
	DrawLine(from, to math2d.Vec2, width float64, color render.Color)
	DrawLineIn(from, to math2d.Vec2, width float64, color render.Color, space Space)
	DrawPoly(fill, stroke render.Color, strokeWidth float64, points ...math2d.Vec2)
	DrawPolyIn(fill, stroke render.Color, strokeWidth float64, space Space, points ...math2d.Vec2)

	// -- Animation primitives — state-driven ---

	Approach(id string, target, speed float64) float64
	Spring(id string, target float64, cfg SpringConfig) float64
	SpringScale(id string, target float64, cfg SpringConfig, rect math2d.Rect) math2d.Rect

	// Animation primitives — time-driven

	Tween(id string, dur float64, k ease.Kind) float64
	Clock(id string) float64
	Reset(id string)

	// Sequence — multistep scripted animation

	SequenceKeyed(id string, key uint64) Sequence

	// Mid/high-level animation helpers

	Effect(id string, opts EffectOptions) Effect
	FX() FX

	// Command groups — render-only container transforms

	Emit(cmds ...render.Command)
	BeginGroup(opts GroupOptions)
	EndGroup()

	// Screen transitions — keyed screen-change animation

	ScreenTransition(id string, key uint64, opts TransitionOptions) ScreenTransition
}
