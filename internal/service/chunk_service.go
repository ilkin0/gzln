package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/crypto"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

type ChunkService struct {
	repository  sqlc.Querier
	minioClient *minio.Client
	bucketName  string
}

func NewChunkService(repository sqlc.Querier, minioClient *minio.Client, bucketName string) *ChunkService {
	return &ChunkService{
		repository:  repository,
		minioClient: minioClient,
		bucketName:  bucketName,
	}
}

func (cs *ChunkService) GetMinIOClient() *minio.Client {
	return cs.minioClient
}

func (cs *ChunkService) existsBy(ctx context.Context, fileID pgtype.UUID, chunkIndex int64) (bool, error) {
	return cs.repository.ChunkExistsByFileIdAndIndex(ctx, sqlc.ChunkExistsByFileIdAndIndexParams{
		FileID:     fileID,
		ChunkIndex: int32(chunkIndex),
	})
}

func (cs *ChunkService) createChunkRecord(ctx context.Context, fileID pgtype.UUID, chunkIndex64 int64, sotragePath string, encryptedSize int64, chunkHash string) (int64, error) {
	return cs.repository.CreateChunk(ctx, sqlc.CreateChunkParams{
		FileID:        fileID,
		ChunkIndex:    int32(chunkIndex64),
		StoragePath:   sotragePath,
		EncryptedSize: encryptedSize,
		ChunkHash:     chunkHash,
	})
}

func (cs *ChunkService) ProcessChunkUpload(ctx context.Context, req types.ChunkUploadRequest) (types.ChunkUploadResponse, error) {
	slog.Debug("processing chunk upload",
		slog.String("file_id", req.FileID.String()),
		slog.Int64("chunk_index", req.ChunkIndex),
		slog.Int("chunk_size", len(req.ChunkData)),
	)

	// Validate chunk doesn't already exist and file exists with "uploading" status
	err := cs.validateChunkUpload(ctx, req.FileID, req.ChunkIndex)
	if err != nil {
		slog.Warn("chunk validation failed",
			slog.String("error", err.Error()),
			slog.String("file_id", req.FileID.String()),
			slog.Int64("chunk_index", req.ChunkIndex),
		)
		return types.ChunkUploadResponse{}, err
	}

	// Validate Hash
	slog.Debug("validating chunk hash",
		slog.String("file_id", req.FileID.String()),
		slog.Int64("chunk_index", req.ChunkIndex),
		slog.String("expected_hash", req.ExpectedHash),
	)

	err = cs.validateChunkHash(req.ChunkData, req.ExpectedHash)
	if err != nil {
		slog.Warn("chunk hash validation failed",
			slog.String("error", err.Error()),
			slog.String("file_id", req.FileID.String()),
			slog.Int64("chunk_index", req.ChunkIndex),
		)
		return types.ChunkUploadResponse{}, err
	}

	// Upload to Storage
	slog.Debug("uploading chunk to storage",
		slog.String("file_id", req.FileID.String()),
		slog.Int64("chunk_index", req.ChunkIndex),
	)

	filePath, err := cs.uploadChunkToStorage(ctx, req.FileID, req.ChunkIndex, req.ChunkData, req.ContentType, req.Filename)
	if err != nil {
		return types.ChunkUploadResponse{}, err
	}

	// Create chunk metadata record in database
	slog.Debug("creating chunk metadata record",
		slog.String("file_id", req.FileID.String()),
		slog.Int64("chunk_index", req.ChunkIndex),
		slog.String("storage_path", filePath),
	)

	_, err = cs.createChunkRecord(ctx, req.FileID, req.ChunkIndex, filePath, int64(len(req.ChunkData)), req.ExpectedHash)
	if err != nil {
		slog.Error("failed to create chunk record",
			slog.String("error", err.Error()),
			slog.String("file_id", req.FileID.String()),
			slog.Int64("chunk_index", req.ChunkIndex),
		)
		return types.ChunkUploadResponse{}, err
	}

	slog.Info("chunk uploaded successfully",
		slog.String("file_id", req.FileID.String()),
		slog.Int64("chunk_index", req.ChunkIndex),
		slog.String("hash", req.ExpectedHash),
	)

	return types.ChunkUploadResponse{
		ChunkIndex:   req.ChunkIndex,
		Status:       "uploaded",
		ReceivedHash: req.ExpectedHash,
	}, nil
}

func (cs *ChunkService) validateChunkHash(data []byte, expectedHash string) error {
	computedHash := crypto.HashBytes(data)
	if !crypto.CompareHash(expectedHash, computedHash) {
		return fmt.Errorf("hash mismatch for chunk upload")
	}

	return nil
}

