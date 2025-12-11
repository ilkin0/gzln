package types

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type InitUploadRequest struct {
	Salt              string `json:"salt"`
	EncryptedFilename string `json:"encrypted_filename"`
	EncryptedMimeType string `json:"encrypted_mime_type"`
	TotalSize         int64  `json:"total_size"`
	ChunkCount        int32  `json:"chunk_count"`
	ChunkSize         int32  `json:"chunk_size"`
	ExpiresInHours    int    `json:"expires_in_hours,omitempty"`
	MaxDownloads      int32  `json:"max_downloads,omitempty"`
	Pbkdf2Iterations  int32  `json:"pbkdf2_iterations"`
}

type InitUploadResponse struct {
	FileID      string `json:"file_id"`
	ShareID     string `json:"share_id"`
	UploadToken string `json:"upload_token"`
	ExpiresAt   string `json:"expires_at"`
}

type UploadResponse struct {
	FileID      string    `json:"file_id"`
	FileName    string    `json:"file_name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url"`
}

type ChunkUploadRequest struct {
	FileID       pgtype.UUID
	ChunkIndex   int64
	ChunkData    []byte
	ExpectedHash string
	ContentType  string
	Filename     string
}

type ChunkUploadResponse struct {
	ChunkIndex   int64  `json:"chunk_index"`
	Status       string `json:"status"`
	ReceivedHash string `json:"received_hash"`
}
