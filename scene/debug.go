package scene

import (
	"slices"
	"strconv"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

type DebugLayers uint32

const (
	DebugColliders DebugLayers = 1 << iota
	DebugContacts
	DebugGrid
	DebugActiveGrid
	DebugCandidateGrid
	DebugVelocities
	DebugEntityIDs
	DebugRenderLayers
	DebugInputState
	DebugRuntimeStats
)

type DebugOverlayConfig struct {
	Bounds   math2d.Rect
	CellSize float64
	Style    DebugStyle
}

type DebugOption func(*DebugOverlayConfig)

type DebugColliderStyle struct {
	ColliderColor       render.Color
	ColliderStrokeWidth float64
	ColliderLayer       int
}

type DebugContactStyle struct {
	ContactColor        render.Color
	ContactPointRadius  float64
	ContactLineWidth    float64
	ContactNormalLength float64
	ContactLayer        int
}

type DebugGridStyle struct {
	GridColor       render.Color
	GridDashLength  float64
	GridStrokeWidth float64
	GridLayer       int

	ActiveGridFill        render.Color
	ActiveGridStroke      render.Color
	ActiveGridStrokeWidth float64
	ActiveGridLayer       int

	CandidateGridFill        render.Color
	CandidateGridStroke      render.Color
	CandidateGridStrokeWidth float64
	CandidateGridLayer       int
}

type DebugVelocityStyle struct {
	VelocityScale     float64
	VelocityMinLength float64
	VelocityColor     render.Color
	VelocityWidth     float64
	VelocityLayer     int
}

type DebugEntityIDStyle struct {
	Color  render.Color
	Size   float64
	Offset math2d.Vec2
	Layer  int
}

type DebugStyle struct {
	Colliders DebugColliderStyle
	Contacts  DebugContactStyle
	Grid      DebugGridStyle
	Velocity  DebugVelocityStyle
	EntityIDs DebugEntityIDStyle
}

func (l DebugLayers) Has(flag DebugLayers) bool {
	return l&flag != 0
}

func DefaultDebugOverlayConfig() DebugOverlayConfig {
	return DebugOverlayConfig{
		CellSize: defaultCellSize,
		Style:    DefaultDebugStyle(),
	}
}

func WithDebugBounds(bounds math2d.Rect) DebugOption {
	return func(cfg *DebugOverlayConfig) {
		if bounds.Width() <= 0 || bounds.Height() <= 0 {
			return
		}
		cfg.Bounds = bounds
	}
}

func WithDebugCellSize(cellSize float64) DebugOption {
	return func(cfg *DebugOverlayConfig) {
		if cellSize <= 0 {
			return
		}
		cfg.CellSize = cellSize
	}
}

func WithDebugStyle(style DebugStyle) DebugOption {
	return func(cfg *DebugOverlayConfig) {
		if style == (DebugStyle{}) {
			return
		}
		cfg.Style = style
	}
}

func WithDebugOverlayConfig(overlay DebugOverlayConfig) DebugOption {
	return func(cfg *DebugOverlayConfig) {
		WithDebugBounds(overlay.Bounds)(cfg)
		WithDebugCellSize(overlay.CellSize)(cfg)
		WithDebugStyle(overlay.Style)(cfg)
	}
}

func (w *World) DebugCommands(layers DebugLayers, opts ...DebugOption) []render.Command {
	cfg := DefaultDebugOverlayConfig()

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	cmds := make([]render.Command, 0, 4)

	if layers.Has(DebugGrid) {
		grid := w.debugGridAreaCached(cfg)
		// The grid dwarfs every other layer in command count, so it decides
		// the capacity of the frame's slice rather than growing it by halves.
		cmds = append(make([]render.Command, 0, len(grid)+16), grid...)
	}
	if layers.Has(DebugActiveGrid) {
		cmds = append(cmds, w.debugActiveGridCommands(
			cfg.Style.Grid.ActiveGridFill,
			cfg.Style.Grid.ActiveGridStroke,
			cfg.Style.Grid.ActiveGridStrokeWidth,
			cfg.Style.Grid.ActiveGridLayer,
		)...)
	}
	if layers.Has(DebugCandidateGrid) {
		cmds = append(cmds, w.debugCandidateGridCommands(
			cfg.Style.Grid.CandidateGridFill,
			cfg.Style.Grid.CandidateGridStroke,
			cfg.Style.Grid.CandidateGridStrokeWidth,
			cfg.Style.Grid.CandidateGridLayer,
		)...)
	}
	if layers.Has(DebugColliders) {
		cmds = append(
			cmds,
			w.debugColliderCommands(
				cfg.Style.Colliders.ColliderColor,
				cfg.Style.Colliders.ColliderStrokeWidth,
				cfg.Style.Colliders.ColliderLayer,
			)...,
		)
		// The collision grid is collision geometry, so it shares the flag and
		// the style of the entity colliders.
		cmds = append(cmds, w.debugCollisionGridCommands(
			cfg.Style.Colliders.ColliderColor,
			cfg.Style.Colliders.ColliderStrokeWidth,
			cfg.Style.Colliders.ColliderLayer,
		)...)
	}
	if layers.Has(DebugContacts) {
		cmds = append(
			cmds,
			w.debugContactCommands(
				cfg.Style.Contacts.ContactColor,
				cfg.Style.Contacts.ContactPointRadius,
				cfg.Style.Contacts.ContactLineWidth,
				cfg.Style.Contacts.ContactNormalLength,
				cfg.Style.Contacts.ContactLayer,
			)...,
		)
	}
	if layers.Has(DebugVelocities) {
		cmds = append(
			cmds,
			w.debugVelocityCommands(
				cfg.Style.Velocity.VelocityColor,
				cfg.Style.Velocity.VelocityWidth,
				cfg.Style.Velocity.VelocityScale,
				cfg.Style.Velocity.VelocityLayer)...)
	}
	if layers.Has(DebugEntityIDs) {
		cmds = append(cmds, w.debugEntityIDCommands(
			cfg.Style.EntityIDs.Color,
			cfg.Style.EntityIDs.Size,
			cfg.Style.EntityIDs.Offset,
			cfg.Style.EntityIDs.Layer,
		)...)
	}

	return cmds
}

// debugCollisionGridCache memoizes the collision grid's solid-cell overlay.
// The grid mutates rarely (SetSolid), so the per-frame cost is one word-slice
// comparison instead of a full cell scan and command rebuild.
type debugCollisionGridCache struct {
	words       []uint64
	w, h        int
	origin      math2d.Vec2
	cellSize    math2d.Vec2
	color       render.Color
	strokeWidth float64
	layer       int
	commands    []render.Command
	built       bool
}

// debugCollisionGridCommands returns the solid-cell overlay of the collision
// grid, rebuilding it only when the grid or the style changed.
func (w *World) debugCollisionGridCommands(color render.Color, strokeWidth float64, layer int) []render.Command {
	g := w.collisionGrid
	if g == nil || g.W <= 0 || g.H <= 0 {
		return nil
	}
	cache := &w.debugCollisionGrid
	if cache.built &&
		cache.w == g.W && cache.h == g.H &&
		cache.origin == g.Origin && cache.cellSize == g.CellSize &&
		cache.color == color && cache.strokeWidth == strokeWidth && cache.layer == layer &&
		slices.Equal(cache.words, g.Words) {
		return cache.commands
	}

	rects := g.debugSolidCellRects()
	commands := make([]render.Command, 0, len(rects))
	for _, rect := range rects {
		commands = append(commands, render.DrawRect{
			Rect:        rect,
			Stroke:      color,
			StrokeWidth: strokeWidth,
			BaseCommand: render.BaseCommand{LayerValue: layer},
		})
	}

	cache.words = append(cache.words[:0], g.Words...)
	cache.w, cache.h = g.W, g.H
	cache.origin, cache.cellSize = g.Origin, g.CellSize
	cache.color, cache.strokeWidth, cache.layer = color, strokeWidth, layer
	cache.commands = commands
	cache.built = true
	return commands
}

// debugGridCache memoizes the grid overlay. The grid is a pure function of
// the bounds, the cell size and the style, none of which move while a game
// runs — but it is one command per dash, in the thousands, and every command
// is an interface value that has to be boxed on the heap. Rebuilding it each
// frame cost more than every other debug layer put together, and the garbage
// showed up as periodic frame drops rather than a steady tax.
type debugGridCache struct {
	bounds   math2d.Rect
	cellSize float64
	style    DebugGridStyle
	commands []render.Command
	built    bool
}

// debugGridAreaCached returns the grid for this configuration, rebuilding it
// only when the configuration it was built for has changed.
func (w *World) debugGridAreaCached(cfg DebugOverlayConfig) []render.Command {
	cache := &w.debugGrid
	if cache.built &&
		cache.bounds == cfg.Bounds &&
		cache.cellSize == cfg.CellSize &&
		cache.style == cfg.Style.Grid {
		return cache.commands
	}

	cache.bounds = cfg.Bounds
	cache.cellSize = cfg.CellSize
	cache.style = cfg.Style.Grid
	cache.commands = debugGridAreaCommands(
		cfg.Bounds,
		cfg.CellSize,
		cfg.Style.Grid.GridColor,
		cfg.Style.Grid.GridStrokeWidth,
		cfg.Style.Grid.GridDashLength,
		cfg.Style.Grid.GridLayer,
	)
	cache.built = true
	return cache.commands
}

func (w *World) debugVelocityCommands(color render.Color, width, scale float64, layer int) []render.Command {
	if width <= 0 || scale == 0 {
		return nil
	}

	commands := make([]render.Command, 0, w.velocities.len())

	// velocities.ids is already ascending, so iterate the store directly for
	// deterministic order without a redundant copy and sort.
	for i, id := range w.velocities.ids {
		velocity := w.velocities.dense[i]
		transform, ok := w.transforms.get(id)
		if !ok {
			continue
		}

		if velocity.Linear == (math2d.Vec2{}) {
			continue
		}

		from := transform.Position.Add(w.debugOffset(id, transform.Position))
		to := from.Add(velocity.Linear.Mul(scale))

		commands = append(commands, render.DrawLine{
			From:  from,
			To:    to,
			Width: width,
			Color: color,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func (w *World) debugEntityIDCommands(color render.Color, size float64, offset math2d.Vec2, layer int) []render.Command {
	if size <= 0 {
		return nil
	}

	commands := make([]render.Command, 0, w.transforms.len())

	// transforms.ids is already ascending, so iterate the store directly for
	// deterministic order without a redundant copy and sort.
	for i, id := range w.transforms.ids {
		transform := w.transforms.dense[i]

		commands = append(commands, render.DrawText{
			Position: transform.Position.
				Add(w.debugOffset(id, transform.Position)).
				Add(offset),
			Text:     strconv.FormatUint(uint64(id), 10),
			Color:    color,
			Size:     size,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func DefaultDebugStyle() DebugStyle {
	return DebugStyle{
		Colliders: DebugColliderStyle{
			ColliderColor:       render.RGBA(1, 1, 0, 1),
			ColliderStrokeWidth: 0.08,
			ColliderLayer:       100,
		},
		Contacts: DebugContactStyle{
			ContactColor:        render.RGBA(1, 0, 1, 1),
			ContactPointRadius:  0.18,
			ContactLineWidth:    0.12,
			ContactNormalLength: 1.25,
			ContactLayer:        101,
		},
		Grid: DebugGridStyle{
			GridColor:       render.RGBA(0.5, 0.5, 0.5, 0.4),
			GridDashLength:  1.5,
			GridStrokeWidth: 0.2,
			GridLayer:       200,

			ActiveGridFill:        render.RGBA(1, 0.55, 0.05, 0.22),
			ActiveGridStroke:      render.RGBA(1, 0.75, 0.05, 0.9),
			ActiveGridStrokeWidth: 0.12,
			ActiveGridLayer:       201,

			CandidateGridFill:        render.RGBA(1, 0.95, 0.05, 0.35),
			CandidateGridStroke:      render.RGBA(1, 1, 0.15, 1),
			CandidateGridStrokeWidth: 0.18,
			CandidateGridLayer:       202,
		},
		Velocity: DebugVelocityStyle{
			VelocityLayer: 103,
			VelocityColor: render.RGBA(0.5, 0.5, 0.5, 0.4),
			VelocityWidth: 0.3,
			VelocityScale: 0.1,
		},
		EntityIDs: DebugEntityIDStyle{
			Color:  render.RGBA(1, 1, 1, 0.5),
			Size:   2,
			Offset: math2d.Vec2{X: 0.5, Y: -0.5},
			Layer:  104,
		},
	}
}
