package server

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"uuid"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/aeciopires/go-sse/app/internal/metrics"
)

// reconnectDelay is sent to the browser in the "retry:" field: how long the
// EventSource waits before reconnecting after the connection drops.
const reconnectDelay = 3 * time.Second

// Event is a single Server-Sent Event.
// See https://html.spec.whatwg.org/multipage/server-sent-events.html
type Event struct {
	ID    uint64
	Name  string
	Data  any           // encoded as JSON in the "data:" field
	Retry time.Duration // optional "retry:" field (0 = omitted)
}

type welcomePayload struct {
	Message    string    `json:"message"`
	ClientID   string    `json:"client_id"`
	IntervalMS int64     `json:"interval_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

type updatePayload struct {
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	ServerTime  int64     `json:"server_time"`
	CPUUsage    int       `json:"cpu_usage"`
	MemoryUsage int       `json:"memory_usage"`
	Connections int64     `json:"connections"`
	EventID     uint64    `json:"event_id"`
	ClientID    string    `json:"client_id"`
}

// writeEvent serializes e in the text/event-stream format with a single Write.
func writeEvent(w io.Writer, e Event) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return fmt.Errorf("encode %q event: %w", e.Name, err)
	}
	var buf []byte
	if e.Retry > 0 {
		buf = fmt.Appendf(buf, "retry: %d\n", e.Retry.Milliseconds())
	}
	buf = fmt.Appendf(buf, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Name, data)
	_, err = w.Write(buf)
	return err
}

// parseLastEventID reads the Last-Event-ID header sent by browsers when they
// reconnect, so event IDs keep increasing across reconnections.
func parseLastEventID(v string) uint64 {
	id, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// handleSSE streams a "welcome" event followed by an "update" event every
// EventInterval until the client disconnects or the server shuts down.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable proxy buffering (ingress-nginx)

	// The stream outlives the server-wide WriteTimeout: lift it for this connection.
	if err := rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		s.log.WarnContext(r.Context(), "clear write deadline", "error", err)
	}
	// Flushing sends the status line and headers, establishing the stream.
	if err := rc.Flush(); err != nil {
		s.log.ErrorContext(r.Context(), "streaming not supported", "error", err)
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// End the stream when the client goes away OR the server shuts down.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer context.AfterFunc(s.streamsCtx, cancel)()

	clientID := uuid.NewV7().String()
	log := s.log.With("client_id", clientID)
	start := time.Now()
	s.active.Add(1)
	gauge := s.metrics.ActiveConnections.WithLabelValues("/sse")
	gauge.Inc()
	defer func() {
		s.active.Add(-1)
		gauge.Dec()
		s.metrics.SessionDuration.Observe(time.Since(start).Seconds())
	}()

	id := parseLastEventID(r.Header.Get("Last-Event-ID"))
	log.InfoContext(ctx, "sse client connected",
		"remote_addr", r.RemoteAddr, "user_agent", r.UserAgent(), "last_event_id", id)

	id++
	err := s.sendEvent(ctx, rc, w, clientID, Event{
		ID:    id,
		Name:  "welcome",
		Retry: reconnectDelay,
		Data: welcomePayload{
			Message:    "Connected to go-sse Server-Sent Events",
			ClientID:   clientID,
			IntervalMS: s.cfg.EventInterval.Milliseconds(),
			Timestamp:  time.Now().UTC(),
		},
	})
	if err != nil {
		log.WarnContext(ctx, "sse write failed", "error", err)
		return
	}

	tick := time.Tick(s.cfg.EventInterval)
	for seq := 1; ; seq++ {
		select {
		case <-ctx.Done():
			reason := "client disconnected"
			if s.streamsCtx.Err() != nil {
				reason = "server shutting down"
			}
			log.InfoContext(ctx, "sse client disconnected", "reason", reason,
				"updates_sent", seq-1, "duration_seconds", time.Since(start).Seconds())
			return
		case now := <-tick:
			id++
			err := s.sendEvent(ctx, rc, w, clientID, Event{
				ID:   id,
				Name: "update",
				Data: updatePayload{
					Message:     fmt.Sprintf("Server update #%d", seq),
					Timestamp:   now.UTC(),
					ServerTime:  now.Unix(),
					CPUUsage:    25 + seq%50, // simulated
					MemoryUsage: 40 + seq%40, // simulated
					Connections: s.active.Load(),
					EventID:     id,
					ClientID:    clientID,
				},
			})
			if err != nil {
				log.WarnContext(ctx, "sse write failed", "error", err, "updates_sent", seq-1)
				return
			}
		}
	}
}

// sendEvent writes and flushes one event inside its own trace. Each event is
// a new root span linked to the long-lived connection span, so traces stay
// small and are exported immediately instead of when the stream ends.
func (s *Server) sendEvent(ctx context.Context, rc *http.ResponseController, w io.Writer, clientID string, e Event) error {
	ctx, span := s.tracer.Start(ctx, "sse.send "+e.Name,
		trace.WithNewRoot(),
		trace.WithLinks(trace.LinkFromContext(ctx)),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("sse.event.type", e.Name),
			attribute.String("sse.event.id", strconv.FormatUint(e.ID, 10)),
			attribute.String("sse.client.id", clientID),
		),
	)
	defer span.End()

	start := time.Now()
	err := writeEvent(w, e)
	if err == nil {
		err = rc.Flush()
	}
	metrics.Observe(ctx, s.metrics.EventDuration.WithLabelValues(e.Name), time.Since(start))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "send event")
		return err
	}
	s.metrics.EventsSent.WithLabelValues(e.Name).Inc()
	s.log.DebugContext(ctx, "sse event sent", "client_id", clientID, "event", e.Name, "event_id", e.ID)
	return nil
}
