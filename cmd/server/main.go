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
		log.Fatalf("Failed ti initialize database: %v", err)
	}
	defer db.Pool.Close()

	// Initialize MinIO client
	minioClient, err := storage.NewMinIOClient()
	if err != nil {
		log.Fatalf("Failed to initialize MinIO: %v", err)
	}

	// Initialize FileService
	fileService := service.NewFileService(db.Queries, minioClient.Client)

	// Setup router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Mount routes
	r.Mount("/api/files", routes.FileRoutes(fileService, minioClient.BucketName))

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
