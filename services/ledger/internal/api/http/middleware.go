package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestIDMiddleware injects or extracts an X-Request-ID header into the context and response.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs request details and duration using slog.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)
		reqID, _ := r.Context().Value(requestIDKey).(string)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapper.statusCode,
			"duration_ms", duration.Milliseconds(),
			"request_id", reqID,
			"remote_addr", r.RemoteAddr,
		)
	})
}

// RecoveryMiddleware handles panics cleanly by logging and returning a 500 JSON response.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID, _ := r.Context().Value(requestIDKey).(string)
				slog.Error("panic recovered in http handler",
					"error", rec,
					"request_id", reqID,
				)
				writeJSON(w, http.StatusInternalServerError, ErrorResponse{
					Error: ErrorDetail{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "an unexpected error occurred",
					},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
