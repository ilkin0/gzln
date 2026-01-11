package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
)

func CORS(next http.Handler) http.Handler {
	allowedOrigins := getAllowedOrigins()
	slog.Info("CORS middleware initialized",
		slog.Any("allowed_origins", allowedOrigins),
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		isAllowed := slices.Contains(allowedOrigins, origin)

		if !isAllowed && origin != "" {
			slog.Warn("CORS origin not allowed",
				slog.String("origin", origin),
				slog.Any("allowed", allowedOrigins),
			)
		}

		if isAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		w.Header().Set("Access-Control-Allow-Credentials", "true")

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getAllowedOrigins() []string {
	defaults := []string{
		"http://localhost:5173",
		"http://localhost:4173",
		"http://localhost:3000",
	}

	env := os.Getenv("CORS_ALLOWED_ORIGINS")
	if env == "" {
		return defaults
	}

	origins := strings.Split(env, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return append(defaults, origins...)
}
