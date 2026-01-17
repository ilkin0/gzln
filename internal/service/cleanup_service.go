package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

type CleanupService struct {
	queries     *sqlc.Queries
	minioClient *minio.Client
	bucketName  string
}

func NewCleanupService(queries *sqlc.Queries, minioClient *minio.Client, bucketName string) *CleanupService {
	return &CleanupService{
		queries:     queries,
		minioClient: minioClient,
		bucketName:  bucketName,
	}
}

func (s *CleanupService) CleanupExpiredFiles(ctx context.Context) (int, error) {
	expiredFiles, err := s.queries.GetExpiredFiles(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get expired files: %w", err)
	}

	if len(expiredFiles) == 0 {
		return 0, nil
	}

	if err := s.deleteFileChunks(ctx, expiredFiles); err != nil {
		return 0, fmt.Errorf("failed to delete file chunks: %w", err)
	}

	expiredIds := make([]pgtype.UUID, len(expiredFiles))
	for i, file := range expiredFiles {
		expiredIds[i] = file.ID
	}

	if err := s.queries.ExpireFilesByIds(ctx, expiredIds); err != nil {
		return 0, fmt.Errorf("failed to expire files: %w", err)
	}

	return len(expiredFiles), nil
}

func (s *CleanupService) deleteFileChunks(ctx context.Context, expiredFiles []sqlc.GetExpiredFilesRow) error {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for _, file := range expiredFiles {
			fileID := file.ID.String()
			for i := int32(0); i < file.ChunkCount; i++ {
				objectsCh <- minio.ObjectInfo{
					Key: fmt.Sprintf("%s/%d.enc", fileID, i),
				}
			}
		}
	}()

	var lastErr error
	errorCh := s.minioClient.RemoveObjects(ctx, s.bucketName, objectsCh,
		minio.RemoveObjectsOptions{})
	for e := range errorCh {
		slog.Error("failed to delete object", slog.String("object", e.ObjectName),
			slog.String("error", e.Err.Error()))
		lastErr = e.Err
	}

	return lastErr
}
