// AudioEngine.js

/**
 * Creates and initializes an isolated instances of the audio engine.
 * @returns {Object} Control methods: loadManifest, playCues, setTracks
 */
export function createAudioEngine() {
    // Private scope and state for this instance
    const AudioContextClass = window.AudioContext || window["webkitAudioContext"];
    const audioCtx = new AudioContextClass();
    const manifestBuffers = new Map(); // id -> AudioBuffer
    const activeTracks = new Map();     // trackId -> { source, gainNode, clip }
    let desiredTracks = {};
    let unlocked = false;

    // UNLOCK: Automatic and isolated handling of browser autoplay policies
    const initializeUnlock = () => {
        const unlock = () => {
            unlocked = true;
            if (!document.hidden && audioCtx.state === 'suspended') {
                audioCtx.resume().catch(err => console.warn("Failed to resume AudioContext:", err));
            }
        };
        document.addEventListener('pointerdown', unlock, { once: true });
        document.addEventListener('keydown', unlock, { once: true });
    };
    initializeUnlock();

    document.addEventListener("visibilitychange", () => {
        if (document.hidden) {
            if (audioCtx.state === "running") {
                audioCtx.suspend().catch(err => console.warn("Failed to suspend AudioContext:", err));
            }
            return;
        }
        if (unlocked && audioCtx.state === "suspended") {
            audioCtx.resume().catch(err => console.warn("Failed to resume AudioContext:", err));
        }
    });

    // Internal utility function to apply linear fade-out and stop a track source node
    const fadeAndStop = (activeTrack, startTime, fadeDuration) => {
        const gainParam = activeTrack.gainNode.gain;
        gainParam.setValueAtTime(gainParam.value, startTime);
        gainParam.linearRampToValueAtTime(0, startTime + fadeDuration);
        activeTrack.source.stop(startTime + fadeDuration);
    };

    // Public API exposed by the engine
    return {
        /**
         * MANIFEST: Fetches and decodes a list of audio clips asynchronously.
         * @param {Array<{id: string, url: string}>} clips
         */
        async loadManifest(clips) {
            const promises = clips.map(async (clip) => {
                try {
                    const response = await fetch(clip.url);
                    if (!response.ok) throw new Error(`HTTP ${response.status}`);

                    const arrayBuffer = await response.arrayBuffer();
                    const audioBuffer = await audioCtx.decodeAudioData(arrayBuffer);

                    manifestBuffers.set(clip.id, audioBuffer);
                } catch (error) {
                    console.warn(`Error loading clip "${clip.id}" from ${clip.url}:`, error);
                }
            });

            await Promise.all(promises);
            reconcileTracks();
        },

        /**
         * CUES: Plays instant sound effects enforcing a cap of 4 voices per batch call.
         * @param {Array<{clip: string, pan: number, gain: number}>} cues
         */
        playCues(cues) {
            // Silently discard cues if the context is not running yet (no accumulation)
            if (audioCtx.state !== 'running') {
                return;
            }

            const voiceCounter = new Map();

            for (const cue of cues) {
                const buffer = manifestBuffers.get(cue.clip);
                if (!buffer) continue;

                // Voice limit cap per message call to prevent machine-gun effect
                const currentCount = voiceCounter.get(cue.clip) || 0;
                if (currentCount >= 4) {
                    continue;
                }
                voiceCounter.set(cue.clip, currentCount + 1);

                // Audio node chain creation and connections
                const source = audioCtx.createBufferSource();
                source.buffer = buffer;

                const panner = audioCtx.createStereoPanner();
                panner.pan.value = cue.pan ?? 0;

                const gainNode = audioCtx.createGain();
                gainNode.gain.value = cue.gain ?? 1;

                source.connect(panner);
                panner.connect(gainNode);
                gainNode.connect(audioCtx.destination);

                source.start(0);
            }
        },

        /**
         * TRACKS (Reconcile): Modifies continuous background tracks via state reconciliation.
         * @param {Object<string, {clip: string, gain: number, loop: boolean, fadeSec: number}>} targetTracks
         * @param {number} fadeSec Default duration for fade-in and fade-out transitions (seconds)
         */
        setTracks(targetTracks, fadeSec = 0.5) {
            desiredTracks = targetTracks ?? {};
            reconcileTracks(fadeSec);
        }
    };

    function reconcileTracks(fadeSec = 0.5) {
        const now = audioCtx.currentTime;
        const shortRampSec = 0.05; // Short ramp to prevent audio clicks when updating volume
        const desired = Object.entries(desiredTracks);
        const targetTrackIds = new Set(desired.map(([id]) => id));

        // 1. Remove and fade-out tracks that are not present in the new state array
        for (const [trackId, activeTrack] of activeTracks.entries()) {
            if (!targetTrackIds.has(trackId)) {
                fadeAndStop(activeTrack, now, fadeSec);
                activeTracks.delete(trackId);
            }
        }

        // 2. Add new tracks or update parameters of existing ones
        for (const [trackId, track] of desired) {
            const activeTrack = activeTracks.get(trackId);
            const buffer = manifestBuffers.get(track.clip);

            if (!buffer) continue;
            const trackFadeSec = track.fadeSec ?? fadeSec;

            // Case A: New track OR the assigned audio clip changed completely
            if (!activeTrack || activeTrack.clip !== track.clip) {
                if (activeTrack) {
                    fadeAndStop(activeTrack, now, trackFadeSec);
                }

                const source = audioCtx.createBufferSource();
                source.buffer = buffer;
                source.loop = track.loop ?? false;

                const gainNode = audioCtx.createGain();
                gainNode.gain.setValueAtTime(0, now);
                gainNode.gain.linearRampToValueAtTime(track.gain ?? 1, now + trackFadeSec);

                source.connect(gainNode);
                gainNode.connect(audioCtx.destination);
                source.start(0);

                activeTracks.set(trackId, {
                    source,
                    gainNode,
                    clip: track.clip
                });
            }
            // Case B: Same track and same clip, but gain value changed
            else if (activeTrack.gainNode.gain.value !== track.gain) {
                activeTrack.gainNode.gain.setValueAtTime(activeTrack.gainNode.gain.value, now);
                activeTrack.gainNode.gain.linearRampToValueAtTime(track.gain ?? 1, now + shortRampSec);
            }
        }
    }
}
