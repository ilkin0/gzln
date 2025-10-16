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

func (h *FileHandler) InitUpload(w http.ResponseWriter, r *http.Request) {
	initRequest := new(types.InitUploadRequest)
	if err := json.NewDecoder(r.Body).Decode(initRequest); err != nil {
		http.Error(w, "Failed to read Request Body!", http.StatusBadRequest)
		return
	}
	log.Printf("Init Request: %+v", initRequest)
	if initRequest.FileSize >= 20<<20 {
		http.Error(w, fmt.Sprintf("File size %d exceeds max size %d", initRequest.FileSize, 20<<20), http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	fileID := uuid.New().String()
	chunkCount := initRequest.FileSize / 5

	params := sqlc.CreateFileParams{
		ShareID:           "12_char_shar",
		EncryptedFilename: "encrypted-file",
		EncryptedMimeType: initRequest.MimeType,
		Salt:              "temp-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         initRequest.FileSize,
		ChunkCount:        int32(chunkCount),
		ChunkSize:         1024 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads: 10,
		DeletionTokenHash: pgtype.Text{
			String: "temp-token",
			Valid:  true,
		},
		UploaderIp: netip.MustParseAddr("127.0.0.1"),
	}

	file, err := h.fileService.CreateFileRecord(ctx, params)
	if err != nil {
		log.Printf("Failed to create file record: %v", err)
		http.Error(w, "Failed to create file record", http.StatusInternalServerError)
		return
	}

	log.Printf("Created file record: %+v", file)

	response := types.InitUploadResponse{
		FileID:      fileID,
		ChunkCount:  chunkCount,
		UploadToken: uuid.New().String(),
	}

	log.Printf("Response %+v", response)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
