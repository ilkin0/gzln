package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/handlers"
)

func FileRoutes() chi.Router {
	r := chi.NewRouter()
	fileHandler := &handlers.FileHandler{}

	r.Get("/upload", fileHandler.UploadFile)

	return r
}
