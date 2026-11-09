"use strict";
(() => {
    const video = document.getElementById("video");
    const session = new URLSearchParams(location.search).get("session");
    const endpoint = "s/" + session + "/tick/state";
    let engine = null, started = false, configured = false, ready = false, buffering = true;
    let failure = "", disposed = false, desiredPaused = false, playPending = false;
    function fail(code) { if (!failure) failure = code; }
    function play() {
        if (desiredPaused || playPending || failure || disposed) return;
        playPending = true;
        video.play().catch(() => { if (!desiredPaused && !disposed) fail("play-rejected"); })
            .finally(() => { playPending = false; });
    }
    video.addEventListener("playing", () => { ready = true; buffering = false; });
    video.addEventListener("waiting", () => { buffering = true; });
    video.addEventListener("stalled", () => { buffering = true; });
    video.addEventListener("error", () => fail("media-error"));
    video.addEventListener("ended", () => fail("ended"));
    document.addEventListener("contextmenu", event => event.preventDefault());

    async function detect(url) {
        // Some IPTV URLs have no extension. Inspect a bounded prefix, cancel
        // immediately, then let the appropriate demuxer open its own request.
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 10000);
        try {
            const response = await fetch(url, { signal: controller.signal });
            if (!response.ok) throw new Error("probe");
            const reader = response.body.getReader();
            const { value } = await reader.read();
            if (!value) throw new Error("empty");
            const text = new TextDecoder().decode(value.subarray(0, 128)).trimStart();
            if (text.startsWith("#EXTM3U")) return "hls";
            if (text.startsWith("FLV")) return "flv";
            if (value[0] === 0x47) return "mpegts";
            if (response.headers.get("content-type")?.startsWith("video/") ||
                text.substring(4, 8) === "ftyp") return "native";
            throw new Error("unknown format");
        } finally { clearTimeout(timeout); controller.abort(); }
    }
    async function start(command) {
        started = true;
        const kind = command.kind === "auto" ? await detect(command.source) : command.kind;
        if (disposed) return;
        if (kind === "hls") {
            if (Hls.isSupported()) {
                engine = new Hls({ enableWorker: true, maxBufferLength: 30, backBufferLength: 30 });
                engine.on(Hls.Events.ERROR, (_, data) => { if (data.fatal) fail("hls-fatal"); });
                engine.loadSource(command.source);
                engine.attachMedia(video);
            } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
                video.src = command.source;
            } else { fail("hls-unsupported"); return; }
        } else if (kind === "flv" || kind === "mpegts") {
            if (!mpegts.getFeatureList().mseLivePlayback) { fail("mse-unsupported"); return; }
            engine = mpegts.createPlayer({ type: kind, isLive: true, url: command.source },
                { enableWorker: true, autoCleanupSourceBuffer: true, lazyLoad: false });
            engine.on(mpegts.Events.ERROR, () => fail("mpegts-fatal"));
            engine.attachMediaElement(video);
            engine.load();
        } else { video.src = command.source; }
        configured = true;
        play();
    }
    async function tick() {
        if (disposed) return;
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 5000);
        try {
            const response = await fetch(endpoint, {
                method: "POST", headers: { "Content-Type": "application/json" },
                signal: controller.signal,
                body: JSON.stringify({ ready, position: Number.isFinite(video.currentTime) ? video.currentTime : 0,
                    paused: video.paused, buffering, failure })
            });
            if (!response.ok) { dispose(); return; }
            const command = await response.json();
            if (failure) { dispose(); return; }
            desiredPaused = command.paused;
            video.volume = command.volume / 100;
            video.muted = command.muted;
            if (!started) start(command).catch(() => fail("start-failed"));
            if (desiredPaused) video.pause();
            else if (configured && video.paused) play();
        } catch (_) {
            // Native startup/stall watchdogs bound failure recovery even when
            // the local control connection itself is unavailable.
        } finally {
            clearTimeout(timeout);
            if (!disposed) setTimeout(tick, 250);
        }
    }
    function dispose() {
        if (disposed) return;
        disposed = true;
        video.pause();
        if (engine) engine.destroy();
        video.removeAttribute("src");
        video.load();
    }
    window.addEventListener("pagehide", dispose);
    tick();
})();
