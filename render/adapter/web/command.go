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

	Radius      float64 `json:"radius,omitempty"`
	StartAngle  float64 `json:"startAngle,omitempty"`
	EndAngle    float64 `json:"endAngle,omitempty"`
	Width       float64 `json:"width,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`

	Fill   *ColorDTO `json:"fill,omitempty"`
	Stroke *ColorDTO `json:"stroke,omitempty"`
	Color  *ColorDTO `json:"color,omitempty"`

	Text  string  `json:"text,omitempty"`
	Size  float64 `json:"size,omitempty"`
	Layer int     `json:"layer"`
}

func NewCommandDTOs(commands []render.Command) []CommandDTO {
	commands = append([]render.Command(nil), commands...)
	sort.SliceStable(commands, func(i, j int) bool {
		return commandLayer(commands[i]) < commandLayer(commands[j])
	})

	commandDTOs := make([]CommandDTO, 0, len(commands))

	for _, command := range commands {
		var commandDTO CommandDTO

		switch command := command.(type) {
		case render.DrawRect:
			commandDTO = CommandDTO{
				Type: "rect",

				Rect:        new(NewRectDTO(command.Rect)),
				Fill:        new(NewColorDTO(command.Fill)),
				Stroke:      new(NewColorDTO(command.Stroke)),
				StrokeWidth: command.StrokeWidth,
				Layer:       command.Layer,
			}
		case render.DrawLine:
			commandDTO = CommandDTO{
				Type: "line",

				From:  new(NewVec2DTO(command.From)),
				To:    new(NewVec2DTO(command.To)),
				Width: command.Width,
				Color: new(NewColorDTO(command.Color)),
				Layer: command.Layer,
			}
		case render.DrawCircle:
			commandDTO = CommandDTO{
				Type: "circle",

				Center:      new(NewVec2DTO(command.Center)),
				Radius:      command.Radius,
				Fill:        new(NewColorDTO(command.Fill)),
				Stroke:      new(NewColorDTO(command.Stroke)),
				StrokeWidth: command.StrokeWidth,
				Layer:       command.Layer,
			}
		case render.DrawPoint:
			commandDTO = CommandDTO{
				Type:   "point",
				Center: new(NewVec2DTO(command.Center)),
				Radius: command.Radius,
				Color:  new(NewColorDTO(command.Color)),
				Layer:  command.Layer,
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
				Layer:      command.Layer,
			}
		case render.DrawText:
			commandDTO = CommandDTO{
				Type:     "text",
				Position: new(NewVec2DTO(command.Position)),
				Text:     command.Text,
				Color:    new(NewColorDTO(command.Color)),
				Size:     command.Size,
				Layer:    command.Layer,
			}
		default:
			continue
		}

		commandDTOs = append(commandDTOs, commandDTO)
	}

	return commandDTOs
}

func commandLayer(command render.Command) int {
	switch command := command.(type) {
	case render.DrawRect:
		return command.Layer
	case render.DrawCircle:
		return command.Layer
	case render.DrawPoint:
		return command.Layer
	case render.DrawLine:
		return command.Layer
	case render.DrawText:
		return command.Layer
	case render.DrawArc:
		return command.Layer
	default:
		return 0
	}
}
