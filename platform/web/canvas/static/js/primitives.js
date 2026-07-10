import { colorToCSS, hasAlpha } from "./color.js";
import { homographyFromQuad, applyHomography } from "./homography.js";

const tintCache = new Map();

// Number of grid cells (per axis) used to approximate the projective quad
// mapping in drawImageQuad. Higher = closer to true perspective, at the
// cost of more triangle draws per frame.
const IMAGE_QUAD_GRID_SIZE = 16;

export function drawRect(ctx, command) {
    const rect = command.rect;
    const radius = Math.max(0, Math.min(
        command.cornerRadius ?? 0,
        Math.abs(rect.width) * 0.5,
        Math.abs(rect.height) * 0.5,
    ));

    if (radius > 0) {
        ctx.beginPath();
        roundedRectPath(ctx, rect, radius);
    }

    if (hasAlpha(command.fill)) {
        ctx.fillStyle = colorToCSS(command.fill);
        if (radius > 0) {
            ctx.fill();
        } else {
            ctx.fillRect(rect.x, rect.y, rect.width, rect.height);
        }
    }

    if (hasAlpha(command.stroke) && command.strokeWidth > 0) {
        ctx.strokeStyle = colorToCSS(command.stroke);
        ctx.lineWidth = command.strokeWidth;
        if (radius > 0) {
            ctx.stroke();
        } else {
            ctx.strokeRect(rect.x, rect.y, rect.width, rect.height);
        }
    }
}

export function drawCircle(ctx, command) {
    const center = command.center;

    if (hasAlpha(command.fill)) {
        ctx.fillStyle = colorToCSS(command.fill);
        ctx.beginPath();
        ctx.arc(center.x, center.y, command.radius, 0, Math.PI * 2);
        ctx.fill();
    }

    if (hasAlpha(command.stroke) && command.strokeWidth > 0) {
        ctx.strokeStyle = colorToCSS(command.stroke);
        ctx.lineWidth = command.strokeWidth;
        ctx.beginPath();
        ctx.arc(center.x, center.y, command.radius, 0, Math.PI * 2);
        ctx.stroke();
    }
}

export function drawPoint(ctx, command) {
    if (!hasAlpha(command.color)) {
        return;
    }

    ctx.fillStyle = colorToCSS(command.color);
    ctx.beginPath();
    ctx.arc(command.center.x, command.center.y, command.radius, 0, Math.PI * 2);
    ctx.fill();
}

export function drawLine(ctx, command) {
    if (!hasAlpha(command.color) || command.width <= 0) {
        return;
    }

    ctx.strokeStyle = colorToCSS(command.color);
    ctx.lineWidth = command.width;
    ctx.lineCap = "round";

    ctx.beginPath();
    ctx.moveTo(command.from.x, command.from.y);
    ctx.lineTo(command.to.x, command.to.y);
    ctx.stroke();
}

export function drawArc(ctx, command) {
    if (!hasAlpha(command.color) || command.width <= 0) {
        return;
    }

    ctx.strokeStyle = colorToCSS(command.color);
    ctx.lineWidth = command.width;
    ctx.lineCap = "round";

    ctx.beginPath();
    ctx.arc(
        command.center.x,
        command.center.y,
        command.radius,
        command.startAngle,
        command.endAngle,
    );
    ctx.stroke();
}

export function drawText(ctx, command) {
    if (!hasAlpha(command.color) || !command.text) {
        return;
    }

    ctx.save();
    ctx.translate(command.position.x, command.position.y);
    if (command.rotation) {
        ctx.rotate(command.rotation);
    }

    ctx.fillStyle = colorToCSS(command.color);
    ctx.font = `${command.size}px "IBM Plex Mono", Consolas, monospace`;
    ctx.textAlign = command.align ?? "left";
    ctx.textBaseline = command.baseline ?? "alphabetic";
    ctx.fillText(command.text, 0, 0);
    ctx.restore();
}

