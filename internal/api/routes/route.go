package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/handlers"
	"github.com/ilkin0/gzln/internal/service"
)

func FileRoutes(fileService *service.FileService, chunkService *service.ChunkService, bucketName string) chi.Router {
	r := chi.NewRouter()
	fileHandler := handlers.NewFileHandler(fileService, bucketName)
	chunkHandler := handlers.NewChunkHandler(chunkService, bucketName)

	// File routes
	r.Post("/upload", fileHandler.UploadFile)
	r.Post("/upload/init", fileHandler.InitUpload)
	r.Post("/{fileId}/chunks", chunkHandler.HandleChunkUpload)
	r.Post("/{fileId}/finalize", fileHandler.FinalizeFileUpload)
	return r
}
