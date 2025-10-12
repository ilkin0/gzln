package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/handlers"
	"github.com/minio/minio-go/v7"
)

func FileRoutes(minioClient *minio.Client, bucketName string) chi.Router {
	r := chi.NewRouter()
	fileHandler := handlers.NewFileHandler(minioClient, bucketName)

	// File routes
	r.Post("/upload", fileHandler.UploadFile)
	r.Get("/{fileID}", fileHandler.DownloadFile)
	r.Get("/{fileID}/info", fileHandler.GetFileInfo)
	return r
}
