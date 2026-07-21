package render_test

import (
	"strconv"
	"testing"

	"github.com/dhannyell/frameforge/render"
)

type discardSink struct{}

func (discardSink) VisitRect(render.DrawRect)               {}
func (discardSink) VisitCircle(render.DrawCircle)           {}
func (discardSink) VisitEllipse(render.DrawEllipse)         {}
func (discardSink) VisitPoint(render.DrawPoint)             {}
func (discardSink) VisitLine(render.DrawLine)               {}
func (discardSink) VisitText(render.DrawText)               {}
func (discardSink) VisitArc(render.DrawArc)                 {}
func (discardSink) VisitPolygon(render.DrawPolygon)         {}
func (discardSink) VisitImage(render.DrawImage)             {}
func (discardSink) VisitImageQuad(render.DrawImageQuad)     {}
func (discardSink) VisitParticles(render.DrawParticles)     {}
func (discardSink) VisitSpriteBatch(render.DrawSpriteBatch) {}
func (discardSink) VisitTextBatch(render.DrawTextBatch)     {}

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

// BenchmarkDispatchOrderedMixedCommands dispatches an ordered frame mixing
// streaming commands with the retained batch commands, matching how games
// interleave both tiers in painter order.
func BenchmarkDispatchOrderedMixedCommands(b *testing.B) {
	for _, count := range []int{1_000, 32_000} {
		commands := make([]render.Command, count)
		for i := range commands {
			base := render.BaseCommand{LayerValue: i}
			switch i % 4 {
			case 0:
				commands[i] = render.DrawImage{BaseCommand: base}
			case 1:
				commands[i] = render.DrawSpriteBatch{BaseCommand: base, Batch: 1}
			case 2:
				commands[i] = render.DrawText{BaseCommand: base}
			default:
				commands[i] = render.DrawTextBatch{BaseCommand: base, Batch: 2}
			}
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
