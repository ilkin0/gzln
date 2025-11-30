package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/crypto"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

type FileHandler struct {
	fileService *service.FileService
	bucketName  string
}

type ChunkHandler struct {
	chunkService *service.ChunkService
	bucketName   string
}

func NewFileHandler(fileService *service.FileService, bucketName string) *FileHandler {
	return &FileHandler{
		fileService: fileService,
		bucketName:  bucketName,
	}
}

func NewChunkHandler(chunkService *service.ChunkService, bucketName string) *ChunkHandler {
	return &ChunkHandler{
		chunkService: chunkService,
		bucketName:   bucketName,
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
		URL:         fmt.Sprintf("/api/v1/files/%s", fileID+ext),
	}

	log.Printf("Response %+v", response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *ChunkHandler) HandleChunkUpload(w http.ResponseWriter, r *http.Request) {
	// TODO validate token??

	err := r.ParseForm() /// TODO add max chunk Size
	if err != nil {
		http.Error(w, "Failed to parse Form Data", http.StatusBadRequest)
		return
	}

	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		http.Error(w, "Authorization token is missing", http.StatusUnauthorized)
		return
	}

	file, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "File chunk is missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate Hash
	chunkBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read the chunk", http.StatusInternalServerError)
		return
	}

	computedHash := crypto.HashBytes(chunkBytes)
	requestHash := r.FormValue("hash")

	if !crypto.CompareHash(requestHash, computedHash) {
		http.Error(w, "Hash mismatch for chunk", http.StatusBadRequest)
	}

	// Validate chunk_index from DB
	fileID := chi.URLParam(r, "fileId")
	var fileUUID pgtype.UUID
	err = fileUUID.Scan(fileID)
	if err != nil {
		http.Error(w, "Invalid FileID failed to parse pgtype.UUID", http.StatusBadRequest)
		return
	}

	chunkIndexStr := r.FormValue("chunk_index")
	chunkIndex64, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	exists, err := h.chunkService.ExistsBy(ctx, fileUUID, int32(chunkIndex64))

	if exists {
		http.Error(w, "Chunk already uploaded with this ID", http.StatusBadRequest)
		return
	} else if err != nil {
		http.Error(w, "Error duinr chunk id check", http.StatusInternalServerError)
		return
	}

	// Upload chunk to MinIOS3

	// Inster a metadata Row in DBfor the chunk

	// return response
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
