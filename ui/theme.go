package ui

import "github.com/dhannyell/arasy/render"

// Theme groups widget styles used by runtime.Config.
type Theme struct {
	// Font is the manifest FontID every themed widget paints its text with.
	// Zero keeps the built-in engine font, so an unset theme renders exactly
	// as before. Games resolve the id from their manifest (ctx.Assets.Font)
	// and set it once here.
	Font     render.FontID
	Button   ButtonTheme
	Label    LabelTheme
	Panel    PanelTheme
	Progress ProgressBarTheme
	Slider   SliderTheme
	Card     CardTheme
	Row      RowTheme
	Header   HeaderTheme
	Badge    BadgeTheme
	Text      TextTheme
	Focus     FocusTheme
	Tabs      TabsTheme
	TextInput TextInputTheme
	Checkbox  CheckboxTheme
	Dropdown  DropdownTheme
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

// SliderTheme defines the horizontal track and thumb appearance.
type SliderTheme struct {
	Track        render.Color
	Fill         render.Color
	Border       render.Color
	Thumb        render.Color
	ThumbHovered render.Color
	ThumbActive  render.Color
	TrackHeight  float64
	ThumbRadius  float64
	BorderWidth  float64
}

type CardTheme struct {
	Background   render.Color
	Border       render.Color
	BorderWidth  float64
	CornerRadius float64
	AccentWidth  float64
}

type RowTheme struct {
	Normal    render.Color
	Hovered   render.Color
	Selected  render.Color
	Zebra     render.Color
	Height    float64
	TextColor render.Color
}

type HeaderTheme struct {
	Background   render.Color
	TextColor    render.Color
	Height       float64
	Border       render.Color
	BorderWidth  float64
	CornerRadius float64
}

type BadgeTheme struct {
	Background   render.Color
	TextColor    render.Color
	Border       render.Color
	BorderWidth  float64
	CornerRadius float64
	PaddingX     float64
	PaddingY     float64
	TextSize     float64
}

type TextTheme struct {
	Display float64
	Heading float64
	Body    float64
	Caption float64
	Color   render.Color
	Muted   render.Color
}

// TabsTheme styles the header band of a tab bar. BarHeight is the height of
// the band; the body below it belongs to the caller.
type TabsTheme struct {
	BarHeight    float64
	Bar          render.Color
	TabActive    render.Color
	TabInactive  render.Color
	TabHovered   render.Color
	TextColor    render.Color
	TextSize     float64
	Border       render.Color
	BorderWidth  float64
	CornerRadius float64
}

// TextInputTheme styles a text field. BlinkPeriod is the full caret cycle in
// seconds of render time, so it never touches simulation state.
type TextInputTheme struct {
	Background        render.Color
	BackgroundHovered render.Color
	BackgroundFocused render.Color
	Border            render.Color
	BorderWidth       float64
	CornerRadius      float64
	TextColor         render.Color
	PlaceholderColor  render.Color
	CaretColor        render.Color
	CaretWidth        float64
	TextSize          float64
	PaddingX          float64
	PaddingY          float64
	BlinkPeriod       float64
}

// CheckboxTheme styles the box and its check mark. BoxSize is the side of the
// square, centred vertically in the rect the caller passes.
type CheckboxTheme struct {
	Background        render.Color
	BackgroundHovered render.Color
	BackgroundChecked render.Color
	Border            render.Color
	BorderWidth       float64
	CornerRadius      float64
	Mark              render.Color
	MarkWidth         float64
	BoxSize           float64
	LabelGap          float64
	TextColor         render.Color
	TextSize          float64
}

// DropdownTheme styles the closed button and the popup list it opens.
// MaxVisibleRows caps the popup height; longer lists scroll.
type DropdownTheme struct {
	Background        render.Color
	BackgroundHovered render.Color
	BackgroundOpen    render.Color
	Border            render.Color
	BorderWidth       float64
	CornerRadius      float64
	TextColor         render.Color
	TextSize          float64
	PaddingX          float64
	ArrowSize         float64
	PopupBackground   render.Color
	MaxVisibleRows    int
}

type FocusTheme struct {
	Color        render.Color
	Width        float64
	CornerRadius float64
	Inset        float64
}

// DefaultTheme is semi-transparent dark background, white text, subtle borders.
func DefaultTheme() Theme {
	white := render.RGBA(0.95, 0.96, 0.98, 1)
	subtle := render.RGBA(1, 1, 1, 0.12) // borda discreta
	panelBg := render.RGBA(0.07, 0.09, 0.12, 0.85)
	btnBg := render.RGBA(0.16, 0.19, 0.24, 0.92)
	btnHover := render.RGBA(0.22, 0.26, 0.33, 0.96)
	btnActive := render.RGBA(0.30, 0.36, 0.45, 1)
	accent := render.RGBA(0.30, 0.70, 0.95, 1)
	muted := render.RGBA(0.68, 0.72, 0.78, 1)

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
			Fill:        accent,
			Border:      subtle,
			BorderWidth: 1,
		},
		Slider: SliderTheme{
			Track:        render.RGBA(1, 1, 1, 0.10),
			Fill:         accent,
			Border:       subtle,
			Thumb:        white,
			ThumbHovered: render.RGBA(0.82, 0.91, 1, 1),
			ThumbActive:  accent,
			TrackHeight:  6,
			ThumbRadius:  8,
			BorderWidth:  1,
		},
		Card: CardTheme{
			Background:   panelBg,
			Border:       subtle,
			BorderWidth:  1,
			CornerRadius: 8,
			AccentWidth:  4,
		},
		Row: RowTheme{
			Normal:    render.RGBA(1, 1, 1, 0.00),
			Hovered:   render.RGBA(1, 1, 1, 0.08),
			Selected:  render.RGBA(0.30, 0.70, 0.95, 0.18),
			Zebra:     render.RGBA(1, 1, 1, 0.04),
			Height:    32,
			TextColor: white,
		},
		Header: HeaderTheme{
			Background:   render.RGBA(1, 1, 1, 0.08),
			TextColor:    white,
			Height:       36,
			Border:       subtle,
			BorderWidth:  1,
			CornerRadius: 6,
		},
		Badge: BadgeTheme{
			Background:   render.RGBA(0.30, 0.70, 0.95, 0.22),
			TextColor:    white,
			Border:       render.RGBA(0.30, 0.70, 0.95, 0.45),
			BorderWidth:  1,
			CornerRadius: 6,
			PaddingX:     8,
			PaddingY:     3,
			TextSize:     12,
		},
		Text: TextTheme{
			Display: 40,
			Heading: 24,
			Body:    16,
			Caption: 12,
			Color:   white,
			Muted:   muted,
		},
		Focus: FocusTheme{
			Color:        accent,
			Width:        2,
			CornerRadius: 6,
			Inset:        2,
		},
		TextInput: TextInputTheme{
			Background:        render.RGBA(0, 0, 0, 0.25),
			BackgroundHovered: render.RGBA(1, 1, 1, 0.06),
			BackgroundFocused: render.RGBA(0, 0, 0, 0.35),
			Border:            subtle,
			BorderWidth:       1,
			CornerRadius:      6,
			TextColor:         white,
			PlaceholderColor:  muted,
			CaretColor:        white,
			CaretWidth:        2,
			TextSize:          16,
			PaddingX:          8,
			PaddingY:          6,
			BlinkPeriod:       1,
		},
		Checkbox: CheckboxTheme{
			Background:        render.RGBA(0, 0, 0, 0.25),
			BackgroundHovered: render.RGBA(1, 1, 1, 0.10),
			BackgroundChecked: accent,
			Border:            subtle,
			BorderWidth:       1,
			CornerRadius:      4,
			Mark:              white,
			MarkWidth:         2,
			BoxSize:           18,
			LabelGap:          8,
			TextColor:         white,
			TextSize:          14,
		},
		Dropdown: DropdownTheme{
			Background:        btnBg,
			BackgroundHovered: btnHover,
			BackgroundOpen:    btnActive,
			Border:            subtle,
			BorderWidth:       1,
			CornerRadius:      6,
			TextColor:         white,
			TextSize:          14,
			PaddingX:          8,
			ArrowSize:         8,
			PopupBackground:   render.RGBA(0.10, 0.12, 0.16, 0.98),
			MaxVisibleRows:    8,
		},
		Tabs: TabsTheme{
			BarHeight:    32,
			Bar:          render.RGBA(1, 1, 1, 0.04),
			TabActive:    btnActive,
			TabInactive:  btnBg,
			TabHovered:   btnHover,
			TextColor:    white,
			TextSize:     14,
			Border:       subtle,
			BorderWidth:  1,
			CornerRadius: 6,
		},
	}
}

