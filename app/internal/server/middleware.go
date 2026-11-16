package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aeciopires/go-sse/app/internal/metrics"
)

// instrument counts requests and observes their latency. Streaming responses
// (/sse) are counted but not timed, since their duration is the session length
// (tracked by sse_session_duration_seconds).
func (s *Server) instrument(path string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.metrics.HTTPRequests.WithLabelValues(path, r.Method).Inc()
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if path == "/sse" {
			return
		}
		code := strconv.Itoa(rec.statusCode())
		metrics.Observe(r.Context(), s.metrics.HTTPDuration.WithLabelValues(path, r.Method, code), time.Since(start))
	})
}

// statusRecorder captures the response status code. It implements Unwrap so
// http.ResponseController can reach the underlying writer (Flush, deadlines).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) statusCode() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

// securityHeaders sets conservative browser security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"style-src 'self' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data:; connect-src 'self'; "+
				"object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
