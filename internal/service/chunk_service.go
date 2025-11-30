package service

import (
	"context"

	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

type ChunkService struct {
	repository  sqlc.Querier
	minioClient *minio.Client
}

func NewChunkService(repository sqlc.Querier, minioClient *minio.Client) *ChunkService {
	return &ChunkService{
		repository:  repository,
		minioClient: minioClient,
	}
}

func (c *ChunkService) ExistsBy(ctx context.Context, fileID pgtype.UUID, chunkIndex int32) (bool, error) {
	return c.repository.ChunkExistsByFileIdAndIndex(ctx, fileID, chunkIndex)
}

func (c *ChunkService) UploadChunk() {
}
