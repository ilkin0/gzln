package testutil

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/netip"
	"testing"
	"time"

	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

func ParseUUID(t *testing.T, uuidStr string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	err := id.Scan(uuidStr)
	require.NoError(t, err)
	return id
}

func GenerateShareID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 12
	b := make([]byte, length)

	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

type TestFileOptions struct {
	ShareID      string
	MaxDownloads int32
	ChunkCount   int32
	ChunkSize    int32
	TotalSize    int64
	ExpiresIn    time.Duration
	Status       string // "uploading" or "ready"
}

func DefaultTestFileOptions() TestFileOptions {
	return TestFileOptions{
		ShareID:      GenerateShareID(),
		MaxDownloads: 5,
		ChunkCount:   2,
		ChunkSize:    512,
		TotalSize:    1024,
		ExpiresIn:    24 * time.Hour,
		Status:       "ready",
	}
}

func CreateTestFile(t *testing.T, queries *sqlc.Queries, ctx context.Context, opts TestFileOptions) sqlc.File {
	t.Helper()

	if opts.ShareID == "" {
		opts.ShareID = GenerateShareID()
	}
	if opts.MaxDownloads == 0 {
		opts.MaxDownloads = 5
	}
	if opts.ChunkCount == 0 {
		opts.ChunkCount = 2
	}
	if opts.ChunkSize == 0 {
		opts.ChunkSize = 512
	}
	if opts.TotalSize == 0 {
		opts.TotalSize = 1024
	}
	if opts.ExpiresIn == 0 {
		opts.ExpiresIn = 24 * time.Hour
	}

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           opts.ShareID,
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         opts.TotalSize,
		ChunkCount:        opts.ChunkCount,
		ChunkSize:         opts.ChunkSize,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(opts.ExpiresIn),
			Valid: true,
		},
		MaxDownloads:      opts.MaxDownloads,
		DeletionTokenHash: pgtype.Text{String: "deletion-token", Valid: true},
		UploaderIp:        netip.MustParseAddr("127.0.0.1"),
	})
	require.NoError(t, err)

	if opts.Status == "ready" {
		file, err = queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
			ID:     file.ID,
			Status: "ready",
		})
		require.NoError(t, err)
	}

	return file
}

func CreateReadyFile(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()
	return CreateTestFile(t, queries, ctx, DefaultTestFileOptions())
}

func CreateExpiredFile(t *testing.T, queries *sqlc.Queries, db *database.Database, ctx context.Context) sqlc.File {
	t.Helper()

	file := CreateReadyFile(t, queries, ctx)

	now := time.Now()
	_, err := db.Pool.Exec(ctx, `
		UPDATE files
		SET created_at = $1, expires_at = $2
		WHERE id = $3
	`, now.Add(-2*time.Hour), now.Add(-1*time.Hour), file.ID)
	require.NoError(t, err)

	file, err = queries.GetFileByID(ctx, file.ID)
	require.NoError(t, err)

	return file
}

func CreateMaxedDownloadsFile(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()

	opts := DefaultTestFileOptions()
	opts.MaxDownloads = 3
	file := CreateTestFile(t, queries, ctx, opts)

	for i := 0; i < int(opts.MaxDownloads); i++ {
		_, err := queries.CompleteFileDownloadByShareId(ctx, file.ShareID)
		require.NoError(t, err)
	}

	file, err := queries.GetFileByID(ctx, file.ID)
	require.NoError(t, err)

	return file
}

func CreateUploadingFile(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()
	opts := DefaultTestFileOptions()
	opts.Status = "uploading"
	return CreateTestFile(t, queries, ctx, opts)
}

func UploadTestChunks(t *testing.T, minioClient *minio.Client, bucketName string, fileID string, chunkCount int) {
	t.Helper()
	ctx := context.Background()

	for i := 0; i < chunkCount; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", fileID, i)
		data := []byte(fmt.Sprintf("chunk data %d", i))

		_, err := minioClient.PutObject(ctx, bucketName, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}
}
