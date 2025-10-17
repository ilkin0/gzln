package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/jackc/pgx/v5/pgtype"
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

// Helper function to extract client IP from request
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first (for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// Helper function to generate a random share ID (12 characters)
func generateShareID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 12)
	for i := range b {
		b[i] = charset[uuid.New().ID()%uint32(len(charset))]
	}
	return string(b)
}

func (h *FileHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	// 1. Parse HTTP request body into DTO
	initRequest := new(types.InitUploadRequest)
	if err := json.NewDecoder(r.Body).Decode(initRequest); err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	log.Printf("Init Request: %+v", initRequest)

	// 2. Validate required fields
	if initRequest.Salt == "" {
		http.Error(w, "salt is required", http.StatusBadRequest)
		return
	}
	if initRequest.EncryptedFilename == "" {
		http.Error(w, "encrypted_filename is required", http.StatusBadRequest)
		return
	}
	if initRequest.EncryptedMimeType == "" {
		http.Error(w, "encrypted_mime_type is required", http.StatusBadRequest)
		return
	}
	if initRequest.TotalSize <= 0 {
		http.Error(w, "total_size must be positive", http.StatusBadRequest)
		return
	}
	if initRequest.ChunkCount <= 0 {
		http.Error(w, "chunk_count must be positive", http.StatusBadRequest)
		return
	}
	if initRequest.ChunkSize <= 0 {
		http.Error(w, "chunk_size must be positive", http.StatusBadRequest)
		return
	}
	if initRequest.Pbkdf2Iterations <= 0 {
		http.Error(w, "pbkdf2_iterations must be positive", http.StatusBadRequest)
		return
	}

	// Validate business rules
	if initRequest.TotalSize > 5<<30 {
		http.Error(w, fmt.Sprintf("File size %d exceeds max size of 5GB", initRequest.TotalSize), http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	// 3. Generate IDs
	fileID := uuid.New()
	shareID := generateShareID()
	uploadToken := uuid.New().String()

	// 4. Set defaults for optional fields
	maxDownloads := initRequest.MaxDownloads
	if maxDownloads == 0 {
		maxDownloads = 1
	}
	expiresInHours := initRequest.ExpiresInHours
	if expiresInHours == 0 {
		expiresInHours = 24
	}

	expiresAt := time.Now().Add(time.Duration(expiresInHours) * time.Hour)

	clientIPStr := getClientIP(r)
	clientIP, err := netip.ParseAddr(clientIPStr)
	if err != nil {

		clientIP = netip.MustParseAddr("127.0.0.1")
		log.Printf("Failed to parse client IP %s: %v, using 127.0.0.1", clientIPStr, err)
	}

	params := sqlc.CreateFileParams{
		ShareID:           shareID,
		EncryptedFilename: initRequest.EncryptedFilename,
		EncryptedMimeType: initRequest.EncryptedMimeType,
		Salt:              initRequest.Salt,
		Pbkdf2Iterations:  initRequest.Pbkdf2Iterations,
		TotalSize:         initRequest.TotalSize,
		ChunkCount:        initRequest.ChunkCount,
		ChunkSize:         initRequest.ChunkSize,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
		MaxDownloads: maxDownloads,
		DeletionTokenHash: pgtype.Text{
			String: uploadToken, // TODO: Hash this token before storing
			Valid:  true,
		},
		UploaderIp: clientIP,
	}

	_, err = h.fileService.CreateFileRecord(ctx, params)
	if err != nil {
		log.Printf("Failed to create file record: %v", err)
		http.Error(w, "Failed to create file record", http.StatusInternalServerError)
		return
	}

	log.Printf("Created file record: ID=%s, ShareID=%s", fileID.String(), shareID)

	response := types.InitUploadResponse{
		FileID:      fileID.String(),
		ShareID:     shareID,
		UploadToken: uploadToken,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}

	log.Printf("Response: %+v", response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
