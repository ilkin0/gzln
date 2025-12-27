package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ilkin0/gzln/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestFileRoutes_EndpointsRegistered(t *testing.T) {
	fileService := service.NewFileService(nil, nil, nil)
	chunkService := service.NewChunkService(nil, nil, "test-bucket")
	router := FileRoutes(fileService, chunkService, "test-bucket")

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "POST /upload endpoint exists",
			method:         "POST",
			path:           "/upload",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST /upload/init endpoint exists",
			method:         "POST",
			path:           "/upload/init",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusNotFound, w.Code, "Route should be registered")
		})
	}
}

func TestFileRoutes_MethodNotAllowed(t *testing.T) {
	fileService := service.NewFileService(nil, nil, nil)
	chunkService := service.NewChunkService(nil, nil, "test-bucket")
	router := FileRoutes(fileService, chunkService, "test-bucket")

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "GET /upload not allowed",
			method: "GET",
			path:   "/upload",
		},
		{
			name:   "PUT /upload not allowed",
			method: "PUT",
			path:   "/upload",
		},
		{
			name:   "DELETE /upload not allowed",
			method: "DELETE",
			path:   "/upload",
		},
		{
			name:   "GET /upload/init not allowed",
			method: "GET",
			path:   "/upload/init",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.True(t,
				w.Code == http.StatusMethodNotAllowed || w.Code == http.StatusNotFound,
				"Expected 404 or 405, got %d", w.Code)
		})
	}
}

func TestFileRoutes_NonExistentPath(t *testing.T) {
	fileService := service.NewFileService(nil, nil, nil)
	chunkService := service.NewChunkService(nil, nil, "test-bucket")
	router := FileRoutes(fileService, chunkService, "test-bucket")

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "Non-existent route should return 404")
}

func TestDownloadRoutes_Creation(t *testing.T) {
	fileService := service.NewFileService(nil, nil, nil)
	chunkService := service.NewChunkService(nil, nil, "test-bucket")

	router := DownloadRoutes(fileService, chunkService, "test-bucket")
	assert.NotNil(t, router, "Download routes should be created successfully")
}
