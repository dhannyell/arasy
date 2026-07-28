package raster

import (
	"image"
	"log/slog"

	"github.com/dhannyell/arasy/internal/diagnostic"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func (r *Renderer) drawTriangles(img *image.RGBA, command render.DrawTriangles) {
	if err := render.ValidateTriangles(command); err != nil {
		r.diagnostics.Log(r.log, slog.LevelWarn,
			diagnostic.Key{Code: "raster-invalid-triangles"},
			"raster: invalid DrawTriangles command skipped", "error", err)
		return
	}
	if command.Color.A <= 0 {
		return
	}
	for index := 0; index < len(command.Indices); index += 3 {
		points := []math2d.Vec2{
			r.Camera.WorldToScreen(command.Transform.Apply(command.Vertices[command.Indices[index]])),
			r.Camera.WorldToScreen(command.Transform.Apply(command.Vertices[command.Indices[index+1]])),
			r.Camera.WorldToScreen(command.Transform.Apply(command.Vertices[command.Indices[index+2]])),
		}
		fillPolygon(img, points, r.clipBounds(img, polygonBounds(points), command.Clip), toRGBA(command.Color))
	}
}
