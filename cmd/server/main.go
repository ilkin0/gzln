package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ilkin0/gzln/internal/api/routes"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/logger"
	custommiddleware "github.com/ilkin0/gzln/internal/middleware"
	"github.com/ilkin0/gzln/internal/scheduler"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/ilkin0/gzln/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	slog.SetDefault(logger.Init())

	ctx := context.Background()

	slog.Info("starting gzln file sharing service",
		slog.String("version", "1.0.1"),
	)

	// Initialize Database
	db, err := database.NewDatabase(ctx)
	if err != nil {
		slog.Error("failed to initialize database",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}
	runTx := database.NewTxRunner(db.Pool)
	defer db.Pool.Close()

	slog.Info("database initialized successfully")

	// Initialize MinIO client
	minioClient, err := storage.NewMinIOClient()
	if err != nil {
		slog.Error("failed to initialize MinIO",
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	slog.Info("minio client initialized successfully",
		slog.String("bucket", minioClient.BucketName),
	)

	// Initialize services
	fileService := service.NewFileService(db.Queries, runTx, minioClient.Client)
	chunkService := service.NewChunkService(db.Queries, minioClient.Client, minioClient.BucketName)

	cleanupService := service.NewCleanupService(db.Queries, minioClient.Client, minioClient.BucketName)

	// Start scheduler
	sched := scheduler.New(cleanupService, 5*time.Minute)
	sched.Start(ctx)

	// Setup router
	r := chi.NewRouter()

	// CORS middleware
	r.Use(custommiddleware.CORS)

	// Standard middleware
	r.Use(logger.RequestLogger)
	r.Use(logger.RequestID)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Mount routes
	r.Mount("/api/v1/files", routes.FileRoutes(fileService, chunkService, minioClient.BucketName))
	r.Mount("/api/v1/download", routes.DownloadRoutes(fileService, chunkService, minioClient.BucketName))

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("server starting",
		slog.String("port", port),
		slog.String("address", fmt.Sprintf("http://localhost:%s", port)),
	)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server failed",
			slog.String("error", err.Error()),
			slog.String("port", port),
		)
		os.Exit(1)
	}
}
