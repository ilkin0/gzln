package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/handlers"
	"github.com/ilkin0/gzln/internal/service"
)

func FileRoutes(fileService *service.FileService, bucketName string) chi.Router {
	r := chi.NewRouter()
	fileHandler := handlers.NewFileHandler(fileService, bucketName)

	// File routes
	r.Post("/upload", fileHandler.UploadFile)
	r.Post("/upload/init", fileHandler.InitUpload)
	return r
}
