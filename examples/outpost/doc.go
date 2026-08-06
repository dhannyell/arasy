// Package outpost is the engine's isometric showcase: a floating island in
// the style of tactics games, drawn entirely with vector commands.
//
// Everything that makes the scene read as 2.5D is engine vocabulary from
// docs/en/2-5d.md, used together:
//
//   - Camera.View is the whole projection. The world is a plain square grid
//     the simulation reasons about; one affine on the camera turns it into
//     diamonds on screen. Nothing in this package draws a diamond on purpose.
//   - Volume is extrusion, not perspective. A raised block is
//     Projection.ExtrudeRect; a lone face is Projection.Wall; both sweep along
//     the same LiftVector the engine derives for height, so authored volume
//     and lifted entities can never disagree about which way up is.
//   - Height is scene.Height. A unit ordered to another tile hops there in a
//     ballistic arc, and the shadow that detaches under it is the engine's
//     GroundProjection at work, not hand-drawn. Standing still it keeps a
//     contact shadow from the same projection: scene.ShadowStyle opts in.
//   - Depth order is one layer and base-sort. Compact sprites (units, trees)
//     are entities; volumetric props (mountains, buildings) are
//     scene.CommandSprite entities, so their raw commands sort by the tile
//     they stand on like everything else.
//   - Composites are scene.Attachment. A tree is a trunk with two canopies
//     hung up the lift axis; a unit carries its chassis plate and highlight
//     the same way. Each composite is one thing in the painter order.
//   - Picking is ScreenToWorld. A click lands on the square grid because the
//     camera's affine is invertible, so tile math never sees the projection.
//   - Light is declared, render-only. A scene.AmbientLight dims the actors
//     into dusk and one scene.PointLight on the plaza lantern warms them back
//     near it; World.SetLitLayers keeps marks, hover and clouds out of the
//     band. None of it enters a checksum: authored color is state, light
//     never is.
//
// Click a unit to select it, click a grass tile to send it hopping there.
package outpost
