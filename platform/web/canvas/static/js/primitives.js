import { colorToCSS, hasAlpha } from "./color.js";

const tintCache = new Map();

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
        // Same missing-texture placeholder as the raster backend.
        ctx.fillStyle = "rgb(255, 0, 255)";
        ctx.fillRect(dest.x, dest.y, dest.width, dest.height);
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
