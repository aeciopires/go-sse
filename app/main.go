package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// --- Prometheus Metrics ---

var (
	// httpRequestsCounter counts incoming HTTP requests by path and method.
	httpRequestsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sse_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"path", "method"},
	)

	// sseConnectionsCounter counts active SSE connections.
	sseConnectionsCounter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sse_active_connections",
			Help: "Number of active SSE connections",
		},
		[]string{"endpoint"},
	)

	// sseEventsCounter counts total events sent.
	sseEventsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sse_events_sent_total",
			Help: "Total number of events sent via SSE",
		},
		[]string{"event_type"},
	)

	// sseEventDurationHistogram measures event processing time.
	sseEventDurationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sse_event_duration_seconds",
			Help:    "Duration of SSE event processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsCounter)
	prometheus.MustRegister(sseConnectionsCounter)
	prometheus.MustRegister(sseEventsCounter)
	prometheus.MustRegister(sseEventDurationHistogram)
}

// --- Data Structs ---

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	ID        string                 `json:"id"`
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// ConnectionManager manages active SSE connections
type ConnectionManager struct {
	mu          sync.RWMutex
	connections int
}

var connManager = &ConnectionManager{}

// --- Connection Management ---

func (cm *ConnectionManager) increment() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections++
	sseConnectionsCounter.WithLabelValues("/sse").Set(float64(cm.connections))
}

func (cm *ConnectionManager) decrement() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections--
	if cm.connections < 0 {
		cm.connections = 0
	}
	sseConnectionsCounter.WithLabelValues("/sse").Set(float64(cm.connections))
}

func (cm *ConnectionManager) getCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.connections
}

// --- Main Application ---

func main() {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	mux := http.NewServeMux()

	// Frontend
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		recordHTTPRequest(r.URL.Path, r.Method)
		tmpl.Execute(w, nil)
	})

	// Serve static assets
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Health Check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		recordHTTPRequest(r.URL.Path, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "service": "go-sse"}`))
	})

	// SSE Endpoint with required headers
	mux.HandleFunc("/sse", handleSSE)

	// Metrics
	mux.Handle("/metrics", promhttp.Handler())

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// --- SSE Handler ---

// handleSSE handles Server-Sent Events connections with proper headers and streaming
func handleSSE(w http.ResponseWriter, r *http.Request) {
	recordHTTPRequest("/sse", r.Method)

	// Set SSE required headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Flush the headers to establish connection
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Increment active connections
	connManager.increment()
	defer connManager.decrement()

	// Track event processing time
	start := time.Now()

	// Send welcome message
	welcomeEvent := SSEEvent{
		ID:        "1",
		Event:     "welcome",
		Data:      map[string]interface{}{"message": "Connected to go-sse Server-Sent Events"},
		Timestamp: time.Now(),
	}
	sendSSEEvent(w, flusher, welcomeEvent)

	// Create a ticker for sending periodic updates
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Create a channel for handling client disconnection
	clientGone := make(chan bool)
	go func() {
		<-r.Context().Done()
		clientGone <- true
	}()

	// Counter for event IDs
	eventID := 2
	eventCount := 0

	// Send events periodically
	for {
		select {
		case <-clientGone:
			log.Println("Client disconnected from /sse")
			end := time.Since(start).Seconds()
			sseEventDurationHistogram.WithLabelValues("session").Observe(end)
			return

		case <-ticker.C:
			// Create event data
			event := SSEEvent{
				ID:    fmt.Sprintf("%d", eventID),
				Event: "update",
				Data: map[string]interface{}{
					"message":       fmt.Sprintf("Server update #%d", eventCount+1),
					"timestamp":     time.Now().Format(time.RFC3339),
					"server_time":   time.Now().Unix(),
					"cpu_usage":     25 + (eventCount % 50), // Simulated CPU usage
					"memory_usage":  40 + (eventCount % 40), // Simulated memory usage
					"connections":   connManager.getCount(),
					"request_count": eventID,
				},
				Timestamp: time.Now(),
			}

			start := time.Now()
			if !sendSSEEvent(w, flusher, event) {
				end := time.Since(start).Seconds()
				sseEventDurationHistogram.WithLabelValues("update").Observe(end)
				return
			}
			end := time.Since(start).Seconds()
			sseEventDurationHistogram.WithLabelValues("update").Observe(end)
			sseEventsCounter.WithLabelValues("update").Inc()

			eventID++
			eventCount++
		}
	}
}

// sendSSEEvent formats and sends a single SSE event
func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event SSEEvent) bool {
	// Format the event according to Server-Sent Events specification
	// https://html.spec.whatwg.org/multipage/server-sent-events.html

	// Write event ID
	fmt.Fprintf(w, "id: %s\n", event.ID)

	// Write event type
	fmt.Fprintf(w, "event: %s\n", event.Event)

	// Write data as JSON
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		log.Printf("Error marshaling event data: %v", err)
		return false
	}
	fmt.Fprintf(w, "data: %s\n\n", string(dataJSON))

	// Flush to send data immediately
	flusher.Flush()
	return true
}

// --- Helper Functions ---

// recordHTTPRequest increments the HTTP requests counter for a path and method.
func recordHTTPRequest(path, method string) {
	httpRequestsCounter.WithLabelValues(path, method).Inc()
}
