package types

import "time"

type InitUploadRequest struct {
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
}

type InitUploadResponse struct {
	FileID      string `json:"file_id"`
	UploadToken string `json:"upload_token"`
	ChunkCount  int64  `json:"chunk_count"`
}

type UploadResponse struct {
	FileID      string    `json:"file_id"`
	FileName    string    `json:"file_name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url"`
}
