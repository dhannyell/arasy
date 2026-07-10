package dto

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

type CommandDTO struct {
	Type string `json:"type"`

	Rect     *RectDTO `json:"rect,omitempty"`
	Center   *Vec2DTO `json:"center,omitempty"`
	Position *Vec2DTO `json:"position,omitempty"`
	From     *Vec2DTO `json:"from,omitempty"`
	To       *Vec2DTO `json:"to,omitempty"`

	Radius       float64 `json:"radius,omitempty"`
	StartAngle   float64 `json:"startAngle,omitempty"`
	EndAngle     float64 `json:"endAngle,omitempty"`
	Width        float64 `json:"width,omitempty"`
	StrokeWidth  float64 `json:"strokeWidth,omitempty"`
	CornerRadius float64 `json:"cornerRadius,omitempty"`

	Fill   *ColorDTO `json:"fill,omitempty"`
	Stroke *ColorDTO `json:"stroke,omitempty"`
	Color  *ColorDTO `json:"color,omitempty"`

	Text     string     `json:"text,omitempty"`
	Size     float64    `json:"size,omitempty"`
	Align    string     `json:"align,omitempty"`
	Baseline string     `json:"baseline,omitempty"`
	Rotation float64    `json:"rotation,omitempty"`
	Layer    int        `json:"layer"`
	Points   []*Vec2DTO `json:"points,omitempty"`
	Clip     *RectDTO   `json:"clip,omitempty"`

	Texture uint32         `json:"texture,omitempty"`
	Src     *RectDTO       `json:"src,omitempty"`
	Tint    *ColorDTO      `json:"tint,omitempty"`
	Corners *[4][2]float64 `json:"corners,omitempty"`
}

func NewCommandDTOs(commands []render.Command) []CommandDTO {
	sink := &commandDTOSink{commandDTOs: make([]CommandDTO, 0, len(commands))}
	render.Each(commands, sink)
	return sink.commandDTOs
}

type commandDTOSink struct {
	commandDTOs []CommandDTO
}

func (s *commandDTOSink) VisitRect(command render.DrawRect) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type: "rect",

		Rect:         new(NewRectDTO(command.Rect)),
		Fill:         new(NewColorDTO(command.Fill)),
		Stroke:       new(NewColorDTO(command.Stroke)),
		StrokeWidth:  command.StrokeWidth,
		CornerRadius: command.CornerRadius,
		Layer:        command.Layer(),
		Clip:         newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitLine(command render.DrawLine) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type: "line",

		From:  new(NewVec2DTO(command.From)),
		To:    new(NewVec2DTO(command.To)),
		Width: command.Width,
		Color: new(NewColorDTO(command.Color)),
		Layer: command.Layer(),
		Clip:  newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitCircle(command render.DrawCircle) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type: "circle",

		Center:      new(NewVec2DTO(command.Center)),
		Radius:      command.Radius,
		Fill:        new(NewColorDTO(command.Fill)),
		Stroke:      new(NewColorDTO(command.Stroke)),
		StrokeWidth: command.StrokeWidth,
		Layer:       command.Layer(),
		Clip:        newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitPoint(command render.DrawPoint) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type:   "point",
		Center: new(NewVec2DTO(command.Center)),
		Radius: command.Radius,
		Color:  new(NewColorDTO(command.Color)),
		Layer:  command.Layer(),
		Clip:   newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitArc(command render.DrawArc) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type:       "arc",
		Center:     new(NewVec2DTO(command.Center)),
		Radius:     command.Radius,
		StartAngle: command.StartAngle,
		EndAngle:   command.EndAngle,
		Width:      command.Width,
		Color:      new(NewColorDTO(command.Color)),
		Layer:      command.Layer(),
		Clip:       newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitText(command render.DrawText) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type:     "text",
		Position: new(NewVec2DTO(command.Position)),
		Text:     command.Text,
		Color:    new(NewColorDTO(command.Color)),
		Size:     command.Size,
		Align:    textAlignToString(command.Align),
		Baseline: textBaselineToString(command.Baseline),
		Rotation: command.Rotation,
		Layer:    command.Layer(),
		Clip:     newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitPolygon(command render.DrawPolygon) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type: "polygon",

		Points:      NewVec2SliceDTO(command.Points),
		Fill:        new(NewColorDTO(command.Fill)),
		Stroke:      new(NewColorDTO(command.Stroke)),
		StrokeWidth: command.StrokeWidth,
		Layer:       command.Layer(),
		Clip:        newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitImage(command render.DrawImage) {
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type:     "image",
		Texture:  uint32(command.Texture),
		Rect:     new(NewRectDTO(command.Dest)),
		Src:      newOptionalRectDTO(command.Src),
		Tint:     newOptionalColorDTO(command.Tint),
		Rotation: command.Rotation,
		Layer:    command.Layer(),
		Clip:     newOptionalRectDTO(command.Clip),
	})
}

func (s *commandDTOSink) VisitImageQuad(command render.DrawImageQuad) {
	corners := newCornersDTO(command.Corners)
	s.commandDTOs = append(s.commandDTOs, CommandDTO{
		Type:    "imagequad",
		Texture: uint32(command.Texture),
		Corners: &corners,
		Src:     newOptionalRectDTO(command.Src),
		Tint:    newOptionalColorDTO(command.Tint),
		Layer:   command.Layer(),
		Clip:    newOptionalRectDTO(command.Clip),
	})
}

// newCornersDTO flattens the quad's four corners (source top-left,
// top-right, bottom-right, bottom-left) into plain [x, y] pairs for JSON.
func newCornersDTO(corners [4]math2d.Vec2) [4][2]float64 {
	var out [4][2]float64
	for i, c := range corners {
		out[i] = [2]float64{c.X, c.Y}
	}
	return out
}

func textAlignToString(a render.TextAlign) string {
	switch a {
	case render.AlignCenter:
		return "center"
	case render.AlignRight:
		return "right"
	default:
		return "left"
	}
}

func textBaselineToString(b render.TextBaseline) string {
	switch b {
	case render.BaselineTop:
		return "top"
	case render.BaselineMiddle:
		return "middle"
	case render.BaselineBottom:
		return "bottom"
	default:
		return "alphabetic"
	}
}