export function drawPolygon(ctx, command) {
    if (!command.points || command.points.length < 3) {
        return;
    }

    ctx.beginPath();
    ctx.moveTo(command.points[0].x, command.points[0].y);
    for (let i = 1; i < command.points.length; i++) {
        ctx.lineTo(command.points[i].x, command.points[i].y);
    }

    ctx.closePath();

    if (hasAlpha(command.fill)) {
        ctx.fillStyle = colorToCSS(command.fill);
        ctx.fill();
    }

    if (hasAlpha(command.stroke) && command.strokeWidth > 0) {
        ctx.strokeStyle = colorToCSS(command.stroke);
        ctx.lineWidth = command.strokeWidth;
        ctx.stroke();
    }
}

export function drawImage(ctx, command, registry) {
    const dest = command.rect;
    const bitmap = registry?.get(command.texture ?? 0);

    if (!bitmap) {
        // Same missing-texture placeholder as the raster backend, rotated
        // around the destination center like the raster and wasm canvas2d
        // backends (TestDrawImageMissingTextureRotated is the raster anchor).
        ctx.fillStyle = "rgb(255, 0, 255)";
        if (command.rotation) {
            ctx.save();
            ctx.translate(dest.x + dest.width / 2, dest.y + dest.height / 2);
            ctx.rotate(command.rotation);
            ctx.fillRect(-dest.width / 2, -dest.height / 2, dest.width, dest.height);
            ctx.restore();
        } else {
            ctx.fillRect(dest.x, dest.y, dest.width, dest.height);
        }
        return;
    }

    const src = command.src ?? { x: 0, y: 0, width: bitmap.width, height: bitmap.height };

    ctx.save();
    if (command.tint) {
        // Alpha modulation is native; color modulation uses a cached
        // offscreen multiply (see tintedBitmap below).
        ctx.globalAlpha = command.tint.a ?? 1;
    }
    const source = command.tint ? tintedBitmap(bitmap, command.texture, command.tint) : bitmap;

    if (command.rotation) {
        // Pivot on the destination center, mirroring the raster backend.
        ctx.translate(dest.x + dest.width / 2, dest.y + dest.height / 2);
        ctx.rotate(command.rotation);
        ctx.drawImage(source, src.x, src.y, src.width, src.height,
            -dest.width / 2, -dest.height / 2, dest.width, dest.height);
    } else {
        ctx.drawImage(source, src.x, src.y, src.width, src.height,
            dest.x, dest.y, dest.width, dest.height);
    }
    ctx.restore();
}

