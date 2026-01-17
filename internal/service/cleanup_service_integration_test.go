package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cleanupTestEnv struct {
	cleanupService *CleanupService
	queries        *sqlc.Queries
	minioClient    *minio.Client
	bucketName     string
}

func setupCleanupTestEnv(t *testing.T) (*cleanupTestEnv, func()) {
	t.Helper()

	containers := testutil.SetupTestContainers(t)

	cleanupService := NewCleanupService(
		containers.Database.Queries,
		containers.MinioClient.Client,
		containers.MinioClient.BucketName,
	)

	return &cleanupTestEnv{
		cleanupService: cleanupService,
		queries:        containers.Database.Queries,
		minioClient:    containers.MinioClient.Client,
		bucketName:     containers.MinioClient.BucketName,
	}, containers.Cleanup
}

func createExpiredFile2(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           fmt.Sprintf("exp%08d", time.Now().UnixNano()%100000000),
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024,
		ChunkCount:        2,
		ChunkSize:         512,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(-1 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "deletion-token", Valid: true},
		UploaderIp:        netip.MustParseAddr("127.0.0.1"),
	})
	require.NoError(t, err)

	file, err = queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	return file
}

func createActiveFile(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           fmt.Sprintf("act%09d", time.Now().UnixNano()%1000000000)[:12],
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024,
		ChunkCount:        2,
		ChunkSize:         512,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "deletion-token", Valid: true},
		UploaderIp:        netip.MustParseAddr("127.0.0.1"),
	})
	require.NoError(t, err)

	file, err = queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	return file
}

func createMaxedDownloadsFile(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           fmt.Sprintf("max%09d", time.Now().UnixNano()%1000000000)[:12],
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024,
		ChunkCount:        2,
		ChunkSize:         512,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour), // Not expired by time
			Valid: true,
		},
		MaxDownloads:      3,
		DeletionTokenHash: pgtype.Text{String: "deletion-token", Valid: true},
		UploaderIp:        netip.MustParseAddr("127.0.0.1"),
	})
	require.NoError(t, err)

	file, err = queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err = queries.CompleteFileDownloadByShareId(ctx, file.ShareID)
		require.NoError(t, err)
	}

	file, err = queries.GetFileByID(ctx, file.ID)
	require.NoError(t, err)

	return file
}

func uploadTestChunks(t *testing.T, minioClient *minio.Client, bucketName string, fileID string, chunkCount int) {
	t.Helper()
	ctx := context.Background()

	for i := 0; i < chunkCount; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", fileID, i)
		data := []byte(fmt.Sprintf("chunk data %d", i))

		_, err := minioClient.PutObject(ctx, bucketName, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}
}

func TestCleanupExpiredFiles_Integration_NoExpiredFiles(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	activeFile := createActiveFile(t, env.queries, ctx)
	uploadTestChunks(t, env.minioClient, env.bucketName, activeFile.ID.String(), 2)

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, deleted)

	file, err := env.queries.GetFileByID(ctx, activeFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", file.Status)

	for i := 0; i < 2; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", activeFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.NoError(t, err, "Chunk %d should still exist", i)
	}
}

func TestCleanupExpiredFiles_Integration_ExpiredByTime(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	expiredFile := createExpiredFile2(t, env.queries, ctx)
	uploadTestChunks(t, env.minioClient, env.bucketName, expiredFile.ID.String(), 2)

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	file, err := env.queries.GetFileByID(ctx, expiredFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "expired", file.Status)

	for i := 0; i < 2; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", expiredFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.Error(t, err, "Chunk %d should be deleted", i)
	}
}

func TestCleanupExpiredFiles_Integration_ExpiredByDownloadCount(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	maxedFile := createMaxedDownloadsFile(t, env.queries, ctx)
	uploadTestChunks(t, env.minioClient, env.bucketName, maxedFile.ID.String(), 2)

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	file, err := env.queries.GetFileByID(ctx, maxedFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "expired", file.Status)

	for i := 0; i < 2; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", maxedFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.Error(t, err, "Chunk %d should be deleted", i)
	}
}

func TestCleanupExpiredFiles_Integration_MultipleExpiredFiles(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	expiredFile1 := createExpiredFile2(t, env.queries, ctx)
	expiredFile2 := createExpiredFile2(t, env.queries, ctx)
	activeFile := createActiveFile(t, env.queries, ctx)

	uploadTestChunks(t, env.minioClient, env.bucketName, expiredFile1.ID.String(), 2)
	uploadTestChunks(t, env.minioClient, env.bucketName, expiredFile2.ID.String(), 2)
	uploadTestChunks(t, env.minioClient, env.bucketName, activeFile.ID.String(), 2)

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	file1, _ := env.queries.GetFileByID(ctx, expiredFile1.ID)
	file2, _ := env.queries.GetFileByID(ctx, expiredFile2.ID)
	activeFileAfter, _ := env.queries.GetFileByID(ctx, activeFile.ID)

	assert.Equal(t, "expired", file1.Status)
	assert.Equal(t, "expired", file2.Status)
	assert.Equal(t, "ready", activeFileAfter.Status)

	for i := 0; i < 2; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", activeFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.NoError(t, err, "Active file chunk %d should still exist", i)
	}
}

func TestCleanupExpiredFiles_Integration_ChunksDeletedFromMinIO(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	expiredFile := createExpiredFile2(t, env.queries, ctx)

	// Upload chunks to MinIO
	chunkCount := 5
	for i := 0; i < chunkCount; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", expiredFile.ID.String(), i)
		data := []byte(fmt.Sprintf("encrypted chunk data %d with some content", i))
		_, err := env.minioClient.PutObject(ctx, env.bucketName, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}

	for i := 0; i < chunkCount; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", expiredFile.ID.String(), i)
		obj, err := env.minioClient.GetObject(ctx, env.bucketName, objectName, minio.GetObjectOptions{})
		require.NoError(t, err)
		data, err := io.ReadAll(obj)
		require.NoError(t, err)
		assert.Contains(t, string(data), fmt.Sprintf("chunk data %d", i))
		obj.Close()
	}

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	for i := 0; i < chunkCount; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", expiredFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.Error(t, err, "Chunk %d should be deleted from MinIO", i)
	}
}
