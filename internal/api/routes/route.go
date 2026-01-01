package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/handlers"
	"github.com/ilkin0/gzln/internal/middleware"
	"github.com/ilkin0/gzln/internal/service"
)

func FileRoutes(fileService *service.FileService, chunkService *service.ChunkService, bucketName string) chi.Router {
	r := chi.NewRouter()
	fileHandler := handlers.NewFileHandler(fileService, bucketName)
	chunkHandler := handlers.NewChunkHandler(chunkService, bucketName)

	// File routes
	r.Post("/upload", fileHandler.UploadFile)

	r.With(middleware.UploadInitLimiter()).
		Post("/upload/init", fileHandler.InitUpload)

	r.With(middleware.ChunkUploadLimiter()).
		Post("/{fileID}/chunks", chunkHandler.HandleChunkUpload)

	r.With(middleware.UploadFinalizeLimiter()).
		Post("/{fileID}/finalize", fileHandler.FinalizeFileUpload)

	return r
}

func DownloadRoutes(fileService *service.FileService, chunkService *service.ChunkService, bucketName string) chi.Router {
	r := chi.NewRouter()
	fileHandler := handlers.NewFileHandler(fileService, bucketName)
	chunkHandler := handlers.NewChunkHandler(chunkService, bucketName)

	// Download routes
	r.With(middleware.MetadataLimiter()).
		Get("/{shareID}/metadata", fileHandler.GetFileMetadata)

	r.With(middleware.ChunkDownloadLimiter()).
		Get("/{shareID}/chunks/{chunkIndex}", chunkHandler.DownloadChunk)

	r.With(middleware.DownloadCompleteLimiter()).
		Post("/{shareID}/complete", fileHandler.CompleteDownload)

	return r
}
