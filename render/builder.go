package render

import (
	"errors"
	"fmt"
	"math"

	"github.com/dhannyell/arasy/math2d"
)

// Group describes metadata and opacity composed into nested streaming commands.
// A zero Group is the identity.
type Group struct {
	Transform   math2d.Affine
	Opacity     float64
	Clip        math2d.Rect
	LayerOffset int
}

// BuildErrorKind identifies a command-composition failure.
type BuildErrorKind uint8

const (
	ErrInvalidGroup BuildErrorKind = iota + 1
	ErrStackUnderflow
	ErrUnclosedGroup
	ErrUnsupportedGroupedCommand
)

// BuildError reports an invalid Builder operation.
type BuildError struct {
	Kind    BuildErrorKind
	Command Command
}

func (e *BuildError) Error() string {
	switch e.Kind {
	case ErrInvalidGroup:
		return "render: group contains a non-finite value"
	case ErrStackUnderflow:
		return "render: pop without a matching push"
	case ErrUnclosedGroup:
		return "render: commands requested with unclosed groups"
	case ErrUnsupportedGroupedCommand:
		return fmt.Sprintf("render: %T cannot be composed under a modified group", e.Command)
	default:
		return "render: command builder failed"
	}
}

// Is permits errors.Is checks against BuildError values by Kind.
func (e *BuildError) Is(target error) bool {
	var other *BuildError
	return errors.As(target, &other) && e.Kind == other.Kind
}

type groupState struct {
	transform   math2d.Affine
	opacity     float64
	clip        math2d.Rect
	layerOffset int
	modified    bool
}

// Builder composes nested groups into streaming render commands.
type Builder struct {
	commands []Command
	stack    []groupState
	err      error
}

// Reset reuses dst as the output prefix and clears builder state.
func (b *Builder) Reset(dst []Command) {
	b.commands = dst
	b.stack = b.stack[:0]
	b.err = nil
}

// Push opens a nested group.
func (b *Builder) Push(group Group) {
	if b.err != nil {
		return
	}
	if !finiteGroup(group) {
		b.err = &BuildError{Kind: ErrInvalidGroup}
		return
	}
	local := normalizeGroup(group)
	if len(b.stack) == 0 {
		b.stack = append(b.stack, local)
		return
	}
	parent := b.stack[len(b.stack)-1]
	state := groupState{
		transform:   parent.transform.Mul(local.transform),
		opacity:     parent.opacity * local.opacity,
		clip:        intersectOptionalRects(parent.clip, local.clip),
		layerOffset: parent.layerOffset + local.layerOffset,
		modified:    parent.modified || local.modified,
	}
	b.stack = append(b.stack, state)
}

// Add appends command after applying the current effective group.
func (b *Builder) Add(command Command) {
	if b.err != nil || command == nil {
		return
	}
	if len(b.stack) == 0 {
		b.commands = append(b.commands, command)
		return
	}
	state := b.stack[len(b.stack)-1]
	composed, keep, err := composeCommand(command, state)
	if err != nil {
		b.err = err
		return
	}
	if keep {
		b.commands = append(b.commands, composed)
	}
}

// Pop closes the innermost group.
func (b *Builder) Pop() {
	if b.err != nil {
		return
	}
	if len(b.stack) == 0 {
		b.err = &BuildError{Kind: ErrStackUnderflow}
		return
	}
	b.stack = b.stack[:len(b.stack)-1]
}

// Commands returns the composed command slice.
func (b *Builder) Commands() []Command {
	if b.err == nil && len(b.stack) != 0 {
		b.err = &BuildError{Kind: ErrUnclosedGroup}
	}
	return b.commands
}

// Err returns the first builder error.
func (b *Builder) Err() error {
	return b.err
}

func normalizeGroup(group Group) groupState {
	transform := group.Transform
	if transform.IsIdentity() {
		transform = math2d.Identity()
	}
	return groupState{
		transform: transform, opacity: group.Opacity, clip: group.Clip,
		layerOffset: group.LayerOffset,
		modified: !transform.IsIdentity() || group.Opacity != 1 ||
			group.Clip != (math2d.Rect{}) || group.LayerOffset != 0,
	}
}

func finiteGroup(group Group) bool {
	values := [...]float64{
		group.Transform.A, group.Transform.B, group.Transform.C,
		group.Transform.D, group.Transform.E, group.Transform.F,
		group.Opacity,
		group.Clip.Min.X, group.Clip.Min.Y, group.Clip.Max.X, group.Clip.Max.Y,
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

func intersectOptionalRects(a, b math2d.Rect) math2d.Rect {
	if a == (math2d.Rect{}) {
		return b
	}
	if b == (math2d.Rect{}) {
		return a
	}
	return math2d.Rect{
		Min: math2d.Vec2{X: math.Max(a.Min.X, b.Min.X), Y: math.Max(a.Min.Y, b.Min.Y)},
		Max: math2d.Vec2{X: math.Min(a.Max.X, b.Max.X), Y: math.Min(a.Max.Y, b.Max.Y)},
	}
}

func emptyRect(rect math2d.Rect) bool {
	return rect != (math2d.Rect{}) && (rect.Width() <= 0 || rect.Height() <= 0)
}

func composeBase(base BaseCommand, state groupState) (BaseCommand, bool) {
	base.LayerValue += state.layerOffset
	transform := base.Transform
	if transform.IsIdentity() {
		transform = math2d.Identity()
	}
	base.Transform = state.transform.Mul(transform)
	base.Clip = intersectOptionalRects(base.Clip, state.clip)
	return base, !emptyRect(base.Clip)
}

func composeCommand(command Command, state groupState) (Command, bool, error) {
	if state.modified {
		switch command.(type) {
		case DrawParticles, DrawSpriteBatch, DrawTextBatch:
			return nil, false, &BuildError{Kind: ErrUnsupportedGroupedCommand, Command: command}
		}
	}

	switch cmd := command.(type) {
	case DrawRect:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Fill = withOpacity(cmd.Fill, state.opacity)
		cmd.Stroke = withOpacity(cmd.Stroke, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawCircle:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Fill = withOpacity(cmd.Fill, state.opacity)
		cmd.Stroke = withOpacity(cmd.Stroke, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawEllipse:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Fill = withOpacity(cmd.Fill, state.opacity)
		cmd.Stroke = withOpacity(cmd.Stroke, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawPoint:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Color = withOpacity(cmd.Color, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawLine:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Color = withOpacity(cmd.Color, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawText:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Color = withOpacity(cmd.Color, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawArc:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Color = withOpacity(cmd.Color, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawPolygon:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Fill = withOpacity(cmd.Fill, state.opacity)
		cmd.Stroke = withOpacity(cmd.Stroke, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawTriangles:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Color = withOpacity(cmd.Color, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawImage:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Tint = imageTintOpacity(cmd.Tint, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawImageQuad:
		cmd.BaseCommand, _ = composeBase(cmd.BaseCommand, state)
		cmd.Tint = imageTintOpacity(cmd.Tint, state.opacity)
		return cmd, !emptyRect(cmd.Clip), nil
	case DrawParticles, DrawSpriteBatch, DrawTextBatch:
		return command, true, nil
	default:
		return nil, false, &BuildError{Kind: ErrUnsupportedGroupedCommand, Command: command}
	}
}

func withOpacity(color Color, opacity float64) Color {
	color.A *= opacity
	return color
}

func imageTintOpacity(color Color, opacity float64) Color {
	if color == (Color{}) {
		return Color{R: 1, G: 1, B: 1, A: opacity}
	}
	color.A *= opacity
	return color
}
