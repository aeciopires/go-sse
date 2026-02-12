let eventSource = null;
let eventCount = 0;

/**
 * Connect to SSE endpoint
 */
function connectSSE() {
    if (eventSource !== null) {
        console.log("SSE connection already established");
        return;
    }

    console.log("Connecting to SSE endpoint...");
    eventSource = new EventSource("/sse");

    // Handle open connection
    eventSource.onopen = function() {
        console.log("SSE connection established");
        updateConnectionStatus(true);
        document.getElementById("eventStream").innerHTML = "";
        eventCount = 0;
        document.getElementById("eventCount").textContent = "0";
    };

    // Handle generic events
    eventSource.onmessage = function(event) {
        console.log("Message event received:", event);
        handleSSEMessage(event.data, "message");
    };

    // Handle welcome event
    eventSource.addEventListener("welcome", function(event) {
        console.log("Welcome event received:", event);
        handleSSEMessage(event.data, event.type);
    });

    // Handle update events
    eventSource.addEventListener("update", function(event) {
        console.log("Update event received:", event);
        handleSSEMessage(event.data, event.type);
    });

    // Handle errors
    eventSource.onerror = function(event) {
        console.error("SSE error occurred:", event);
        if (eventSource.readyState === EventSource.CLOSED) {
            console.log("SSE connection closed");
            updateConnectionStatus(false);
            disconnectSSE();
        }
    };
}

/**
 * Disconnect from SSE endpoint
 */
function disconnectSSE() {
    if (eventSource !== null) {
        console.log("Disconnecting from SSE endpoint...");
        eventSource.close();
        eventSource = null;
        updateConnectionStatus(false);
    }
}

/**
 * Handle incoming SSE message
 */
function handleSSEMessage(data, eventType) {
    try {
        let parsedData;
        try {
            parsedData = JSON.parse(data);
        } catch (e) {
            // If data is not JSON, treat it as plain text
            parsedData = { message: data };
        }

        // Update metrics if available
        if (parsedData.cpu_usage !== undefined) {
            document.getElementById("cpuUsage").textContent = parsedData.cpu_usage + "%";
        }
        if (parsedData.memory_usage !== undefined) {
            document.getElementById("memoryUsage").textContent = parsedData.memory_usage + "%";
        }
        if (parsedData.timestamp !== undefined) {
            const date = new Date(parsedData.timestamp);
            document.getElementById("lastUpdate").textContent = date.toLocaleTimeString();
        }

        // Add event to stream
        eventCount++;
        document.getElementById("eventCount").textContent = eventCount;
        addEventToStream(parsedData, eventType);

    } catch (error) {
        console.error("Error handling SSE message:", error);
    }
}

/**
 * Add event to event stream display
 */
function addEventToStream(data, eventType) {
    const eventStream = document.getElementById("eventStream");

    // Clear placeholder if it exists
    const placeholder = eventStream.querySelector(".event-placeholder");
    if (placeholder) {
        placeholder.remove();
    }

    // Create event element
    const eventElement = document.createElement("div");
    eventElement.className = `event-item ${eventType}`;

    // Format timestamp
    let timestamp = new Date().toLocaleTimeString();
    if (data.timestamp) {
        timestamp = new Date(data.timestamp).toLocaleTimeString();
    }

    // Create HTML content
    eventElement.innerHTML = `
        <div class="event-id">[${timestamp}] Event #${eventCount}</div>
        <div class="event-type">Type: ${eventType}</div>
        <div class="event-data"><pre>${JSON.stringify(data, null, 2)}</pre></div>
    `;

    // Add to stream
    eventStream.appendChild(eventElement);

    // Auto-scroll if enabled
    if (document.getElementById("autoScroll").checked) {
        eventStream.scrollTop = eventStream.scrollHeight;
    }

    // Keep only last 100 events to prevent memory issues
    const events = eventStream.querySelectorAll(".event-item");
    if (events.length > 100) {
        events[0].remove();
    }
}

/**
 * Update connection status display
 */
function updateConnectionStatus(connected) {
    const statusElement = document.getElementById("connectionStatus");
    const connectBtn = document.getElementById("connectBtn");
    const disconnectBtn = document.getElementById("disconnectBtn");

    if (connected) {
        statusElement.textContent = "● Connected";
        statusElement.className = "status status-connected";
        connectBtn.disabled = true;
        disconnectBtn.disabled = false;
    } else {
        statusElement.textContent = "● Disconnected";
        statusElement.className = "status status-disconnected";
        connectBtn.disabled = false;
        disconnectBtn.disabled = true;
    }
}

/**
 * Clear event stream
 */
function clearEvents() {
    document.getElementById("eventStream").innerHTML = '<div class="event-placeholder">Event stream cleared</div>';
    eventCount = 0;
    document.getElementById("eventCount").textContent = "0";
}

/**
 * Initialize on page load
 */
document.addEventListener("DOMContentLoaded", function() {
    console.log("Page loaded. SSE testing application ready.");
    updateConnectionStatus(false);

    // Handle page unload to clean up connection
    window.addEventListener("beforeunload", function() {
        if (eventSource !== null) {
            eventSource.close();
        }
    });
});
