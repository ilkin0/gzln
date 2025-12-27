package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

var (
	ErrNotFound             = errors.New("file not found")
	ErrNotReady             = errors.New("file not ready")
	ErrExpired              = errors.New("file expired")
	ErrDownloadLimitReached = errors.New("download limit reached")
)

type FileService struct {
	repository  sqlc.Querier
	minioClient *minio.Client
	runTx       database.TxRunner
}

func NewFileService(repository sqlc.Querier, runTx database.TxRunner, minioClient *minio.Client) *FileService {
	return &FileService{
		repository:  repository,
		runTx:       runTx,
		minioClient: minioClient,
	}
}

func (s *FileService) GetMinIOClient() *minio.Client {
	return s.minioClient
}

func generateShareID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 12
	b := make([]byte, length)

	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func (s *FileService) InitFileUpload(ctx context.Context, req types.InitUploadRequest, clientIPStr string) (*types.InitUploadResponse, error) {
	if err := s.validateUploadRequest(req); err != nil {
		return nil, err
	}

	shareID := generateShareID()
	uploadToken := uuid.New().String()

	maxDownloads := req.MaxDownloads
	if maxDownloads == 0 {
		maxDownloads = 5 // TODO make it configurable
	}

	expiresInHours := req.ExpiresInHours
	if expiresInHours == 0 {
		expiresInHours = 72 // TODO make it configurable
	}

	expiresAt := time.Now().Add(time.Duration(expiresInHours) * time.Hour)
	clientIP, err := netip.ParseAddr(clientIPStr)
	if err != nil {
		clientIP = netip.MustParseAddr("127.0.0.1")
	}

	params := sqlc.CreateFileParams{
		ShareID:           shareID,
		EncryptedFilename: req.EncryptedFilename,
		EncryptedMimeType: req.EncryptedMimeType,
		Salt:              req.Salt,
		Pbkdf2Iterations:  req.Pbkdf2Iterations,
		TotalSize:         req.TotalSize,
		ChunkCount:        req.ChunkCount,
		ChunkSize:         req.ChunkSize,
		ExpiresAt: pgtype.Timestamptz{
			Time:  expiresAt,
			Valid: true,
		},
		MaxDownloads: maxDownloads,
		DeletionTokenHash: pgtype.Text{
			String: uploadToken, // TODO: Hash deletion_token before storing?
			Valid:  true,
		},
		UploaderIp: clientIP,
	}

	createdFile, err := s.repository.CreateFile(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	return &types.InitUploadResponse{
		FileID:      createdFile.ID.String(),
		ShareID:     shareID,
		UploadToken: uploadToken,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *FileService) validateUploadRequest(req types.InitUploadRequest) error {
	if req.Salt == "" {
		return fmt.Errorf("salt is required")
	}
	if req.EncryptedFilename == "" {
		return fmt.Errorf("encrypted_filename is required")
	}
	if req.EncryptedMimeType == "" {
		return fmt.Errorf("encrypted_mime_type is required")
	}
	if req.TotalSize <= 0 {
		return fmt.Errorf("total_size must be positive")
	}
	if req.ChunkCount <= 0 {
		return fmt.Errorf("chunk_count must be positive")
	}
	if req.ChunkSize <= 0 {
		return fmt.Errorf("chunk_size must be positive")
	}

	// Validate chunk_count calculation to ensure data integrity and prevent
	// malicious/buggy clients from sending incorrect values that could cause
	// incomplete uploads or storage inconsistencies
	expectedChunkCount := (req.TotalSize + int64(req.ChunkSize) - 1) / int64(req.ChunkSize)
	if int64(req.ChunkCount) != expectedChunkCount {
		return fmt.Errorf("chunk_count mismatch: expected %d, got %d", expectedChunkCount,
			req.ChunkCount)
	}

	lastChunkSize := req.TotalSize - (int64(req.ChunkCount-1) * int64(req.ChunkSize))
	if lastChunkSize <= 0 || lastChunkSize > int64(req.ChunkSize) {
		return fmt.Errorf("invalid last chunk size: %d", lastChunkSize)
	}

	if req.Pbkdf2Iterations <= 0 {
		return fmt.Errorf("pbkdf2_iterations must be positive")
	}

	const maxFileSize = 5 << 30 // 5GB TODO make it configurable
	if req.TotalSize > maxFileSize {
		return fmt.Errorf("file size %d exceeds maximum of %dGB", req.TotalSize, maxFileSize)
	}

	return nil
}

func (s *FileService) GetFileByShareID(ctx context.Context, shareID string) (sqlc.File, error) {
	return s.repository.GetFileByShareID(ctx, shareID)
}

func (s *FileService) UpdateFileStatus(ctx context.Context, fileID pgtype.UUID, status string) (sqlc.File, error) {
	return s.repository.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     fileID,
		Status: status,
	})
}

