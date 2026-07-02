import { connectFrameSocket } from "./socket.js";
import { createCanvasRenderer } from "./renderer.js";

const canvas = document.getElementById("ff-canvas");
const stats = document.getElementById("stats");
const historyStats = document.getElementById("history");
const playPauseButton = document.getElementById("play-pause");
const backButton = document.getElementById("back");
const backSecondsButton = document.getElementById("back-seconds");
const backSecondsInput = document.getElementById("back-seconds-value");
const backTickButton = document.getElementById("back-tick");
const backTickInput = document.getElementById("back-tick-value");
const scrubInput = document.getElementById("scrub");
const scrubValueLabel = document.getElementById("scrub-value");
const stepButton = document.getElementById("step");
const resetButton = document.getElementById("reset");
const speedInput = document.getElementById("speed");
const speedValue = document.getElementById("speed-value");
const debugInputs = [...document.querySelectorAll("[data-debug-layer]")];

const DebugLayers = {
    colliders: 1 << 0,
    contacts: 1 << 1,
    grid: 1 << 2,
    activeGrid: 1 << 3,
    candidateGrid: 1 << 4,
    velocityLine: 1 << 5,
    entityID: 1 << 6,
    renderLayers: 1 << 7,
    inputState: 1 << 8,
    runtimeStats: 1 << 9,

};

let connection = null;
let paused = false;
let lastFrame = null;
let pendingRewindFromTick = null;

let scrubbing = false;
let lastScrubTick = null;
let lastScrubSendTime = 0;
const SCRUB_THROTTLE_MS = 50;

let fpsFrames = 0;
let fpsLastTime = performance.now();
let measuredFPS = 0;

const inputCodes = new Set([
    "KeyW",
    "KeyA",
    "KeyS",
    "KeyD",
    "ArrowUp",
    "ArrowDown",
    "ArrowLeft",
    "ArrowRight",
    "Space"
]);
const pressedCodes = new Set();

const renderer = createCanvasRenderer(canvas, {
    onResize(size) {
        connection?.send({
            type: "resize",
            width: size.width,
            height: size.height,
        });
    },
});

playPauseButton.addEventListener("click", () => {
    paused = !paused;
    playPauseButton.textContent = paused ? "Play" : "Pause";

    connection?.send({
        type: "control",
        action: paused ? "pause" : "play",
    });
});

stepButton.addEventListener("click", () => {
    paused = true;
    playPauseButton.textContent = "Play";

    connection?.send({
        type: "control",
        action: "step",
    });
});

backButton.addEventListener("click", () => {
    paused = true;
    playPauseButton.textContent = "Play";
    markPendingRewind(frameSafeTick(lastFrame?.tick) - 1);

    connection?.send({
        type: "control",
        action: "back",
    });
});

backSecondsButton.addEventListener("click", () => {
    const seconds = Number(backSecondsInput.value);
    if (!Number.isFinite(seconds) || seconds <= 0) {
        return;
    }
    const clampedSeconds = clampBackSeconds(seconds);
    if (clampedSeconds <= 0) {
        return;
    }
    const targetTick = backSecondsTargetTick(clampedSeconds);
    backSecondsInput.value = String(clampedSeconds);

    paused = true;
    playPauseButton.textContent = "Play";
    markPendingRewind(targetTick);

    connection?.send({
        type: "control",
        action: "backSeconds",
        seconds: clampedSeconds,
    });
});

backTickButton.addEventListener("click", () => {
    const requestedTick = Number(backTickInput.value);
    if (!Number.isInteger(requestedTick) || requestedTick < 0) {
        return;
    }
    const tick = clampHistoryTick(requestedTick);
    backTickInput.value = String(tick);

    paused = true;
    playPauseButton.textContent = "Play";
    markPendingRewind(tick);

    connection?.send({
        type: "control",
        action: "backToTick",
        tick,
    });
});

scrubInput.addEventListener("input", () => {
    scrubbing = true;
    const tick = clampHistoryTick(Number(scrubInput.value));
    updateScrubLabel(tick);

    const now = performance.now();
    if (now - lastScrubSendTime < SCRUB_THROTTLE_MS) {
        return;
    }
    requestScrubTick(tick);
});

