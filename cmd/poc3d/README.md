## First 3D Test 🎈

### It worked! I managed to draw a few cubes and control the camera from multiple angles.

## Notes

- Technically, 3D rendering was already possible since the engine already had the `DrawTriangles` command; I just hadn't tested it before.


Technically, 3D rendering was already possible since the engine already had the `DrawTriangles` command; I just hadn't tested it before.
- Rewind/replay functionality worked as well.

## Some findings:
- Rendering based on the centroid creates some artifacts; backface culling was necessary.
- I tested with 1 and 512 cubes, and the cost remains linear: 1 cube takes about 1µs, and 512 take about 570µs. On my machine, I could render around 10,000 cubes without a bottleneck.

## Next steps:
- Switch to a per-face draw command; a depth buffer should handle this and also resolve sorting issues.
- Support for compiled glTF meshes.
- Stop rendering objects outside the viewport (frustum culling).
- Add clipping to prevent vertices behind the camera's near plane from triggering face-by-face calculations.