package ui

import "github.com/dhannyell/frameforge/render"

// Theme groups widget styles used by runtime.Config.
type Theme struct {
	Button   ButtonTheme
	Label    LabelTheme
	Panel    PanelTheme
	Progress ProgressBarTheme
}

// ButtonStyle describes one button state.
type ButtonStyle struct {
	Background  render.Color
	Border      render.Color
	BorderWidth float64
	TextColor   render.Color
	TextSize    float64 // viewport pixels
}

// ButtonTheme defines button states and transition speeds.
type ButtonTheme struct {
	Normal     ButtonStyle
	Hovered    ButtonStyle
	Active     ButtonStyle
	HoverSpeed float64 // 1/s between Normal and Hovered
	PressSpeed float64 // 1/s entering or leaving Active
}

type LabelTheme struct {
	Color render.Color
	Size  float64 // em pixels de viewport
}

type PanelTheme struct {
	Background  render.Color
	Border      render.Color
	BorderWidth float64
}

type ProgressBarTheme struct {
	Background  render.Color // trilho
	Fill        render.Color // preenchimento (0..value)
	Border      render.Color
	BorderWidth float64
}

// DefaultTheme is semi-transparent dark background, white text, subtle borders.
func DefaultTheme() Theme {
	white := render.RGBA(0.95, 0.96, 0.98, 1)
	subtle := render.RGBA(1, 1, 1, 0.12) // borda discreta
	panelBg := render.RGBA(0.07, 0.09, 0.12, 0.85)
	btnBg := render.RGBA(0.16, 0.19, 0.24, 0.92)
	btnHover := render.RGBA(0.22, 0.26, 0.33, 0.96)
	btnActive := render.RGBA(0.30, 0.36, 0.45, 1)

	return Theme{
		Button: ButtonTheme{
			Normal:     ButtonStyle{Background: btnBg, Border: subtle, BorderWidth: 1, TextColor: white, TextSize: 16},
			Hovered:    ButtonStyle{Background: btnHover, Border: subtle, BorderWidth: 1, TextColor: white, TextSize: 16},
			Active:     ButtonStyle{Background: btnActive, Border: subtle, BorderWidth: 1, TextColor: white, TextSize: 16},
			HoverSpeed: 12,
			PressSpeed: 20,
		},
		Label: LabelTheme{Color: white, Size: 16},
		Panel: PanelTheme{Background: panelBg, Border: subtle, BorderWidth: 1},
		Progress: ProgressBarTheme{
			Background:  render.RGBA(1, 1, 1, 0.10),
			Fill:        render.RGBA(0.30, 0.70, 0.95, 1),
			Border:      subtle,
			BorderWidth: 1,
		},
	}
}
