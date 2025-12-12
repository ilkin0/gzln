package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/netip"
	"time"

	"github.com/google/uuid"
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
		maxDownloads = 100
	}

	expiresInHours := req.ExpiresInHours
	if expiresInHours == 0 {
		expiresInHours = 72
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
			String: uploadToken, // TODO: Hash this token before storing
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
	if req.Pbkdf2Iterations <= 0 {
		return fmt.Errorf("pbkdf2_iterations must be positive")
	}

	const maxFileSize = 5 << 30 // 5GB
	if req.TotalSize > maxFileSize {
		return fmt.Errorf("file size %d exceeds maximum of 5GB", req.TotalSize)
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

func (s *FileService) GetFileByID(ctx context.Context, fileID pgtype.UUID) (*sqlc.File, error) {
	return nil, nil
}

func (s *FileService) IncrementDownloadCount(ctx context.Context, fileID pgtype.UUID) error {
	return nil
}
