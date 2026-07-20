package render_test

import (
	"strconv"
	"testing"

	"github.com/dhannyell/frameforge/render"
)

type discardSink struct{}

func (discardSink) VisitRect(render.DrawRect)           {}
func (discardSink) VisitCircle(render.DrawCircle)       {}
func (discardSink) VisitPoint(render.DrawPoint)         {}
func (discardSink) VisitLine(render.DrawLine)           {}
func (discardSink) VisitText(render.DrawText)           {}
func (discardSink) VisitArc(render.DrawArc)             {}
func (discardSink) VisitPolygon(render.DrawPolygon)     {}
func (discardSink) VisitImage(render.DrawImage)         {}
func (discardSink) VisitImageQuad(render.DrawImageQuad) {}
func (discardSink) VisitParticles(render.DrawParticles) {}

func BenchmarkDispatchOrderedCommands(b *testing.B) {
	for _, count := range []int{1_000, 32_000} {
		commands := make([]render.Command, count)
		for i := range commands {
			commands[i] = render.DrawImage{BaseCommand: render.BaseCommand{LayerValue: i}}
		}

		b.Run(strconv.Itoa(count), func(b *testing.B) {
			b.Run("Each", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					render.Each(commands, discardSink{})
				}
			})
			b.Run("EachOrdered", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					render.EachOrdered(commands, discardSink{})
				}
			})
		})
	}
}
