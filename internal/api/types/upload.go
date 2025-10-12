package types

import "time"

type InitUploadRequest struct{}

type UploadResponse struct {
	FileID      string    `json:"file_id"`
	FileName    string    `json:"file_name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url"`
}
