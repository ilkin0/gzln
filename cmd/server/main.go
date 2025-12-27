package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ilkin0/gzln/internal/api/routes"
	"github.com/ilkin0/gzln/internal/database"
	custommiddleware "github.com/ilkin0/gzln/internal/middleware"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/ilkin0/gzln/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or unable to load it")
	}

	ctx := context.Background()

	// Initialize Database
	db, err := database.NewDatabase(ctx)
	if err != nil {
		log.Fatalf("Failedt initialize database: %v", err)
	}
	runTx := database.NewTxRunner(db.Pool)
	defer db.Pool.Close()

	// Initialize MinIO client
	minioClient, err := storage.NewMinIOClient()
	if err != nil {
		log.Fatalf("Failed to initialize MinIO: %v", err)
	}

	// Initialize FileService
	fileService := service.NewFileService(db.Queries, runTx, minioClient.Client)
	chunkService := service.NewChunkService(db.Queries, minioClient.Client, minioClient.BucketName)

	// Setup router
	r := chi.NewRouter()

	// CORS middleware
	r.Use(custommiddleware.CORS)

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Mount routes
	r.Mount("/api/v1/files", routes.FileRoutes(fileService, chunkService, minioClient.BucketName))
	r.Mount("/api/v1/download", routes.DownloadRoutes(fileService, chunkService, minioClient.BucketName))

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
