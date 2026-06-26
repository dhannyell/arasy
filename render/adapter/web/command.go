package web

import (
	"github.com/dhannyell/frameforge/render"
)

type CommandDTO struct {
	Type string `json:"type"`

	Rect   RectDTO `json:"rect"`
	Center Vec2DTO `json:"center"`
	From   Vec2DTO `json:"from"`
	To     Vec2DTO `json:"to"`

	Radius      float64 `json:"radius,omitempty"`
	StartAngle  float64 `json:"startAngle,omitempty"`
	EndAngle    float64 `json:"endAngle,omitempty"`
	Width       float64 `json:"width,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`

	Fill   ColorDTO `json:"fill"`
	Stroke ColorDTO `json:"stroke"`
	Color  ColorDTO `json:"color"`

	Text  string  `json:"text,omitempty"`
	Size  float64 `json:"size,omitempty"`
	Layer int     `json:"layer"`
}

func NewCommandDTOs(commands []render.Command) []CommandDTO {
	commandDTOs := make([]CommandDTO, 0, len(commands))

	for _, command := range commands {
		var commandDTO CommandDTO

		switch command := command.(type) {
		case render.DrawRect:
			commandDTO = CommandDTO{
				Type: "rect",

				Rect:        NewRectDTO(command.Rect),
				Fill:        NewColorDTO(command.Fill),
				Stroke:      NewColorDTO(command.Stroke),
				StrokeWidth: command.StrokeWidth,
				Layer:       command.Layer,
			}
		case render.DrawLine:
			commandDTO = CommandDTO{
				Type: "line",

				From:  NewVec2DTO(command.From),
				To:    NewVec2DTO(command.To),
				Width: command.Width,
				Color: NewColorDTO(command.Color),
				Layer: command.Layer,
			}
		case render.DrawCircle:
			commandDTO = CommandDTO{
				Type: "circle",

				Center:      NewVec2DTO(command.Center),
				Radius:      command.Radius,
				Fill:        NewColorDTO(command.Fill),
				Stroke:      NewColorDTO(command.Stroke),
				StrokeWidth: command.StrokeWidth,
				Layer:       command.Layer,
			}
		case render.DrawPoint:
			commandDTO = CommandDTO{
				Type:   "point",
				Center: NewVec2DTO(command.Center),
				Radius: command.Radius,
				Color:  NewColorDTO(command.Color),
				Layer:  command.Layer,
			}
		case render.DrawArc:
			commandDTO = CommandDTO{
				Type:       "arc",
				Center:     NewVec2DTO(command.Center),
				Radius:     command.Radius,
				StartAngle: command.StartAngle,
				EndAngle:   command.EndAngle,
				Width:      command.Width,
				Color:      NewColorDTO(command.Color),
				Layer:      command.Layer,
			}
		default:
			continue
		}

		commandDTOs = append(commandDTOs, commandDTO)
	}

	return commandDTOs
}
