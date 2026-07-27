package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func CorrelationLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Echo back X-Request-ID header to client
		w.Header().Set("X-Request-ID", requestID)

		// Create request-scoped logger enriched with request_id
		reqLogger := log.With().
			Str("request_id", requestID).
			Logger()

		// Inject logger into request context
		ctx := reqLogger.WithContext(r.Context())

		sw := &statusResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(sw, r.WithContext(ctx))

		durationMs := float64(time.Since(start).Microseconds()) / 1000.0

		event := reqLogger.Info()
		if sw.statusCode >= 500 {
			event = reqLogger.Error()
		} else if sw.statusCode >= 400 {
			event = reqLogger.Warn()
		}

		event.
			Str("func", "CorrelationLoggerMiddleware").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status_code", sw.statusCode).
			Float64("duration_ms", durationMs).
			Msg("request completed")
	})
}
