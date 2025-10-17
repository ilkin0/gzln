package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/minio/minio-go/v7"
)

type FileHandler struct {
	fileService *service.FileService
	bucketName  string
}

func NewFileHandler(fileService *service.FileService, bucketName string) *FileHandler {
	return &FileHandler{
		fileService: fileService,
		bucketName:  bucketName,
	}
}

func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File to read file", http.StatusBadRequest)
	}
	defer file.Close()

	fileID := uuid.New().String()
	ext := filepath.Ext(header.Filename)
	objectname := fmt.Sprintf("%s%s", fileID, ext)

	ctx := context.Background()
	info, err := h.fileService.GetMinIOClient().PutObject(
		ctx,
		h.bucketName,
		objectname,
		file,
		header.Size,
		minio.PutObjectOptions{
			ContentType: header.Header.Get("Content-Type"),
			UserMetadata: map[string]string{
				"original-filename": header.Filename,
			},
		},
	)
	if err != nil {
		http.Error(w, "Failed to upload file", http.StatusInternalServerError)
		return
	}

	response := types.UploadResponse{
		FileID:      fileID,
		FileName:    header.Filename,
		Size:        info.Size,
		ContentType: header.Header.Get("Content-Type"),
		UploadedAt:  time.Now(),
		URL:         fmt.Sprintf("/api/files/%s", fileID+ext),
	}

	log.Printf("Response %+v", response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	return r.RemoteAddr
}

func (h *FileHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	var req types.InitUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	clientIP := getClientIP(r)

	ctx := context.Background()
	response, err := h.fileService.InitFileUpload(ctx, req, clientIP)
	if err != nil {
		log.Printf("Failed to init upload: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Upload initialized: ShareID=%s, FileID=%s", response.ShareID, response.FileID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
