package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
	"github.com/ilkin0/gzln/internal/logger"
)

type RateLimitConfig struct {
	UploadInitLimit       int
	ChunkUploadLimit      int
	UploadFinalizeLimit   int
	MetadataLimit         int
	ChunkDownloadLimit    int
	DownloadCompleteLimit int
	TimeWindow            time.Duration
}

func LoadRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		UploadInitLimit:       getEnvInt("RATE_LIMIT_UPLOAD_INIT", 10),
		ChunkUploadLimit:      getEnvInt("RATE_LIMIT_CHUNK_UPLOAD", 60),
		UploadFinalizeLimit:   getEnvInt("RATE_LIMIT_UPLOAD_FINALIZE", 20),
		MetadataLimit:         getEnvInt("RATE_LIMIT_METADATA", 30),
		ChunkDownloadLimit:    getEnvInt("RATE_LIMIT_CHUNK_DOWNLOAD", 110),
		DownloadCompleteLimit: getEnvInt("RATE_LIMIT_DOWNLOAD_COMPLETE", 20),
		TimeWindow: time.
			Duration(getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60)) * time.Second,
	}
}

func getEnvInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

var config = LoadRateLimitConfig()

func UploadInitLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		config.UploadInitLimit,
		config.TimeWindow,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(rateLimitExceededHandler(config.TimeWindow)),
	)
}

func ChunkUploadLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		config.ChunkUploadLimit,
		config.TimeWindow,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(rateLimitExceededHandler(config.TimeWindow)),
	)
}

func UploadFinalizeLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		config.UploadFinalizeLimit,
		config.TimeWindow,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(rateLimitExceededHandler(config.TimeWindow)),
	)
}

func MetadataLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		config.MetadataLimit,
		config.TimeWindow,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(rateLimitExceededHandler(config.TimeWindow)),
	)
}

func ChunkDownloadLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		config.ChunkDownloadLimit,
		config.TimeWindow,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(rateLimitExceededHandler(config.TimeWindow)),
	)
}

func DownloadCompleteLimiter() func(http.Handler) http.Handler {
	return httprate.Limit(
		config.DownloadCompleteLimit,
		config.TimeWindow,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(rateLimitExceededHandler(config.TimeWindow)),
	)
}

func rateLimitExceededHandler(retryAfter time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.FromContext(r.Context())
		log.Warn("rate limit exceeded",
			slog.String("ip", r.RemoteAddr),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("user_agent", r.UserAgent()),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Retry-After", retryAfter.String())
		w.Write([]byte(`{"success":false,"message":"Rate limit exceeded. Please
  try again later."}`))
	}
}
