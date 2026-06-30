import { colorToCSS, hasAlpha } from "./color.js";

export function drawRect(ctx, command) {
    const rect = command.rect;

    if (hasAlpha(command.fill)) {
        ctx.fillStyle = colorToCSS(command.fill);
        ctx.fillRect(rect.x, rect.y, rect.width, rect.height);
    }

    if (hasAlpha(command.stroke) && command.strokeWidth > 0) {
        ctx.strokeStyle = colorToCSS(command.stroke);
        ctx.lineWidth = command.strokeWidth;
        ctx.strokeRect(rect.x, rect.y, rect.width, rect.height);
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
    if (!hasAlpha(command.color) || !command.text) return;

    ctx.fillStyle = colorToCSS(command.color);
    ctx.font = `${command.size}px system-ui, sans-serif`;
    ctx.textAlign    = command.align    ?? "left";
    ctx.textBaseline = command.baseline ?? "alphabetic";
    ctx.fillText(command.text, command.position.x, command.position.y);
    ctx.textAlign    = "left";        // reset para não vazar em outros draws
    ctx.textBaseline = "alphabetic";
}
