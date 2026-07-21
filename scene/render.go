package scene

import (
	"math"
	"slices"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func (w *World) RenderCommands() []render.Command {
	return w.RenderCommandsInto(nil, 0)
}

// RenderCommandsInterpolated returns render commands with transforms
// interpolated by alpha between the previous and current simulation state.
// alpha=0 produces the current (most recent) pose; alpha=0.5 is halfway
// between previous and current. Debug colliders/grid/velocity are always
// rendered at the current simulation state, not interpolated.
//
// Interpolation is a presentation cache: it never enters Checksum or recorded
// replays, though World.Clone preserves it so restored frames avoid smear.
func (w *World) RenderCommandsInterpolated(alpha float64) []render.Command {
	return w.RenderCommandsInto(nil, alpha)
}

// RenderCommandsInto appends this world's render commands to dst and returns
// the extended slice, letting callers reuse their command buffer across
// frames. Commands remain ordered by entity ID.
func (w *World) RenderCommandsInto(dst []render.Command, alpha float64) []render.Command {
	totalSprites := w.circleSprites.len() + w.rectSprites.len() + w.imageSprites.len()
	ids := w.renderIDScratch[:0]
	if cap(ids) < totalSprites {
		ids = make([]EntityID, 0, totalSprites)
	}

	ids = append(ids, w.circleSprites.ids...)
	ids = append(ids, w.rectSprites.ids...)
	ids = append(ids, w.imageSprites.ids...)

	// Depth sort: Layer dominates, then Y position (lower Y = drawn first = behind),
	// then EntityID for determinism.
	slices.SortFunc(ids, func(a, b EntityID) int {
		layerA, layerB := w.spriteLayer(a), w.spriteLayer(b)
		if layerA != layerB {
			return layerA - layerB
		}
		ta, aok := w.transforms.get(a)
		tb, bok := w.transforms.get(b)
		if aok && bok {
			if ta.Position.Y < tb.Position.Y {
				return -1
			}
			if ta.Position.Y > tb.Position.Y {
				return 1
			}
		}
		return int(a) - int(b)
	})
	w.renderIDScratch = ids

	commands := dst
	if commands == nil {
		commands = make([]render.Command, 0, totalSprites)
	}
	useAlpha := alpha
	if useAlpha <= 0 || useAlpha != useAlpha {
		useAlpha = 0
	} else if useAlpha >= 1 {
		useAlpha = math.Nextafter(1, 0)
	}

	hasProjection := !w.projection.IsZero()

	for _, id := range ids {
		transform, ok := w.transforms.get(id)
		if !ok {
			continue
		}

		interpolated := transform
		if useAlpha > 0 {
			if prev, hasPrev := w.renderTransforms.get(id); hasPrev {
				interpolated.Position = prev.Position.Lerp(transform.Position, useAlpha)
				interpolated.Rotation = math2d.Lerp(prev.Rotation, transform.Rotation, useAlpha)
				interpolated.Scale = prev.Scale.Lerp(transform.Scale, useAlpha)
			}
		}

		// Height projection: lift + scale + shadow (render-only).
		pos := interpolated.Position
		scl := interpolated.Scale
		if hasProjection {
			if h, hasH := w.heights.get(id); hasH {
				z := h.Z
				if useAlpha > 0 {
					if prevH, hasPrevH := w.renderHeights.get(id); hasPrevH {
						z = math2d.Lerp(prevH.Z, h.Z, useAlpha)
					}
				}
				if z > 0 {
					ground := pos
					fp := w.spriteFootprint(id)
					proj := w.projection.Project(ground, fp, z)
					// Ground shadow first, so it draws behind the lifted body.
					// Same layer, emitted adjacent to the body, so the depth
					// sort keeps the composite entity's commands together.
					if proj.ShadowAlpha > 0 && (proj.ShadowRadii.X > 0 || proj.ShadowRadii.Y > 0) {
						commands = append(commands, render.DrawEllipse{
							Center:  ground.Add(proj.ShadowCenter),
							RadiusX: proj.ShadowRadii.X,
							RadiusY: proj.ShadowRadii.Y,
							Fill:    render.RGBA(0, 0, 0, proj.ShadowAlpha),
							BaseCommand: render.BaseCommand{
								LayerValue: w.spriteLayer(id),
							},
						})
					}
					pos = proj.BodyPosition
					if proj.BodyScale > 0 {
						scl = scl.Mul(proj.BodyScale)
					}
				}
			}
		}

		if circle, hasCircle := w.circleSprites.get(id); hasCircle {
			r := circle.Radius
			if scl.X > 0 {
				r *= scl.X
			}
			commands = append(commands, render.DrawCircle{
				Center: pos,
				Radius: r,
				Fill:   circle.Color,
				BaseCommand: render.BaseCommand{
					LayerValue: circle.Layer,
				},
			})
			continue
		}

		if rect, hasRect := w.rectSprites.get(id); hasRect {
			commands = append(commands, render.DrawRect{
				Rect:        rect.Rect,
				Fill:        rect.Color,
				Stroke:      rect.Stroke,
				StrokeWidth: rect.StrokeWidth,
				BaseCommand: render.BaseCommand{
					LayerValue: rect.Layer,
				},
			})
		}

		if img, hasImage := w.imageSprites.get(id); hasImage {
			halfSize := img.Size.Mul(0.5)
			scaledHalf := math2d.Vec2{
				X: halfSize.X * scl.X,
				Y: halfSize.Y * scl.Y,
			}
			commands = append(commands, render.DrawImage{
				Texture: img.Texture,
				Dest: math2d.NewRectFromCorners(
					pos.Sub(scaledHalf),
					pos.Add(scaledHalf),
				),
				Src:      img.Src,
				Tint:     img.Tint,
				Rotation: interpolated.Rotation,
				BaseCommand: render.BaseCommand{
					LayerValue: img.Layer,
				},
			})
		}
	}

	return commands
}

// spriteLayer returns the layer of the first sprite found for the entity.
func (w *World) spriteLayer(id EntityID) int {
	if s, ok := w.circleSprites.get(id); ok {
		return s.Layer
	}
	if s, ok := w.rectSprites.get(id); ok {
		return s.Layer
	}
	if s, ok := w.imageSprites.get(id); ok {
		return s.Layer
	}
	return 0
}

// spriteFootprint returns the half-extents of the entity's sprite footprint,
// used by Projection for shadow positioning.
func (w *World) spriteFootprint(id EntityID) math2d.Vec2 {
	if s, ok := w.circleSprites.get(id); ok {
		return math2d.Vec2{X: s.Radius, Y: s.Radius}
	}
	if s, ok := w.rectSprites.get(id); ok {
		return math2d.Vec2{
			X: (s.Rect.Max.X - s.Rect.Min.X) * 0.5,
			Y: (s.Rect.Max.Y - s.Rect.Min.Y) * 0.5,
		}
	}
	if s, ok := w.imageSprites.get(id); ok {
		return math2d.Vec2{X: s.Size.X * 0.5, Y: s.Size.Y * 0.5}
	}
	return math2d.Vec2{}
}

func (w *World) SyncRenderState() {
	w.renderTransforms.copyFrom(&w.transforms)
	w.renderHeights.copyFrom(&w.heights)
}

func (w *World) debugColliderCommands(color render.Color, strokeWidth float64, layer int) []render.Command {
	ids := make([]EntityID, 0, w.circleColliders.len()+w.rectColliders.len())
	ids = append(ids, w.circleColliders.ids...)
	for _, id := range w.rectColliders.ids {
		if _, isCircle := w.circleColliders.get(id); isCircle {
			// circle collider takes precedence on the same entity (see
			// detectOverlapPair); don't draw the ignored rect.
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)

	commands := make([]render.Command, 0, len(ids))

	for _, id := range ids {
		transform, ok := w.transforms.get(id)
		if !ok {
			continue
		}

		if collider, isCircle := w.circleColliders.get(id); isCircle {
			commands = append(commands, render.DrawCircle{
				Center:      transform.Position,
				Radius:      collider.Radius,
				Stroke:      color,
				StrokeWidth: strokeWidth,
				BaseCommand: render.BaseCommand{
					LayerValue: layer,
				},
			})
			continue
		}

		if collider, isRect := w.rectColliders.get(id); isRect {
			half := collider.HalfExtends
			commands = append(commands, render.DrawRect{
				Rect: math2d.NewRectFromCorners(
					transform.Position.Sub(half),
					transform.Position.Add(half),
				),
				Stroke:      color,
				StrokeWidth: strokeWidth,
				BaseCommand: render.BaseCommand{
					LayerValue: layer,
				},
			})
		}
	}

	return commands
}

func (w *World) debugContactCommands(color render.Color, pointRadius, lineWidth, normalLength float64, layer int) []render.Command {
	contacts := w.Contacts()
	commands := make([]render.Command, 0, len(contacts)*2)

	for _, contact := range contacts {
		transformA, ok := w.transforms.get(contact.A)
		if !ok {
			continue
		}

		transformB, ok := w.transforms.get(contact.B)
		if !ok {
			continue
		}

		point := transformA.Position.Lerp(transformB.Position, 0.5)
		normalEnd := point.Add(contact.Normal.Mul(normalLength))

		commands = append(commands,
			render.DrawPoint{
				Center: point,
				Radius: pointRadius,
				Color:  color,
				BaseCommand: render.BaseCommand{
					LayerValue: layer,
				},
			},
			render.DrawLine{
				From:  point,
				To:    normalEnd,
				Width: lineWidth,
				Color: color,
				BaseCommand: render.BaseCommand{
					LayerValue: layer,
				},
			},
		)
	}

	return commands
}

func (w *World) debugGridCommands(color render.Color, strokeWidth, dashLength float64, layer int) []render.Command {
	if len(w.debugGridCells) == 0 {
		return nil
	}

	commands := make([]render.Command, 0, len(w.debugGridCells)*4)

	dashStep := dashLength
	gap := dashLength * 0.5

	for _, cell := range w.debugGridCells {
		cellSize := cell.Size
		if cellSize <= 0 {
			cellSize = defaultCellSize
		}

		x0 := float64(cell.X) * cellSize
		y0 := float64(cell.Y) * cellSize
		x1 := x0 + cellSize
		y1 := y0 + cellSize

		commands = append(commands, dashedLine(x0, y0, x1, y0, strokeWidth, dashStep, gap, color, layer)...)
		commands = append(commands, dashedLine(x1, y0, x1, y1, strokeWidth, dashStep, gap, color, layer)...)
		commands = append(commands, dashedLine(x1, y1, x0, y1, strokeWidth, dashStep, gap, color, layer)...)
		commands = append(commands, dashedLine(x0, y1, x0, y0, strokeWidth, dashStep, gap, color, layer)...)
	}

	return commands
}

func (w *World) debugActiveGridCommands(fill, stroke render.Color, strokeWidth float64, layer int) []render.Command {
	if len(w.debugGridCells) == 0 {
		return nil
	}

	commands := make([]render.Command, 0, len(w.debugGridCells))

	for _, cell := range w.debugGridCells {
		cellSize := cell.Size
		if cellSize <= 0 {
			cellSize = defaultCellSize
		}

		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{
					X: float64(cell.X) * cellSize,
					Y: float64(cell.Y) * cellSize,
				},
				math2d.Vec2{
					X: cellSize,
					Y: cellSize,
				},
			),
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: strokeWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func (w *World) debugCandidateGridCommands(fill, stroke render.Color, strokeWidth float64, layer int) []render.Command {
	if len(w.debugGridCells) == 0 {
		return nil
	}

	commands := make([]render.Command, 0, len(w.debugGridCells))

	for _, cell := range w.debugGridCells {
		if !cell.Candidate {
			continue
		}

		cellSize := cell.Size
		if cellSize <= 0 {
			cellSize = defaultCellSize
		}

		commands = append(commands, render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{
					X: float64(cell.X) * cellSize,
					Y: float64(cell.Y) * cellSize,
				},
				math2d.Vec2{
					X: cellSize,
					Y: cellSize,
				},
			),
			Fill:        fill,
			Stroke:      stroke,
			StrokeWidth: strokeWidth,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
	}

	return commands
}

func debugGridAreaCommands(bounds math2d.Rect, cellSize float64, color render.Color, strokeWidth, dashLength float64, layer int) []render.Command {
	if cellSize <= 0 || bounds.Width() <= 0 || bounds.Height() <= 0 {
		return nil
	}

	dashStep := dashLength
	gap := dashLength * 0.5
	commands := make([]render.Command, 0)

	minX := bounds.Min.X
	minY := bounds.Min.Y
	maxX := bounds.Max.X
	maxY := bounds.Max.Y

	for x := firstGridLine(minX, cellSize); x <= maxX; x += cellSize {
		commands = append(commands, dashedLine(x, minY, x, maxY, strokeWidth, dashStep, gap, color, layer)...)
	}

	for y := firstGridLine(minY, cellSize); y <= maxY; y += cellSize {
		commands = append(commands, dashedLine(minX, y, maxX, y, strokeWidth, dashStep, gap, color, layer)...)
	}

	commands = append(commands, dashedLine(minX, minY, maxX, minY, strokeWidth, dashStep, gap, color, layer)...)
	commands = append(commands, dashedLine(maxX, minY, maxX, maxY, strokeWidth, dashStep, gap, color, layer)...)
	commands = append(commands, dashedLine(maxX, maxY, minX, maxY, strokeWidth, dashStep, gap, color, layer)...)
	commands = append(commands, dashedLine(minX, maxY, minX, minY, strokeWidth, dashStep, gap, color, layer)...)

	return commands
}

func firstGridLine(min, cellSize float64) float64 {
	return math.Ceil(min/cellSize) * cellSize
}

func dashedLine(x1, y1, x2, y2, width, dashLen, gap float64, color render.Color, layer int) []render.Command {
	dx := x2 - x1
	dy := y2 - y1
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return nil
	}

	ux := dx / length
	uy := dy / length

	var commands []render.Command
	t := 0.0
	for t < length {
		segLen := math.Min(dashLen, length-t)
		commands = append(commands, render.DrawLine{
			From:  math2d.Vec2{X: x1 + ux*t, Y: y1 + uy*t},
			To:    math2d.Vec2{X: x1 + ux*(t+segLen), Y: y1 + uy*(t+segLen)},
			Width: width,
			Color: color,
			BaseCommand: render.BaseCommand{
				LayerValue: layer,
			},
		})
		t += segLen + gap
	}

	return commands
}
