package tess

import "github.com/dhannyell/arasy/render"

// AppendTriangles appends valid indexed triangles to the solid stream.
func AppendTriangles(mesh *Mesh, context ctx, command render.DrawTriangles) {
	if render.ValidateTriangles(command) != nil || command.Color.A <= 0 {
		return
	}
	start := len(mesh.Vertices)
	for index := 0; index < len(command.Indices); index += 3 {
		a := context.cam.WorldToScreen(command.Transform.Apply(command.Vertices[command.Indices[index]]))
		b := context.cam.WorldToScreen(command.Transform.Apply(command.Vertices[command.Indices[index+1]]))
		c := context.cam.WorldToScreen(command.Transform.Apply(command.Vertices[command.Indices[index+2]]))
		pushTri(mesh, screenToClip(context, a), screenToClip(context, b), screenToClip(context, c), command.Color)
	}
	if command.Clip.Width() > 0 && command.Clip.Height() > 0 {
		clipped := clipTriangles(mesh.Vertices[start:], clipRect(context, command.Clip))
		mesh.Vertices = append(mesh.Vertices[:start], clipped...)
	}
}