func (cs *ChunkService) uploadChunkToStorage(ctx context.Context, fileID pgtype.UUID, chunkIndex int64,
	data []byte, contentType, filename string,
) (string, error) {
	objectName := fmt.Sprintf("%s/%d.enc", fileID, chunkIndex)
	reader := bytes.NewReader(data)

	_, err := cs.GetMinIOClient().PutObject(
		ctx,
		cs.bucketName,
		objectName,
		reader,
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"original-filename": filename,
			},
		},
	)
	if err != nil {
		slog.Error("failed to upload chunk to storage",
			slog.String("error", err.Error()),
			slog.String("file_id", fmt.Sprintf("%x-%x-%x-%x-%x", fileID.Bytes[0:4], fileID.Bytes[4:6], fileID.Bytes[6:8], fileID.Bytes[8:10], fileID.Bytes[10:16])),
			slog.Int64("chunk_index", chunkIndex),
			slog.String("object_name", objectName),
		)
		return "", err
	}

	return objectName, nil
}

func (cs *ChunkService) validateChunkUpload(ctx context.Context, fileID pgtype.UUID, chunkIndex int64) error {
	// Validate chunk doesn't already exist
	exists, err := cs.existsBy(ctx, fileID, chunkIndex)
	if err != nil {
		return fmt.Errorf("failed to check chunk existence: %w", err)
	}
	if exists {
		return fmt.Errorf("chunk %d already uploaded for file %s", chunkIndex, fileID.Bytes)
	}

	// Validate file exists with "uploading" status
	exists, err = cs.fileExistsByIdAndStatus(ctx, fileID, "uploading")
	if err != nil {
		return fmt.Errorf("failed to verify file status: %w", err)
	}
	if !exists {
		return fmt.Errorf("file %s does not exist or is not in uploading state", fileID.Bytes)
	}
	return nil
}

func (cs *ChunkService) fileExistsByIdAndStatus(ctx context.Context, fileID pgtype.UUID, status string) (bool, error) {
	return cs.repository.FileExistsByIdAndStatus(ctx, sqlc.FileExistsByIdAndStatusParams{
		ID:     fileID,
		Status: status,
	})
}

func (cs *ChunkService) DownloadChunk(ctx context.Context, shareId string, chunkIndex int64) (io.ReadCloser, error) {
	slog.Debug("fetching chunk details",
		slog.String("share_id", shareId),
		slog.Int64("chunk_index", chunkIndex),
	)

	chunkDetails, err := cs.repository.GetChunkByIndexAndFileShareID(ctx, sqlc.GetChunkByIndexAndFileShareIDParams{
		ShareID:    shareId,
		ChunkIndex: int32(chunkIndex),
	})

	if err != nil {
		slog.Warn("failed to get chunk metadata",
			slog.String("error", err.Error()),
			slog.String("share_id", shareId),
			slog.Int64("chunk_index", chunkIndex),
		)
		return nil, fmt.Errorf("failed to get chunk storage path: %w", err)
	}

	if chunkDetails.DownloadCount >= chunkDetails.MaxDownloads {
		slog.Warn("chunk download limit reached",
			slog.String("share_id", shareId),
			slog.Int64("chunk_index", chunkIndex),
			slog.Int("download_count", int(chunkDetails.DownloadCount)),
			slog.Int("max_downloads", int(chunkDetails.MaxDownloads)),
		)
		return nil, fmt.Errorf("chunk download limit reached")
	}

	slog.Debug("retrieving chunk from storage",
		slog.String("share_id", shareId),
		slog.Int64("chunk_index", chunkIndex),
		slog.String("storage_path", chunkDetails.StoragePath),
	)

	chunk, err := cs.minioClient.GetObject(
		ctx,
		cs.bucketName,
		chunkDetails.StoragePath,
		minio.GetObjectOptions{},
	)
	if err != nil {
		slog.Error("failed to retrieve chunk from storage",
			slog.String("error", err.Error()),
			slog.String("share_id", shareId),
			slog.Int64("chunk_index", chunkIndex),
			slog.String("storage_path", chunkDetails.StoragePath),
		)
		return nil, fmt.Errorf("failed to download chunk from storage: %w", err)
	}

	if _, err := chunk.Stat(); err != nil {
		chunk.Close()
		slog.Error("failed to stat chunk object",
			slog.String("error", err.Error()),
			slog.String("share_id", shareId),
			slog.Int64("chunk_index", chunkIndex),
		)
		return nil, fmt.Errorf("failed to stat chunk: %w", err)
	}

	slog.Info("chunk retrieved successfully",
		slog.String("share_id", shareId),
		slog.Int64("chunk_index", chunkIndex),
	)

	return chunk, nil
}
