package render

import "sort"

// CommandSink receives each render command in painter order.
//
// A backend implements one method per command kind. The concrete command is
// passed by value so sinks cannot mutate the frame while it is being drawn.
type CommandSink interface {
	VisitRect(DrawRect)
	VisitCircle(DrawCircle)
	VisitEllipse(DrawEllipse)
	VisitPoint(DrawPoint)
	VisitLine(DrawLine)
	VisitText(DrawText)
	VisitArc(DrawArc)
	VisitPolygon(DrawPolygon)
	VisitTriangles(DrawTriangles)
	VisitImage(DrawImage)
	VisitImageQuad(DrawImageQuad)
	VisitParticles(DrawParticles)
	VisitSpriteBatch(DrawSpriteBatch)
	VisitTextBatch(DrawTextBatch)
	VisitPostChain(PostChain)
}

// Each stable-sorts commands by layer and dispatches each command exactly
// once. The caller's slice is never reordered.
func Each(commands []Command, sink CommandSink) {
	ordered := commands
	for i := 1; i < len(commands); i++ {
		if commands[i].Layer() < commands[i-1].Layer() {
			// Only copy and stable-sort when an out-of-order layer requires it.
			// Most game frames are emitted in painter order, so avoiding this
			// allocation also keeps the renderer's hot path linear in command
			// count without changing the caller-owned slice.
			ordered = append([]Command(nil), commands...)
			sort.SliceStable(ordered, func(i, j int) bool {
				return ordered[i].Layer() < ordered[j].Layer()
			})
			break
		}
	}

	EachOrdered(ordered, sink)
}

// EachOrdered dispatches commands already known to be in nondecreasing layer
// order. It does not validate or repair ordering. It is for renderers that
// proved the precondition while doing another required frame pass; callers
// must use Each when they have not done so.
func EachOrdered(commands []Command, sink CommandSink) {
	for _, command := range commands {
		switch command := command.(type) {
		case DrawRect:
			sink.VisitRect(command)
		case DrawCircle:
			sink.VisitCircle(command)
		case DrawEllipse:
			sink.VisitEllipse(command)
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
		case DrawTriangles:
			sink.VisitTriangles(command)
		case DrawImage:
			sink.VisitImage(command)
		case DrawImageQuad:
			sink.VisitImageQuad(command)
		case DrawParticles:
			sink.VisitParticles(command)
		case DrawSpriteBatch:
			sink.VisitSpriteBatch(command)
		case DrawTextBatch:
			sink.VisitTextBatch(command)
		case PostChain:
			sink.VisitPostChain(command)
		}
	}
}
