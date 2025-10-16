package types

type FileMetadata struct {
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
}
