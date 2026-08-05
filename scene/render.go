package scene

import (
	"math"
	"slices"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
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
	totalSprites := w.circleSprites.len() + w.rectSprites.len() + w.imageSprites.len() + w.commandSprites.len()
	sorted := w.sortSprites(totalSprites)

	commands := dst
	if commands == nil {
		commands = make([]render.Command, 0, totalSprites)
	}
	useAlpha := alpha
	if useAlpha <= 0 || math.IsNaN(useAlpha) {
		useAlpha = 0
	} else if useAlpha >= 1 {
		useAlpha = math.Nextafter(1, 0)
	}

	hasProjection := !w.projection.IsZero()

	// One camera distortion for the whole frame, and so one correction for
	// every sprite that has to stand up against it. Without a projection this
	// stays absent and every plane correction below is skipped, so a game that
	// never asks for 2.5D does not pay for it existing.
	upright := math2d.Affine{}
	hasUpright := false
	if hasProjection {
		upright, hasUpright = w.projection.UprightCorrection()
	}

	for _, entry := range sorted {
		id := entry.id

		// A command sprite is emitted as authored: its commands already carry
		// their own geometry, and its place in the painter order -- the whole
		// reason it exists -- was settled by the sort above. Only the layer is
		// rewritten, so the group draws as one body on the sprite's layer.
		if cs, hasCommands := w.commandSprites.get(id); hasCommands {
			for _, c := range cs.Commands {
				commands = append(commands, render.WithLayer(c, cs.Layer))
			}
			continue
		}

		// An attached sprite takes everything positional from its parent: the
		// sort already keyed it at the parent's depth, and here it borrows the
		// parent's interpolated transform so it can never trail behind the
		// body it is part of.
		att, isAttached := w.attachments.get(id)
		anchorID := id
		if isAttached {
			anchorID = att.Parent
		}

		transform, hasTransform := w.transforms.get(anchorID)
		if !hasTransform {
			// The sort established the anchor's Transform existed; an attached
			// sprite whose parent despawned mid-frame simply vanishes with it.
			continue
		}

		interpolated := transform
		if useAlpha > 0 {
			if prev, hasPrev := w.renderTransforms.get(anchorID); hasPrev {
				interpolated.Position = prev.Position.Lerp(transform.Position, useAlpha)
				interpolated.Rotation = math2d.Lerp(prev.Rotation, transform.Rotation, useAlpha)
				interpolated.Scale = prev.Scale.Lerp(transform.Scale, useAlpha)
			}
		}

		// Height projection: lift + scale + shadow (render-only).
		pos := interpolated.Position
		scl := interpolated.Scale
		if isAttached {
			pos = pos.Add(att.Offset)

			// The anchor rides the parent's height plus its own offset up the
			// lift axis. No shadow: the parent's already marks the ground for
			// the whole composite.
			if hasProjection {
				z := att.HeightOffset
				if h, hasH := w.heights.get(att.Parent); hasH {
					parentZ := h.Z
					if useAlpha > 0 {
						if prevH, hasPrevH := w.renderHeights.get(att.Parent); hasPrevH {
							parentZ = math2d.Lerp(prevH.Z, h.Z, useAlpha)
						}
					}
					z += parentZ
				}
				if z > 0 {
					proj := w.projection.Project(pos, math2d.Vec2{}, z)
					pos = proj.BodyPosition
					if proj.BodyScale > 0 {
						scl = scl.Mul(proj.BodyScale)
					}
				}
			}
		} else if hasProjection {
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
						// No plane correction: the shadow lies on the ground by
						// definition, which is the one thing the camera already
						// draws correctly on its own.
						commands = append(commands, render.DrawEllipse{
							Center:  ground.Add(proj.ShadowCenter),
							RadiusX: proj.ShadowRadii.X,
							RadiusY: proj.ShadowRadii.Y,
							Fill:    render.RGBA(0, 0, 0, proj.ShadowAlpha),
							BaseCommand: render.BaseCommand{
								LayerValue: w.spriteLayer(id),
								Transform:  proj.ShadowTransform,
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

		// Per-entity decorative affine (squash & stretch, lean), deforming
		// about the final — possibly lifted — body center. Render-only.
		bodyTransform := math2d.Affine{}
		if st, ok := w.spriteTransforms.get(id); ok && !st.M.IsIdentity() {
			bodyTransform = st.M.About(pos)
		}

		// The plane correction is the outer one: the decorative affine deforms
		// the sprite in its own frame, and only then is the result placed on
		// the plane it declares. Composing them the other way round would tilt
		// squash & stretch by the camera.
		if hasUpright && w.spritePlane(id) == PlaneUpright {
			plane := upright.About(pos)
			if bodyTransform.IsIdentity() {
				bodyTransform = plane
			} else {
				bodyTransform = plane.Mul(bodyTransform)
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
					Transform:  bodyTransform,
				},
			})
			continue
		}

		if rect, hasRect := w.rectSprites.get(id); hasRect {
			// Same convention as the circle path: an unset (zero) scale means
			// unscaled, so entities spawned without Scale keep their size.
			sx, sy := scl.X, scl.Y
			if sx <= 0 {
				sx = 1
			}
			if sy <= 0 {
				sy = 1
			}
			geometry := math2d.NewRectFromCorners(
				math2d.Vec2{X: pos.X + rect.Rect.Min.X*sx, Y: pos.Y + rect.Rect.Min.Y*sy},
				math2d.Vec2{X: pos.X + rect.Rect.Max.X*sx, Y: pos.Y + rect.Rect.Max.Y*sy},
			)

			commands = append(commands, render.DrawRect{
				Rect:        geometry,
				Fill:        rect.Color,
				Stroke:      rect.Stroke,
				StrokeWidth: rect.StrokeWidth,
				BaseCommand: render.BaseCommand{
					LayerValue: rect.Layer,
					Transform:  bodyTransform,
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
					Transform:  bodyTransform,
				},
			})
		}
	}

	return commands
}

// sortEntry is one sprite's place in the painter order, resolved before the
// sort rather than inside it. Both fields cost several component lookups to
// work out, and a comparator is asked for them O(n log n) times: recomputing
// them there measured about 50% slower over a thousand sprites.
type sortEntry struct {
	key   float64
	layer int
	id    EntityID
}

// sortSprites returns this world's sprites in painter order: the layer decides
// first, then how deep into the scene each sprite sits, and finally EntityID.
//
// That last tiebreak is not a formality. The depth key is a float64, so two
// sprites landing on the same value is ordinary rather than rare, and without
// a total order the same scene could paint two different ways.
//
// The returned slice is working memory reused between frames; it stays valid
// only until the next call.
func (w *World) sortSprites(totalSprites int) []sortEntry {
	sorted := w.renderSortScratch[:0]
	if cap(sorted) < totalSprites {
		sorted = make([]sortEntry, 0, totalSprites)
	}

	// One axis for the whole frame: whether a game declared one is not a
	// question worth asking per sprite.
	axis := w.depthAxis()

	for _, source := range [][]EntityID{w.circleSprites.ids, w.rectSprites.ids, w.imageSprites.ids, w.commandSprites.ids} {
		for _, id := range source {
			// An attached sprite has no place of its own: it sorts exactly at
			// its parent's origin, which is what keeps a composite object one
			// thing in the painter order. The EntityID tiebreak below then
			// draws attachments after the body they decorate, because they
			// are spawned after it.
			origin := id
			if att, ok := w.attachments.get(id); ok {
				origin = att.Parent
			}

			// A sprite whose entity has no Transform is nowhere and draws
			// nothing. Left in, it would have no depth to compare against and
			// would drag the sprites around it out of order, because a
			// comparator that answers "equal" to everything is not an ordering.
			transform, ok := w.transforms.get(origin)
			if !ok {
				continue
			}
			sorted = append(sorted, sortEntry{
				key:   w.spriteSortOrigin(origin, transform.Position).Dot(axis),
				layer: w.spriteLayer(id),
				id:    id,
			})
		}
	}

	slices.SortFunc(sorted, func(a, b sortEntry) int {
		if a.layer != b.layer {
			return a.layer - b.layer
		}
		if a.key < b.key {
			return -1
		}
		if a.key > b.key {
			return 1
		}
		return int(a.id) - int(b.id)
	})
	w.renderSortScratch = sorted

	return sorted
}

// spriteSortOrigin is the point a sprite sorts by: the base of its own
// footprint, moved by whatever the sprite declared.
//
// The base rather than the centre, and that is the whole reason this exists.
// A tall sprite and a short one standing on the same line have centres at
// different depths and bases at the same one, so sorting by centre puts the
// taller of the two behind the neighbour it is standing beside.
//
// Transform.Scale is deliberately left out: a sprite that pops or breathes
// would otherwise reorder itself mid-animation, and a sprite's footprint is
// where it stands, not how big it is drawing itself this frame.
func (w *World) spriteSortOrigin(id EntityID, position math2d.Vec2) math2d.Vec2 {
	if s, ok := w.circleSprites.get(id); ok {
		return math2d.Vec2{X: position.X, Y: position.Y + s.Radius}.Add(s.SortOffset)
	}
	if s, ok := w.rectSprites.get(id); ok {
		// The rect is anchored at the Transform, so its base is the anchored
		// shape's bottom edge placed at the entity's position.
		return math2d.Vec2{
			X: position.X + (s.Rect.Min.X+s.Rect.Max.X)*0.5,
			Y: position.Y + s.Rect.Max.Y,
		}.Add(s.SortOffset)
	}
	if s, ok := w.imageSprites.get(id); ok {
		return math2d.Vec2{X: position.X, Y: position.Y + s.Size.Y*0.5}.Add(s.SortOffset)
	}
	if s, ok := w.commandSprites.get(id); ok {
		// Raw commands have no footprint the engine can read, so the entity's
		// position is the base -- put it where the prop meets the ground.
		return position.Add(s.SortOffset)
	}

	return position
}

// debugOffset is how far an entity's sprite ends up from where the entity
// actually is, so that a debug mark drawn at the entity can follow it there.
//
// The rule it encodes is "follow where the sprite went, never how it was
// deformed". SpriteTransform is applied About the entity's position, and About
// fixes its pivot, so a decorative squash or stretch moves the sprite nowhere
// and this returns zero -- a collider outline must not squash with the art it
// belongs to, because its whole job is to say what the physics shape really is.
// A transform that places the sprite somewhere else does move it, and then a
// mark left behind at the entity's raw position describes nothing.
func (w *World) debugOffset(id EntityID, position math2d.Vec2) math2d.Vec2 {
	sprite, ok := w.spriteTransforms.get(id)
	if !ok || sprite.M.IsIdentity() {
		return math2d.Vec2{}
	}

	return sprite.M.About(position).Apply(position).Sub(position)
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
	if s, ok := w.commandSprites.get(id); ok {
		return s.Layer
	}
	return 0
}

// spritePlane returns the plane of the first sprite found for the entity.
//
// The camera distorts every shape it draws, which is what a ground sprite
// wants and leaves nothing to do. An upright sprite is the one that has to
// stand back up, by exactly the inverse -- see Projection.UprightCorrection.
func (w *World) spritePlane(id EntityID) SpritePlane {
	if s, ok := w.circleSprites.get(id); ok {
		return s.Plane
	}
	if s, ok := w.rectSprites.get(id); ok {
		return s.Plane
	}
	if s, ok := w.imageSprites.get(id); ok {
		return s.Plane
	}
	return PlaneUpright
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
		if s.ShadowFootprint.X > 0 && s.ShadowFootprint.Y > 0 {
			return s.ShadowFootprint
		}

		return math2d.Vec2{
			X: s.Size.X * 0.5,
			Y: s.Size.Y * 0.5,
		}
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

		position := transform.Position.Add(w.debugOffset(id, transform.Position))

		if collider, isCircle := w.circleColliders.get(id); isCircle {
			commands = append(commands, render.DrawCircle{
				Center:      position,
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
					position.Sub(half),
					position.Add(half),
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

		// The contact point is already the midpoint of the two entities, so the
		// midpoint of their offsets is the consistent way to carry it along.
		offsetA := w.debugOffset(contact.A, transformA.Position)
		offsetB := w.debugOffset(contact.B, transformB.Position)

		point := transformA.Position.Lerp(transformB.Position, 0.5).
			Add(offsetA.Lerp(offsetB, 0.5))
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
