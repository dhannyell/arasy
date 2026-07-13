package uimanager

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/ui"
)

// ─── palette ──────────────────────────────────────────────────────────────────

var (
	bgDeep    = render.RGBA(0.039, 0.071, 0.110, 1)
	bgPanel   = render.RGBA(0.078, 0.129, 0.184, 0.96)
	bgRaised  = render.RGBA(0.106, 0.173, 0.243, 1)
	grass     = render.RGBA(0.184, 0.749, 0.443, 1)
	gold      = render.RGBA(0.941, 0.725, 0.231, 1)
	loss      = render.RGBA(0.898, 0.282, 0.302, 1)
	drawColor = render.RGBA(0.541, 0.608, 0.667, 1)
	textColor = render.RGBA(0.949, 0.961, 0.969, 1)
	muted     = render.RGBA(0.494, 0.561, 0.627, 1)
	line      = render.RGBA(1, 1, 1, 0.08)
	zebra     = render.RGBA(1, 1, 1, 0.035)
	dfBlue    = render.RGBA(0.231, 0.506, 0.965, 1) // #3B82F6 — position badge
)

// ─── stroke helper ────────────────────────────────────────────────────────────

type strokeSpec struct {
	color render.Color
	width float64
}

// ─── theme ────────────────────────────────────────────────────────────────────

func matchdayTheme() ui.Theme {
	t := ui.DefaultTheme()
	t.Button.Normal.Background = gold
	t.Button.Normal.TextColor = bgDeep
	t.Text.Color = textColor
	t.Text.Muted = muted
	t.Header.Background = bgPanel
	t.Header.TextColor = textColor
	t.Header.Border = line
	t.Row.TextColor = textColor
	t.Row.Normal = render.Color{}
	t.Row.Zebra = zebra
	t.Row.Hovered = render.RGBA(1, 1, 1, 0.06)
	t.Row.Selected = render.RGBA(0.941, 0.725, 0.231, 0.16)
	return t
}

// ─── input bindings ───────────────────────────────────────────────────────────

func matchdayBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyEscape), Action: input.ActionPause, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: input.ActionMoveX, Scale: -1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: input.ActionMoveX, Scale: 1},
		{Source: input.Keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: -1},
	}
}
