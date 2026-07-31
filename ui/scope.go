package ui

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// InteractScope exposes the deterministic interaction queries of one
// coordinate space. Obtained via Interactor.In; valid for the current tick.
type InteractScope struct {
	u     *UI
	space Space
}

func (s InteractScope) Clicked(id string, rect math2d.Rect) bool {
	return s.u.clickedIn(id, rect, s.space)
}

func (s InteractScope) Activated(id string, rect math2d.Rect) bool {
	return s.u.activatedIn(id, rect, s.space)
}

func (s InteractScope) Focusable(id string, rect math2d.Rect) {
	s.u.focusableIn(id, rect, s.space)
}

func (s InteractScope) ScrollOffset(id string, view math2d.Rect, contentH float64) float64 {
	return s.u.scrollOffsetIn(id, view, contentH, s.space)
}

func (s InteractScope) SliderInput(id string, rect math2d.Rect, value float64, opts SliderOptions) (next float64, changed bool) {
	return s.u.sliderInputIn(id, rect, value, opts, s.space)
}

// TextInput edits the text the caller holds: pass the value you own, store the
// one that comes back. Typed text is game state — it filters game data, names
// game things — so it rewinds with the simulation instead of hiding in the
// widget. The caret and the key-repeat clock stay in ui.State, where nobody
// has to declare them.
func (s InteractScope) TextInput(id string, rect math2d.Rect, text string, opts TextInputOptions) TextInputResult {
	return s.u.textInputIn(id, rect, text, opts, s.space)
}

func (s InteractScope) TableHit(id string, rect math2d.Rect, cols []TableColumn, rowCount, selected int) int {
	return s.u.tableHitIn(id, rect, cols, rowCount, selected, s.space)
}

func (s InteractScope) ListHit(id string, rect math2d.Rect, rowH float64, rowCount int) int {
	return s.u.listHitIn(id, rect, rowH, rowCount, s.space)
}

// TabsInputValue selects the active tab from this tick's pointer events,
// leaving the index itself with the caller: pass the one you hold, store the
// one returned. Reach for this whenever the selection is state your game
// means something by — a screen the player navigates to belongs in the game's
// own state, where it rewinds with the simulation and can also be set by
// paths the UI never sees, like a keyboard shortcut.
//
// headers carries one rect per tab, so widths, gaps and alignment stay the
// caller's to decide.
func (s InteractScope) TabsInputValue(id string, headers []math2d.Rect, current int) (next int, changed bool) {
	return s.u.tabsInputValueIn(id, headers, current, s.space)
}

// TabsInput is the widget-owned variant: it keeps the active tab in ui.State
// keyed by id, for selections nobody would think to declare or to save, such
// as a tooling overlay's open tab. DrawScope.Tabs only reads what it stored.
func (s InteractScope) TabsInput(id string, rect math2d.Rect, count int) (active int, changed bool) {
	return s.u.tabsInputIn(id, rect, count, s.space)
}

func (s InteractScope) Hovered(rect math2d.Rect) bool {
	return s.u.hoveredIn(rect, s.space)
}

// DrawScope emits widgets and raw primitives into one coordinate space.
// Obtained via Drawer.In; valid for the current frame.
type DrawScope struct {
	u     *UI
	space Space
}

func (s DrawScope) Button(id, label string, rect math2d.Rect) {
	s.u.buttonIn(id, label, rect, s.space)
}

func (s DrawScope) Panel(rect math2d.Rect) {
	s.u.panelIn(rect, s.space)
}

// TitledPanel draws a panel with a title bar. Static container, no state.
func (s DrawScope) TitledPanel(rect math2d.Rect, title string) {
	s.u.titledPanelIn(rect, title, s.space)
}

