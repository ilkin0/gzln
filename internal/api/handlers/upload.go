package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/logger"
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Headers already sent, can't change status code
		// Error will be logged by middleware
		return
	}
}

func (h *ChunkHandler) HandleChunkUpload(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	// TODO validate token??
	err := r.ParseForm()
	if err != nil {
		log.Warn("failed to parse form",
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		log.Warn("missing authorization header")
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

	fileIDStr := chi.URLParam(r, "fileID")
	var fileID pgtype.UUID
	err = fileID.Scan(fileIDStr)
	if err != nil {
		log.Warn("invalid file ID",
			slog.String("file_id_str", fileIDStr),
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	chunkIndexStr := r.FormValue("chunk_index")
	chunkIndex64, err := strconv.ParseInt(chunkIndexStr, 10, 32)
	if err != nil {
		log.Warn("invalid chunk index",
			slog.String("chunk_index_str", chunkIndexStr),
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusBadRequest, "Invalid chunk index")
		return
	}

	log.Info("processing chunk upload",
		slog.String("file_id", fileIDStr),
		slog.Int64("chunk_index", chunkIndex64),
		slog.Int64("chunk_size", int64(len(chunkBytes))),
	)

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
		log.Error("chunk upload failed",
			slog.String("error", err.Error()),
			slog.String("file_id", fileIDStr),
			slog.Int64("chunk_index", chunkIndex64),
		)
		status := mapServiceErrorToHTTP(err)
		utils.Error(w, status, err.Error())
		return
	}

	log.Info("chunk uploaded successfully",
		slog.String("file_id", fileIDStr),
		slog.Int64("chunk_index", chunkIndex64),
		slog.String("hash", result.ReceivedHash),
	)

	utils.Ok(w, types.ChunkUploadResponse{
		ChunkIndex:   result.ChunkIndex,
		Status:       result.Status,
		ReceivedHash: result.ReceivedHash,
	})
}

func (h *FileHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	var req types.InitUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("invalid JSON in upload init request",
			slog.String("error", err.Error()),
		)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	clientIP := getClientIP(r)

	log.Info("initializing upload",
		slog.Int64("total_size", req.TotalSize),
		slog.Int("chunk_count", int(req.ChunkCount)),
		slog.String("client_ip", clientIP),
	)

	ctx := context.Background()
	response, err := h.fileService.InitFileUpload(ctx, req, clientIP)
	if err != nil {
		log.Error("failed to initialize upload",
			slog.String("error", err.Error()),
			slog.String("client_ip", clientIP),
			slog.Int64("total_size", req.TotalSize),
			slog.Int("chunk_count", int(req.ChunkCount)),
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Info("upload initialized successfully",
		slog.String("share_id", response.ShareID),
		slog.String("file_id", response.FileID),
	)

	utils.Ok(w, response)
}

func (h *FileHandler) FinalizeFileUpload(w http.ResponseWriter, r *http.Request) {
	log := logger.FromContext(r.Context())

	fileIDStr := chi.URLParam(r, "fileID")
	var fileID pgtype.UUID
	err := fileID.Scan(fileIDStr)
	if err != nil {
		log.Warn("invalid file ID for finalization",
			slog.String("file_id_str", fileIDStr),
			slog.String("error", err.Error()),
		)
		utils.Error(w, http.StatusBadRequest, "Invalid file ID")
		return
	}

	log.Info("finalizing upload",
		slog.String("file_id", fileIDStr),
	)

	ctx := context.Background()
	ures, err := h.fileService.FinalizeUpload(ctx, fileID)
	if err != nil {
		log.Error("failed to finalize upload",
			slog.String("error", err.Error()),
			slog.String("file_id", fileIDStr),
		)
		status := mapServiceErrorToHTTP(err)
		utils.Error(w, status, err.Error())
		return
	}

	log.Info("upload finalized successfully",
		slog.String("file_id", fileIDStr),
		slog.String("share_id", ures.ShareID),
	)

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
