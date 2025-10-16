package service

import (
	"context"
	"log"
	"net/netip"
	"time"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

type FileService struct {
	repository  sqlc.Querier
	minioClient *minio.Client
}

func NewFileService(repository sqlc.Querier, minioClient *minio.Client) *FileService {
	return &FileService{
		repository:  repository,
		minioClient: minioClient,
	}
}

func (s *FileService) GetMinIOClient() *minio.Client {
	return s.minioClient
}

func (s *FileService) CreateFileRecord(ctx context.Context, params sqlc.CreateFileParams) (sqlc.File, error) {
	log.Printf("Persisting File data: %v", params)
	return s.repository.CreateFile(ctx, params)
}

func (s *FileService) GetFileStatus(ctx context.Context, shareID string) (sqlc.File, error) {
	return s.repository.GetFileByShareID(ctx, shareID)
}

func (s *FileService) UpdateFileStatus(ctx context.Context, fileID pgtype.UUID, status string) (sqlc.File, error) {
	return s.repository.UpdateFileStatus(ctx, fileID, status)
}

func (s *FileService) SaveFileMetadata(ctx context.Context, fileMetadata types.FileMetadata) error {
	params := sqlc.CreateFileParams{
		ShareID:           "example-share-id",
		EncryptedFilename: "encrypted-name",
		EncryptedMimeType: fileMetadata.MimeType,
		Salt:              "generated-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         fileMetadata.FileSize,
		ChunkCount:        10,
		ChunkSize:         1024 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads: 10,
		DeletionTokenHash: pgtype.Text{
			String: "token-hash",
			Valid:  true,
		},
		UploaderIp: netip.MustParseAddr("127.0.0.1"),
	}
	_, err := s.repository.CreateFile(ctx, params)
	return err
}