scrubInput.addEventListener("change", () => {
    const tick = clampHistoryTick(Number(scrubInput.value));
    requestScrubTick(tick);
    scrubbing = false;
});

resetButton.addEventListener("click", () => {
    connection?.send({
        type: "control",
        action: "reset",
    });
});

speedInput.addEventListener("input", () => {
    const speed = Number(speedInput.value);
    speedValue.textContent = `${formatSpeed(speed)}x`;

    connection?.send({
        type: "control",
        action: "setSpeed",
        speed,
    });
});

window.addEventListener("keydown", (event) => {
    if (isEditableTarget(event.target)) {
        return;
    }

    if (!inputCodes.has(event.code) || event.repeat) {
        return;
    }

    event.preventDefault();
    pressedCodes.add(event.code);
    sendKeyboardInput(event.code, "down");
});

window.addEventListener("keyup", (event) => {
    if (isEditableTarget(event.target)) {
        return;
    }

    if (!inputCodes.has(event.code)) {
        return;
    }

    event.preventDefault();
    pressedCodes.delete(event.code);
    sendKeyboardInput(event.code, "up");
});

window.addEventListener("blur", () => {
    for (const code of pressedCodes) {
        sendKeyboardInput(code, "up");
    }
    pressedCodes.clear();
});

connection = connectFrameSocket({
    url: "/ws",

    onOpen() {
        lastFrame = null;
        stats.textContent = "connected";
        connection?.send({
            type: "resize",
            ...renderer.size(),
        });
    },

    onClose() {
        pendingRewindFromTick = null;
        if (lastFrame) {
            renderer.render(lastFrame);
        }
        stats.textContent = "disconnected, retrying...";
    },

    onError(error) {
        console.error(error);
    },

    onFrame(frame) {
        if (isStaleWhileRewinding(frame)) {
            return;
        }

        fpsFrames++;

        const now = performance.now();
        const elapsed = now - fpsLastTime;

        if (elapsed >= 1000) {
            measuredFPS = Math.round((fpsFrames * 1000) / elapsed);
            fpsFrames = 0;
            fpsLastTime = now;
        }

        lastFrame = frame;
        renderer.render(frame);
        syncControls(frame);
        stats.textContent =
            `tick ${frame.tick} | commands ${frame.commands.length} | ` +
            `sim ${formatSpeed(frame.state.simulationHz * frame.state.speed)}Hz effective | render ${measuredFPS}/${frame.state.renderFPS} FPS`;
    },
});

function pointerPayload(e, kind, code) {
    const r = canvas.getBoundingClientRect();
    return {
        type: "input", device: "mouse", kind, code,
        x: (e.clientX - r.left) * canvas.width / r.width,
        y: (e.clientY - r.top)  * canvas.height / r.height,
    };
}

canvas.addEventListener("mousemove", e => connection?.send(pointerPayload(e, "move", "Position")));
canvas.addEventListener("mousedown", e => connection?.send(pointerPayload(e, "down", mouseButtonCode(e.button))));
canvas.addEventListener("mouseup",   e => connection?.send(pointerPayload(e, "up",   mouseButtonCode(e.button))));
canvas.addEventListener("mouseleave", () =>
    connection?.send({ type: "input", device: "mouse", code: "Leave", kind: "leave" }));

function mouseButtonCode(button) {
    return ["Left", "Middle", "Right"][button] ?? "Left";
}

// requestScrubTick sends backToTick, deduped against the last tick actually
// sent purely to cut down on redundant network traffic while dragging —
// the backend's BackToTick is idempotent, so a duplicate send is harmless.
function requestScrubTick(tick) {
    if (tick === lastScrubTick) {
        return;
    }
    lastScrubTick = tick;
    lastScrubSendTime = performance.now();

    paused = true;
    playPauseButton.textContent = "Play";
    markPendingRewind(tick);

    connection?.send({
        type: "control",
        action: "backToTick",
        tick,
    });
}

function updateScrubLabel(tick) {
    scrubValueLabel.textContent = `tick ${tick}`;
}

function markPendingRewind(targetTick) {
    const currentTick = frameSafeTick(lastFrame?.tick);
    const historyLen = Number(lastFrame?.state?.historyLen ?? 0);
    if (historyLen > 0 && targetTick < currentTick) {
        pendingRewindFromTick = currentTick;
    }
}