// drawImageQuad draws a texture mapped onto an arbitrary convex
// quadrilateral (command.corners, in world coordinates, ordered top-left,
// top-right, bottom-right, bottom-left) with perspective-correct
// interpolation.
//
// Canvas2D only exposes affine transforms (no native projective/perspective
// mapping), so this is a DEVELOPMENT-TOOL APPROXIMATION: the unit square is
// subdivided into an IMAGE_QUAD_GRID_SIZE x IMAGE_QUAD_GRID_SIZE grid, and
// each cell's two triangles are drawn with a local affine transform derived
// from three point correspondences. The raster backend implements the exact
// projective mapping and is the semantic reference for this command.
export function drawImageQuad(ctx, command, camera, textures) {
    const corners = command.corners;
    if (!corners || corners.length !== 4) {
        return;
    }

    // The grid cells below are drawn with an absolute setTransform per
    // triangle, which replaces (rather than composes with) the camera
    // transform already active on ctx. So the quad's corners are converted
    // to screen space up front, and the homography is built directly in
    // that space.
    const t = camera?.transform ?? { a: 1, b: 0, c: 0, d: 1, e: 0, f: 0 };
    const screenCorners = corners.map(([x, y]) => applyAffine(t, x, y));

    const h = homographyFromQuad(screenCorners);
    if (!h || !isConvexQuad(screenCorners)) {
        // Degenerate and self-intersecting commands are skipped in every
        // backend; they must not turn into a missing-texture placeholder.
        return;
    }

    const bitmap = textures?.get(command.texture ?? 0);
    if (!bitmap) {
        // Same missing-texture placeholder as the raster backend, but only
        // after the geometry has passed the command-contract validation.
        drawQuadPlaceholder(ctx, corners);
        return;
    }

    const src = command.src ?? { x: 0, y: 0, width: bitmap.width, height: bitmap.height };
    const source = command.tint ? tintedBitmap(bitmap, command.texture, command.tint) : bitmap;
    const alpha = command.tint ? (command.tint.a ?? 1) : 1;

    const n = IMAGE_QUAD_GRID_SIZE;
    for (let j = 0; j < n; j++) {
        const v0 = j / n;
        const v1 = (j + 1) / n;

        for (let i = 0; i < n; i++) {
            const u0 = i / n;
            const u1 = (i + 1) / n;

            const p00 = applyHomography(h, u0, v0);
            const p10 = applyHomography(h, u1, v0);
            const p11 = applyHomography(h, u1, v1);
            const p01 = applyHomography(h, u0, v1);

            const s00 = { x: src.x + u0 * src.width, y: src.y + v0 * src.height };
            const s10 = { x: src.x + u1 * src.width, y: src.y + v0 * src.height };
            const s11 = { x: src.x + u1 * src.width, y: src.y + v1 * src.height };
            const s01 = { x: src.x + u0 * src.width, y: src.y + v1 * src.height };

            drawTexturedTriangle(ctx, source, alpha, s00, s10, s01, p00, p10, p01);
            drawTexturedTriangle(ctx, source, alpha, s10, s11, s01, p10, p11, p01);
        }
    }
}

// drawTexturedTriangle maps the bitmap onto the destination triangle
// (dstA, dstB, dstC) using the affine transform that carries the source
// triangle (srcA, srcB, srcC), expressed in texture pixel coordinates, onto
// it. The transform is set as the absolute canvas transform, then the
// source triangle (in that same local space) is used as a clip path, so
// drawImage(bitmap, 0, 0) only paints the clipped destination triangle.
function drawTexturedTriangle(ctx, bitmap, alpha, srcA, srcB, srcC, dstA, dstB, dstC) {
    const m = affineFromTriangles(srcA, srcB, srcC, dstA, dstB, dstC);
    if (!m) {
        return; // Degenerate source triangle (shouldn't happen for a grid cell).
    }

    ctx.save();
    ctx.globalAlpha = alpha;
    ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]);
    ctx.beginPath();
    ctx.moveTo(srcA.x, srcA.y);
    ctx.lineTo(srcB.x, srcB.y);
    ctx.lineTo(srcC.x, srcC.y);
    ctx.closePath();
    ctx.clip();
    ctx.drawImage(bitmap, 0, 0);
    ctx.restore();
}

// affineFromTriangles solves the 2D affine transform (a, b, c, d, e, f),
// in the ctx.setTransform argument order, that maps p0, p1, p2 exactly onto
// d0, d1, d2 respectively. Three point correspondences fully determine an
// affine map. Returns null when the source triangle is degenerate (zero
// area).
function affineFromTriangles(p0, p1, p2, d0, d1, d2) {
    const denom = (p1.x - p0.x) * (p2.y - p0.y) - (p2.x - p0.x) * (p1.y - p0.y);
    if (Math.abs(denom) <= 1e-12) {
        return null;
    }

    const a = ((d1.x - d0.x) * (p2.y - p0.y) - (d2.x - d0.x) * (p1.y - p0.y)) / denom;
    const b = ((d1.y - d0.y) * (p2.y - p0.y) - (d2.y - d0.y) * (p1.y - p0.y)) / denom;
    const c = ((d2.x - d0.x) * (p1.x - p0.x) - (d1.x - d0.x) * (p2.x - p0.x)) / denom;
    const d = ((d2.y - d0.y) * (p1.x - p0.x) - (d1.y - d0.y) * (p2.x - p0.x)) / denom;
    const e = d0.x - a * p0.x - c * p0.y;
    const f = d0.y - b * p0.x - d * p0.y;

    return [a, b, c, d, e, f];
}

