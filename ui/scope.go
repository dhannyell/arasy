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

func (s InteractScope) TableHit(id string, rect math2d.Rect, cols []TableColumn, rowCount, selected int) int {
	return s.u.tableHitIn(id, rect, cols, rowCount, selected, s.space)
}

func (s InteractScope) ListHit(id string, rect math2d.Rect, rowH float64, rowCount int) int {
	return s.u.listHitIn(id, rect, rowH, rowCount, s.space)
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
