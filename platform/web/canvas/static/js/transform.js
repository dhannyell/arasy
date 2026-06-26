export function applyCamera(ctx, camera) {
    const t = camera.transform;
    ctx.setTransform(t.a, t.b, t.c, t.d, t.e, t.f);
}

export function resetTransform(ctx) {
    ctx.setTransform(1, 0, 0, 1, 0, 0);
}