func (s *FileService) GetFileByID(ctx context.Context, fileID pgtype.UUID) (sqlc.File, error) {
	return s.repository.GetFileByID(ctx, fileID)
}

func (s *FileService) FinalizeUpload(ctx context.Context, fileId pgtype.UUID) (types.FinalizeUploadResponse, error) {
	fileMetadata, err := s.GetFileByID(ctx, fileId)
	if err != nil {
		return types.FinalizeUploadResponse{}, fmt.Errorf("failed to get file metadata: %w", err)
	}

	chunksCount, err := s.repository.CountChunksByFileId(ctx, fileId)
	if err != nil {
		return types.FinalizeUploadResponse{}, fmt.Errorf("failed to count chunks: %w", err)
	}

	if chunksCount != int64(fileMetadata.ChunkCount) {
		return types.FinalizeUploadResponse{}, fmt.Errorf("chunk count does not match file chunk count")
	}

	fileMetadata, err = s.UpdateFileStatus(ctx, fileMetadata.ID, "ready")
	if err != nil {
		return types.FinalizeUploadResponse{}, fmt.Errorf("failed to update file status: %w", err)
	}

	return types.FinalizeUploadResponse{
		ShareID:       fileMetadata.ShareID,
		DeletionToken: fileMetadata.DeletionTokenHash.String,
	}, nil
}

func (s *FileService) GetFileSalt(ctx context.Context, shareID string) (string, error) {
	salt, err := s.repository.GetFileSaltByShareId(ctx, shareID)
	if err != nil {
		return "", fmt.Errorf("salt could not be found for file with %s shareID", shareID)
	}
	return salt, nil
}

func (s *FileService) GetFileMetadataByShareID(ctx context.Context, shareID string) (sqlc.GetFileMetadataByShareIdRow, error) {
	mdata, err := s.repository.GetFileMetadataByShareId(ctx, shareID)
	if err != nil {
		return sqlc.GetFileMetadataByShareIdRow{}, fmt.Errorf("file could not be found for %s shareID", shareID)
	}
	return mdata, nil
}

func (s *FileService) CompleteDownload(ctx context.Context, shareID string) error {
	err := s.runTx(ctx, func(q *sqlc.Queries) error {
		row, err := q.CompleteFileDownloadByShareId(ctx, shareID)
		if err != nil {
			return err
		}
		if row.ReachedLimit.Bool {
			_, err = q.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{ID: row.ID, Status: "exhausted"})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("complete download failed: %w", err)
	}

	meta, gerr := s.repository.GetFileMetadataByShareId(ctx, shareID)
	if gerr != nil {
		return ErrNotFound
	}
	now := time.Now()
	switch {
	case meta.ExpiresAt.Valid && meta.ExpiresAt.Time.Before(now):
		return ErrExpired
	case meta.MaxDownloads > 0 && meta.DownloadCount >= meta.MaxDownloads:
		return ErrDownloadLimitReached
	default:
		return ErrNotReady
	}
}
