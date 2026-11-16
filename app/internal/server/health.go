package server

import (
	json "encoding/json/v2"
	"math"
	"net/http"
	"time"
)

type healthResponse struct {
	Status            string  `json:"status"`
	Service           string  `json:"service,omitempty"`
	Version           string  `json:"version,omitempty"`
	UptimeSeconds     float64 `json:"uptime_seconds,omitzero"`
	ActiveConnections int64   `json:"active_connections,omitzero"`
}

// handleHealth reports general service health (kept for backwards
// compatibility and for humans; Kubernetes uses /livez and /readyz).
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, healthResponse{
		Status:            "ok",
		Service:           s.cfg.ServiceName,
		Version:           s.cfg.ServiceVersion,
		UptimeSeconds:     math.Round(time.Since(s.started).Seconds()*1000) / 1000,
		ActiveConnections: s.active.Load(),
	})
}

// handleLive is the liveness probe: the process is up and serving HTTP.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, healthResponse{Status: "ok"})
}

// handleReady is the readiness probe: it fails while the server is starting
// or shutting down so no new traffic is routed to this instance.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if !s.ready.Load() {
		s.writeJSON(w, r, http.StatusServiceUnavailable, healthResponse{Status: "not_ready"})
		return
	}
	s.writeJSON(w, r, http.StatusOK, healthResponse{Status: "ready"})
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.MarshalWrite(w, v); err != nil {
		s.log.DebugContext(r.Context(), "write json response", "error", err)
	}
}
