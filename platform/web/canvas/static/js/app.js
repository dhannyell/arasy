import { connectFrameSocket } from "./socket.js";
import { createCanvasRenderer } from "./renderer.js";

const canvas = document.getElementById("ff-canvas");
const stats = document.getElementById("stats");
const historyStats = document.getElementById("history");
const playPauseButton = document.getElementById("play-pause");
const backButton = document.getElementById("back");
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
};

let connection = null;
let paused = false;
let lastFrame = null;

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

    connection?.send({
        type: "control",
        action: "back",
    });
});

resetButton.addEventListener("click", () => {
    connection?.send({
        type: "control",
        action: "reset",
    });
});

speedInput.addEventListener("input", () => {
    const speed = Number(speedInput.value);
    speedValue.textContent = `${speed}x`;

    connection?.send({
        type: "control",
        action: "setSpeed",
        speed,
    });
});

window.addEventListener("keydown", (event) => {
    if (!inputCodes.has(event.code) || event.repeat) {
        return;
    }

    event.preventDefault();
    pressedCodes.add(event.code);
    sendKeyboardInput(event.code, "down");
});

window.addEventListener("keyup", (event) => {
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
        if (lastFrame) {
            renderer.render(lastFrame);
        }
        stats.textContent = "disconnected, retrying...";
    },

    onError(error) {
        console.error(error);
    },

    onFrame(frame) {
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
        syncControls(frame.state);
        stats.textContent =
            `tick ${frame.tick} | commands ${frame.commands.length} | ` +
            `sim ${frame.state.simulationHz * frame.state.speed}Hz effective | render ${measuredFPS}/${frame.state.renderFPS} FPS`;
    },
});

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

function sendKeyboardInput(code, kind) {
    connection?.send({
        type: "input",
        device: "keyboard",
        code,
        kind,
    });
}

function syncControls(state) {
    if (!state) {
        return;
    }

    paused = state.paused;
    playPauseButton.textContent = paused ? "Play" : "Pause";

    if (Number(speedInput.value) !== state.speed) {
        speedInput.value = String(state.speed);
    }
    speedValue.textContent = `${state.speed}x`;

    syncDebugInputs(Number(state.debug ?? 0));
    historyStats.textContent = `history ${state.historyLen}`;
}