// TabsAt draws one themed header per rect, highlighting active. It is the
// draw half of TabsInputValue: the caller owns both the index and the layout,
// so no band is drawn behind the headers — that background is the caller's,
// which is what lets the headers be spaced, staggered or styled freely.
func (s DrawScope) TabsAt(headers []math2d.Rect, labels []string, active int) {
	s.u.tabsAtIn(headers, labels, active, s.space)
}

// Tabs draws the band plus an equal-width header row and returns the active
// tab index. It is the draw half of TabsInput, and reads the index that one
// stored; drawing never changes the active tab.
func (s DrawScope) Tabs(id string, rect math2d.Rect, labels []string) int {
	return s.u.tabsIn(id, rect, labels, s.space)
}

// TextInput draws a text field with the caller's text, and the caret while it
// holds focus. Call InteractScope.TextInput with the same id and rect during
// Update to make it editable.
func (s DrawScope) TextInput(id string, rect math2d.Rect, text string, opts TextInputOptions) {
	s.u.textInputDrawIn(id, rect, text, opts, s.space)
}

func (s DrawScope) Label(text string, pos math2d.Vec2) {
	s.u.labelIn(text, pos, s.space)
}

func (s DrawScope) ProgressBar(rect math2d.Rect, value float64) {
	s.u.progressBarIn(rect, value, s.space)
}

func (s DrawScope) Slider(id string, rect math2d.Rect, value float64, opts SliderOptions) {
	s.u.sliderIn(id, rect, value, opts, s.space)
}

func (s DrawScope) Card(rect math2d.Rect, accent render.Color) {
	s.u.cardIn(rect, accent, s.space)
}

func (s DrawScope) Badge(text string, center math2d.Vec2, bg render.Color) {
	s.u.badgeIn(text, center, bg, s.space)
}

func (s DrawScope) ScrollArea(id string, view math2d.Rect, contentH float64) (offset float64, clip math2d.Rect) {
	return s.u.scrollAreaIn(id, view, contentH, s.space)
}

func (s DrawScope) Table(id string, rect math2d.Rect, cols []TableColumn, rowCount, selected int, cell func(row, col int) string) int {
	return s.u.tableIn(id, rect, cols, rowCount, selected, cell, s.space)
}

func (s DrawScope) TableStyled(id string, rect math2d.Rect, cols []TableColumn, rowCount, selected int, cell func(row, col int) string, style func(row, col int) TableCellStyle) int {
	return s.u.tableStyledIn(id, rect, cols, rowCount, selected, cell, style, s.space)
}

func (s DrawScope) List(id string, rect math2d.Rect, rowH float64, rowCount, selected int, draw ListDraw) int {
	return s.u.listIn(id, rect, rowH, rowCount, selected, draw, s.space)
}

func (s DrawScope) Hovered(rect math2d.Rect) bool {
	return s.u.hoveredIn(rect, s.space)
}

func (s DrawScope) FillRect(rect math2d.Rect, fill render.Color) {
	s.u.fillRectIn(rect, fill, s.space)
}

func (s DrawScope) DrawText(text string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline) {
	s.u.drawTextIn(text, pos, color, size, align, baseline, s.space)
}

func (s DrawScope) DrawTextRot(text string, pos math2d.Vec2, color render.Color, size, rotation float64, align render.TextAlign, baseline render.TextBaseline) {
	s.u.drawTextRotIn(text, pos, color, size, rotation, align, baseline, s.space)
}

func (s DrawScope) DrawCircle(center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64) {
	s.u.drawCircleIn(center, radius, fill, stroke, strokeWidth, s.space)
}

func (s DrawScope) DrawLine(from, to math2d.Vec2, width float64, color render.Color) {
	s.u.drawLineIn(from, to, width, color, s.space)
}

func (s DrawScope) DrawPoly(fill, stroke render.Color, strokeWidth float64, points ...math2d.Vec2) {
	s.u.drawPolyIn(fill, stroke, strokeWidth, s.space, points...)
}

func (s DrawScope) FX() FX {
	return FX{ui: s.u, space: s.space}
}
