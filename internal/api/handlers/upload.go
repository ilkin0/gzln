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
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/ilkin0/gzln/internal/utils"
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
	err := r.ParseForm()
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		utils.Error(w, http.StatusUnauthorized, "Authorization required")
		return
	}

	/// TODO add max chunk Size validation
	file, header, err := r.FormFile("chunk")
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "File chunk is missing")
		return
	}
	defer file.Close()

	// Validate Hash
	chunkBytes, err := io.ReadAll(file)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "Failed to read chunk")
		return
	}

	fileIDStr := chi.URLParam(r, "fileId")
	var fileID pgtype.UUID
	err = fileID.Scan(fileIDStr)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	chunkIndexStr := r.FormValue("chunk_index")
	chunkIndex64, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "Invalid chunk index")
		return
	}

	ctx := context.Background()
	req := types.ChunkUploadRequest{
		FileID:       fileID,
		ChunkIndex:   chunkIndex64,
		ChunkData:    chunkBytes,
		ExpectedHash: r.FormValue("hash"),
		ContentType:  header.Header.Get("Content-Type"),
		Filename:     header.Filename,
	}
	result, err := h.chunkService.ProcessChunkUpload(ctx, req)
	if err != nil {
		status := mapServiceErrorToHTTP(err)
		utils.Error(w, status, err.Error())
		return
	}

	utils.Ok(w, types.ChunkUploadResponse{
		ChunkIndex:   result.ChunkIndex,
		Status:       result.Status,
		ReceivedHash: result.ReceivedHash,
	})
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

	utils.Ok(w, response)
}

func (h *FileHandler) FinalizeFileUpload(w http.ResponseWriter, r *http.Request) {
	fileIDStr := chi.URLParam(r, "fileId")
	var fileID pgtype.UUID
	err := fileID.Scan(fileIDStr)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	ctx := context.Background()
	ures, err := h.fileService.FinalizeUpload(ctx, fileID)
	if err != nil {
		status := mapServiceErrorToHTTP(err)
		utils.Error(w, status, err.Error())
		return
	}
	utils.Ok(w, ures)
}

func mapServiceErrorToHTTP(err error) int {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "already uploaded"):
		return http.StatusConflict
	case strings.Contains(errMsg, "invalid"):
		return http.StatusBadRequest
	case strings.Contains(errMsg, "hash mismatch"):
		return http.StatusBadRequest
	case strings.Contains(errMsg, "not found"):
		return http.StatusNotFound
	case strings.Contains(errMsg, "not in uploading state"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
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
