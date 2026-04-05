package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mattcarp12/mdq/internal/metrics"
)

// responseRecorder wraps http.ResponseWriter to capture the status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.ResponseWriter.WriteHeader(statusCode)
}

// LoggingMiddleware logs the incoming HTTP request and its processing duration.
func (s *Server) LoggingMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Initialize our custom recorder with a default 200 OK status
		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Process the request
		next.ServeHTTP(rec, r)

		duration := time.Since(start)

		// Record Prometheus metrics
		statusStr := fmt.Sprintf("%d", rec.statusCode)
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

		// Structured logging with key-value pairs
		slog.Info("HTTP Request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.statusCode),
			slog.String("duration", duration.String()),
			slog.String("ip", r.RemoteAddr),
		)
	}
}
