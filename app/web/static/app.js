// go-sse frontend: consumes the /sse stream with the browser EventSource API.
(function () {
    "use strict";

    const MAX_EVENTS = 100;
    const THEME_KEY = "go-sse-theme";

    let eventSource = null;
    let eventCount = 0;

    const $ = (id) => document.getElementById(id);

    // ---------- Theme (dark is the default, see theme.js) ----------

    function currentTheme() {
        return document.documentElement.getAttribute("data-theme") === "light" ? "light" : "dark";
    }

    function applyTheme(theme) {
        document.documentElement.setAttribute("data-theme", theme);
        const next = theme === "dark" ? "light" : "dark";
        const label = "Switch to " + next + " mode";
        const toggle = $("themeToggle");
        toggle.setAttribute("aria-label", label);
        toggle.title = label;
        try {
            window.localStorage.setItem(THEME_KEY, theme);
        } catch (e) {
            // Storage unavailable: the choice lasts until the page is reloaded.
        }
    }

    // ---------- Connection status ----------

    function setStatus(state) {
        const labels = { connected: "Connected", connecting: "Connecting…", disconnected: "Disconnected" };
        const el = $("connectionStatus");
        el.dataset.state = state;
        el.querySelector(".status-text").textContent = labels[state];
        $("connectBtn").disabled = state !== "disconnected";
        $("disconnectBtn").disabled = state === "disconnected";
    }

    // ---------- SSE ----------

    function connect() {
        if (eventSource !== null) {
            return;
        }
        setStatus("connecting");
        eventSource = new EventSource("/sse");

        eventSource.onopen = () => setStatus("connected");

        eventSource.addEventListener("welcome", (e) => handleMessage(e, "welcome"));
        eventSource.addEventListener("update", (e) => handleMessage(e, "update"));
        eventSource.onmessage = (e) => handleMessage(e, "message");

        eventSource.onerror = () => {
            if (eventSource === null) {
                return;
            }
            // CONNECTING: the browser is retrying automatically (after "retry:" ms).
            if (eventSource.readyState === EventSource.CONNECTING) {
                setStatus("connecting");
            } else if (eventSource.readyState === EventSource.CLOSED) {
                disconnect();
            }
        };
    }

    function disconnect() {
        if (eventSource !== null) {
            eventSource.close();
            eventSource = null;
        }
        setStatus("disconnected");
    }

    function handleMessage(event, type) {
        let data;
        try {
            data = JSON.parse(event.data);
        } catch (e) {
            data = { message: event.data };
        }

        if (data.cpu_usage !== undefined) {
            $("cpuUsage").textContent = data.cpu_usage + "%";
        }
        if (data.memory_usage !== undefined) {
            $("memoryUsage").textContent = data.memory_usage + "%";
        }
        if (data.connections !== undefined) {
            $("connections").textContent = String(data.connections);
        }
        if (data.timestamp !== undefined) {
            $("lastUpdate").textContent = new Date(data.timestamp).toLocaleTimeString();
        }

        eventCount++;
        $("eventCount").textContent = String(eventCount);
        appendEvent(event.lastEventId, type, data);
    }

    // Builds the DOM with textContent only (never innerHTML) to avoid XSS.
    function appendEvent(id, type, data) {
        const stream = $("eventStream");
        const placeholder = stream.querySelector(".event-placeholder");
        if (placeholder) {
            placeholder.remove();
        }

        const item = document.createElement("article");
        item.className = "event-item event-" + type;

        const header = document.createElement("div");
        header.className = "event-header";

        const badge = document.createElement("span");
        badge.className = "event-badge";
        badge.textContent = type;

        const meta = document.createElement("span");
        meta.className = "event-meta";
        const time = data.timestamp ? new Date(data.timestamp) : new Date();
        meta.textContent = time.toLocaleTimeString() + " · id " + (id || "-") + " · #" + eventCount;

        const body = document.createElement("pre");
        body.className = "event-data";
        body.textContent = JSON.stringify(data, null, 2);

        header.append(badge, meta);
        item.append(header, body);
        stream.append(item);

        while (stream.children.length > MAX_EVENTS) {
            stream.firstElementChild.remove();
        }
        if ($("autoScroll").checked) {
            stream.scrollTop = stream.scrollHeight;
        }
    }

    function clearEvents() {
        const stream = $("eventStream");
        stream.replaceChildren();
        const placeholder = document.createElement("div");
        placeholder.className = "event-placeholder";
        placeholder.textContent = "Event stream cleared";
        stream.append(placeholder);
        eventCount = 0;
        $("eventCount").textContent = "0";
    }

    // ---------- Navigation highlight ----------

    function trackSections() {
        const links = document.querySelectorAll("[data-section]");
        const sections = document.querySelectorAll("main section[id]");
        if (!("IntersectionObserver" in window)) {
            return;
        }
        const observer = new IntersectionObserver((entries) => {
            entries.forEach((entry) => {
                if (!entry.isIntersecting) {
                    return;
                }
                links.forEach((link) => {
                    link.classList.toggle("active", link.dataset.section === entry.target.id);
                });
            });
        }, { rootMargin: "-30% 0px -60% 0px" });
        sections.forEach((s) => observer.observe(s));
    }

    document.addEventListener("DOMContentLoaded", () => {
        applyTheme(currentTheme());
        setStatus("disconnected");

        $("themeToggle").addEventListener("click", () => {
            applyTheme(currentTheme() === "dark" ? "light" : "dark");
        });
        $("connectBtn").addEventListener("click", connect);
        $("disconnectBtn").addEventListener("click", disconnect);
        $("clearBtn").addEventListener("click", clearEvents);
        window.addEventListener("pagehide", disconnect);

        trackSections();
    });
})();