// applyAffine transforms a world-space point through a camera Transform2D
// (as sent by the server), matching the same (x,y) -> (a*x+c*y+e, b*x+d*y+f)
// convention as ctx.setTransform/CanvasRenderingContext2D.
function applyAffine(t, x, y) {
    return { x: t.a * x + t.c * y + t.e, y: t.b * x + t.d * y + t.f };
}

function isConvexQuad(points) {
    let sign = 0;
    for (let i = 0; i < 4; i++) {
        const a = points[i];
        const b = points[(i + 1) % 4];
        const c = points[(i + 2) % 4];
        const cross = (b.x - a.x) * (c.y - b.y) - (b.y - a.y) * (c.x - b.x);
        if (Math.abs(cross) <= 1e-12) {
            return false;
        }
        if (sign === 0) {
            sign = cross;
        } else if ((cross > 0) !== (sign > 0)) {
            return false;
        }
    }
    return true;
}

// drawQuadPlaceholder draws the same missing-texture / degenerate-quad
// marker as the raster backend, using the quad's raw world-space corners
// (the ambient camera transform on ctx handles world -> screen).
function drawQuadPlaceholder(ctx, corners) {
    ctx.fillStyle = "rgb(255, 0, 255)";
    ctx.beginPath();
    ctx.moveTo(corners[0][0], corners[0][1]);
    for (let i = 1; i < corners.length; i++) {
        ctx.lineTo(corners[i][0], corners[i][1]);
    }
    ctx.closePath();
    ctx.fill();
}

// tintedBitmap multiplies the bitmap's color channels by the tint,
// preserving the original alpha. Cached per (texture, tint rgb).
function tintedBitmap(bitmap, textureID, tint) {
    const r = Math.round((tint.r ?? 0) * 255);
    const g = Math.round((tint.g ?? 0) * 255);
    const b = Math.round((tint.b ?? 0) * 255);
    if (r === 255 && g === 255 && b === 255) {
        return bitmap; // white tint = alpha-only, handled by globalAlpha
    }
    const key = `${textureID}:${r},${g},${b}`;
    let canvas = tintCache.get(key);
    if (!canvas) {
        canvas = document.createElement("canvas");
        canvas.width = bitmap.width;
        canvas.height = bitmap.height;
        const off = canvas.getContext("2d");
        off.drawImage(bitmap, 0, 0);
        off.globalCompositeOperation = "multiply";
        off.fillStyle = `rgb(${r}, ${g}, ${b})`;
        off.fillRect(0, 0, canvas.width, canvas.height);
        off.globalCompositeOperation = "destination-in";
        off.drawImage(bitmap, 0, 0); // restore original alpha
        tintCache.set(key, canvas);
    }
    return canvas;
}

function roundedRectPath(ctx, rect, radius) {
    if (typeof ctx.roundRect === "function") {
        ctx.roundRect(rect.x, rect.y, rect.width, rect.height, radius);
        return;
    }

    const x = rect.x;
    const y = rect.y;
    const w = rect.width;
    const h = rect.height;
    const r = radius;

    ctx.moveTo(x + r, y);
    ctx.lineTo(x + w - r, y);
    ctx.quadraticCurveTo(x + w, y, x + w, y + r);
    ctx.lineTo(x + w, y + h - r);
    ctx.quadraticCurveTo(x + w, y + h, x + w - r, y + h);
    ctx.lineTo(x + r, y + h);
    ctx.quadraticCurveTo(x, y + h, x, y + h - r);
    ctx.lineTo(x, y + r);
    ctx.quadraticCurveTo(x, y, x + r, y);
    ctx.closePath();
}
