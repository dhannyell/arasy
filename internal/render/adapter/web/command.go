package web

import (
	"sort"

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
}

func NewCommandDTOs(commands []render.Command) []CommandDTO {
	commands = append([]render.Command(nil), commands...)
	sort.SliceStable(commands, func(i, j int) bool {
		return commands[i].Layer() < commands[j].Layer()
	})

	commandDTOs := make([]CommandDTO, 0, len(commands))

	for _, command := range commands {
		var commandDTO CommandDTO

		switch command := command.(type) {
		case render.DrawRect:
			commandDTO = CommandDTO{
				Type: "rect",

				Rect:         new(NewRectDTO(command.Rect)),
				Fill:         new(NewColorDTO(command.Fill)),
				Stroke:       new(NewColorDTO(command.Stroke)),
				StrokeWidth:  command.StrokeWidth,
				CornerRadius: command.CornerRadius,
				Layer:        command.Layer(),
				Clip:         newOptionalRectDTO(command.Clip),
			}
		case render.DrawLine:
			commandDTO = CommandDTO{
				Type: "line",

				From:  new(NewVec2DTO(command.From)),
				To:    new(NewVec2DTO(command.To)),
				Width: command.Width,
				Color: new(NewColorDTO(command.Color)),
				Layer: command.Layer(),
				Clip:  newOptionalRectDTO(command.Clip),
			}
		case render.DrawCircle:
			commandDTO = CommandDTO{
				Type: "circle",

				Center:      new(NewVec2DTO(command.Center)),
				Radius:      command.Radius,
				Fill:        new(NewColorDTO(command.Fill)),
				Stroke:      new(NewColorDTO(command.Stroke)),
				StrokeWidth: command.StrokeWidth,
				Layer:       command.Layer(),
				Clip:        newOptionalRectDTO(command.Clip),
			}
		case render.DrawPoint:
			commandDTO = CommandDTO{
				Type:   "point",
				Center: new(NewVec2DTO(command.Center)),
				Radius: command.Radius,
				Color:  new(NewColorDTO(command.Color)),
				Layer:  command.Layer(),
				Clip:   newOptionalRectDTO(command.Clip),
			}
		case render.DrawArc:
			commandDTO = CommandDTO{
				Type:       "arc",
				Center:     new(NewVec2DTO(command.Center)),
				Radius:     command.Radius,
				StartAngle: command.StartAngle,
				EndAngle:   command.EndAngle,
				Width:      command.Width,
				Color:      new(NewColorDTO(command.Color)),
				Layer:      command.Layer(),
				Clip:       newOptionalRectDTO(command.Clip),
			}
		case render.DrawText:
			commandDTO = CommandDTO{
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
			}
		case render.DrawPolygon:
			commandDTO = CommandDTO{
				Type: "polygon",

				Points:      NewVec2SliceDTO(command.Points),
				Fill:        new(NewColorDTO(command.Fill)),
				Stroke:      new(NewColorDTO(command.Stroke)),
				StrokeWidth: command.StrokeWidth,
				Layer:       command.Layer(),
				Clip:        newOptionalRectDTO(command.Clip),
			}
		default:
			continue
		}

		commandDTOs = append(commandDTOs, commandDTO)
	}

	return commandDTOs
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