function backSecondsTargetTick(seconds) {
    const currentTick = frameSafeTick(lastFrame?.tick);
    const simulationHz = Number(lastFrame?.state?.simulationHz ?? 60);
    const historyStart = Number(lastFrame?.state?.historyStartTick ?? 0);
    const ticksBack = Math.ceil(seconds * simulationHz);
    return Math.max(historyStart, currentTick - ticksBack);
}

function clampBackSeconds(seconds) {
    const currentTick = frameSafeTick(lastFrame?.tick);
    const historyStart = Number(lastFrame?.state?.historyStartTick ?? currentTick);
    const simulationHz = Number(lastFrame?.state?.simulationHz ?? 60);
    const maxSeconds = Math.max(0, (currentTick - historyStart) / simulationHz);
    const clamped = Math.min(seconds, maxSeconds);

    return Number(clamped.toFixed(3));
}

function clampHistoryTick(tick) {
    const currentTick = frameSafeTick(lastFrame?.tick);
    const historyStart = Number(lastFrame?.state?.historyStartTick ?? 0);
    const historyLen = Number(lastFrame?.state?.historyLen ?? 0);
    const minTick = historyLen > 0 ? historyStart : currentTick;

    return Math.min(Math.max(tick, minTick), currentTick);
}

function isStaleWhileRewinding(frame) {
    if (pendingRewindFromTick === null) {
        return false;
    }

    const frameTick = frameSafeTick(frame?.tick);
    if (frameTick < pendingRewindFromTick) {
        pendingRewindFromTick = null;
        return false;
    }

    return true;
}

function debugMaskFromInputs() {
    let mask = 0;

    for (const input of debugInputs) {
        if (input.checked) {
            mask |= DebugLayers[input.dataset.debugLayer];
        }
    }

    return mask >>> 0;
}

for (const input of debugInputs) {
    input.addEventListener("change", () => {
        connection?.send({
            type: "control",
            action: "setDebug",
            debug: debugMaskFromInputs(),
        });
    });
}

function syncDebugInputs(mask) {
    for (const input of debugInputs) {
        const bit = DebugLayers[input.dataset.debugLayer];
        input.checked = (mask & bit) !== 0;
    }
}

function isEditableTarget(target) {
    return target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement ||
        target?.isContentEditable;
}

function sendKeyboardInput(code, kind) {
    connection?.send({
        type: "input",
        device: "keyboard",
        code,
        kind,
    });
}

function syncControls(frame) {
    const state = frame?.state;
    if (!state) {
        return;
    }

    paused = state.paused;
    playPauseButton.textContent = paused ? "Play" : "Pause";

    const speed = Number(state.speed ?? 1);
    if (Number(speedInput.value) !== speed) {
        speedInput.value = String(speed);
    }
    speedValue.textContent = `${formatSpeed(speed)}x`;

    syncDebugInputs(Number(state.debug ?? 0));
    const historyStart = Number(state.historyStartTick ?? 0);
    const historyEnd = Number(state.historyEndTick ?? 0);
    historyStats.textContent = state.historyLen > 0
        ? `history ${historyStart}-${historyEnd}`
        : "history 0";

    const currentTick = frameSafeTick(frame.tick);
    backTickInput.max = String(currentTick);
    backTickInput.min = String(historyStart);

    scrubInput.disabled = state.historyLen <= 0;

    // Bounds and value are frozen while the user is dragging: backToTick
    // truncates the recorder's snapshot stack on every call, so the true
    // max shrinks as you drag left. Updating min/max live would make the
    // track resize under the thumb mid-drag. Resync once, on release.
    if (!scrubbing) {
        scrubInput.min = String(historyStart);
        scrubInput.max = String(currentTick);
        scrubInput.value = String(currentTick);
        updateScrubLabel(currentTick);
        lastScrubTick = null;
    }
}

function frameSafeTick(tick) {
    const value = Number(tick ?? 0);
    if (!Number.isFinite(value) || value < 0) {
        return 0;
    }
    return Math.floor(value);
}

function formatSpeed(value) {
    const number = Number(value ?? 0);
    if (!Number.isFinite(number)) {
        return "0";
    }

    return Number(number.toFixed(2)).toString();
}