func WithDefaults(theme Theme) Theme {
	defaults := DefaultTheme()
	if theme.Button == (ButtonTheme{}) {
		theme.Button = defaults.Button
	}
	if theme.Label == (LabelTheme{}) {
		theme.Label = defaults.Label
	}
	if theme.Panel == (PanelTheme{}) {
		theme.Panel = defaults.Panel
	}
	if theme.Progress == (ProgressBarTheme{}) {
		theme.Progress = defaults.Progress
	}
	if theme.Slider == (SliderTheme{}) {
		theme.Slider = defaults.Slider
	}
	if theme.Card == (CardTheme{}) {
		theme.Card = defaults.Card
	}
	if theme.Row == (RowTheme{}) {
		theme.Row = defaults.Row
	}
	if theme.Header == (HeaderTheme{}) {
		theme.Header = defaults.Header
	}
	if theme.Badge == (BadgeTheme{}) {
		theme.Badge = defaults.Badge
	}
	if theme.Text == (TextTheme{}) {
		theme.Text = defaults.Text
	}
	if theme.Focus == (FocusTheme{}) {
		theme.Focus = defaults.Focus
	}
	if theme.Tabs == (TabsTheme{}) {
		theme.Tabs = defaults.Tabs
	}
	if theme.TextInput == (TextInputTheme{}) {
		theme.TextInput = defaults.TextInput
	}
	if theme.Checkbox == (CheckboxTheme{}) {
		theme.Checkbox = defaults.Checkbox
	}
	if theme.Dropdown == (DropdownTheme{}) {
		theme.Dropdown = defaults.Dropdown
	}
	return theme
}
