package render

import "sort"

// CommandSink receives each render command in painter order.
//
// A backend implements one method per command kind. The concrete command is
// passed by value so sinks cannot mutate the frame while it is being drawn.
type CommandSink interface {
	VisitRect(DrawRect)
	VisitCircle(DrawCircle)
	VisitPoint(DrawPoint)
	VisitLine(DrawLine)
	VisitText(DrawText)
	VisitArc(DrawArc)
	VisitPolygon(DrawPolygon)
	VisitImage(DrawImage)
	VisitImageQuad(DrawImageQuad)
	VisitParticles(DrawParticles)
}

// Each stable-sorts commands by layer and dispatches each command exactly
// once. The caller's slice is never reordered.
func Each(commands []Command, sink CommandSink) {
	ordered := append([]Command(nil), commands...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Layer() < ordered[j].Layer()
	})

	for _, command := range ordered {
		switch command := command.(type) {
		case DrawRect:
			sink.VisitRect(command)
		case DrawCircle:
			sink.VisitCircle(command)
		case DrawPoint:
			sink.VisitPoint(command)
		case DrawLine:
			sink.VisitLine(command)
		case DrawText:
			sink.VisitText(command)
		case DrawArc:
			sink.VisitArc(command)
		case DrawPolygon:
			sink.VisitPolygon(command)
		case DrawImage:
			sink.VisitImage(command)
		case DrawImageQuad:
			sink.VisitImageQuad(command)
		case DrawParticles:
			sink.VisitParticles(command)
		}
	}
}
