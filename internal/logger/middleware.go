package logger

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		w.Header().Set("X-Request-ID", requestID)
		ctx := WithRequestID(r.Context(), requestID)

		logger := slog.Default().With(slog.String("request_id", requestID))
		ctx = WithLogger(ctx, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		log := FromContext(r.Context())
		log.Info("incoming HTTP request",
			slog.String("http.method", r.Method),
			slog.String("http.path", r.URL.Path),
			slog.String("http.remote_addr", r.RemoteAddr),
			slog.String("http.user_agent", r.UserAgent()),
		)

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		logLevel := slog.LevelInfo
		if wrapped.status >= http.StatusInternalServerError {
			logLevel = slog.LevelError
		} else if wrapped.status >= http.StatusBadRequest {
			logLevel = slog.LevelWarn
		}

		log.Log(r.Context(), logLevel, "HTTP request completed",
			slog.String("http.method", r.Method),
			slog.String("http.path", r.URL.Path),
			slog.Int("http.status", wrapped.status),
			slog.Int64("http.duration_ms", duration.Milliseconds()),
			slog.Int("http.bytes", wrapped.bytes),
		)
	})
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}
