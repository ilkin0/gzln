package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cleanupTestEnv struct {
	cleanupService *CleanupService
	queries        *sqlc.Queries
	db             *database.Database
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
		db:             containers.Database,
		minioClient:    containers.MinioClient.Client,
		bucketName:     containers.MinioClient.BucketName,
	}, containers.Cleanup
}

func TestCleanupExpiredFiles_Integration_NoExpiredFiles(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	activeFile := testutil.CreateReadyFile(t, env.queries, ctx)
	testutil.UploadTestChunks(t, env.minioClient, env.bucketName, activeFile.ID.String(), int(activeFile.ChunkCount))

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, deleted)

	file, err := env.queries.GetFileByID(ctx, activeFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", file.Status)

	for i := 0; i < int(activeFile.ChunkCount); i++ {
		objectName := fmt.Sprintf("%s/%d.enc", activeFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.NoError(t, err, "Chunk %d should still exist", i)
	}
}

func TestCleanupExpiredFiles_Integration_ExpiredByTime(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	expiredFile := testutil.CreateExpiredFile(t, env.queries, env.db, ctx)
	testutil.UploadTestChunks(t, env.minioClient, env.bucketName, expiredFile.ID.String(), int(expiredFile.ChunkCount))

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	file, err := env.queries.GetFileByID(ctx, expiredFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "expired", file.Status)

	for i := 0; i < int(expiredFile.ChunkCount); i++ {
		objectName := fmt.Sprintf("%s/%d.enc", expiredFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.Error(t, err, "Chunk %d should be deleted", i)
	}
}

func TestCleanupExpiredFiles_Integration_ExpiredByDownloadCount(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	maxedFile := testutil.CreateMaxedDownloadsFile(t, env.queries, ctx)
	testutil.UploadTestChunks(t, env.minioClient, env.bucketName, maxedFile.ID.String(), int(maxedFile.ChunkCount))

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	file, err := env.queries.GetFileByID(ctx, maxedFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "expired", file.Status)

	for i := 0; i < int(maxedFile.ChunkCount); i++ {
		objectName := fmt.Sprintf("%s/%d.enc", maxedFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.Error(t, err, "Chunk %d should be deleted", i)
	}
}

func TestCleanupExpiredFiles_Integration_MultipleExpiredFiles(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	expiredFile1 := testutil.CreateExpiredFile(t, env.queries, env.db, ctx)
	expiredFile2 := testutil.CreateExpiredFile(t, env.queries, env.db, ctx)
	activeFile := testutil.CreateReadyFile(t, env.queries, ctx)

	testutil.UploadTestChunks(t, env.minioClient, env.bucketName, expiredFile1.ID.String(), int(expiredFile1.ChunkCount))
	testutil.UploadTestChunks(t, env.minioClient, env.bucketName, expiredFile2.ID.String(), int(expiredFile2.ChunkCount))
	testutil.UploadTestChunks(t, env.minioClient, env.bucketName, activeFile.ID.String(), int(activeFile.ChunkCount))

	deleted, err := env.cleanupService.CleanupExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	file1, _ := env.queries.GetFileByID(ctx, expiredFile1.ID)
	file2, _ := env.queries.GetFileByID(ctx, expiredFile2.ID)
	activeFileAfter, _ := env.queries.GetFileByID(ctx, activeFile.ID)

	assert.Equal(t, "expired", file1.Status)
	assert.Equal(t, "expired", file2.Status)
	assert.Equal(t, "ready", activeFileAfter.Status)

	for i := 0; i < int(activeFile.ChunkCount); i++ {
		objectName := fmt.Sprintf("%s/%d.enc", activeFile.ID.String(), i)
		_, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
		require.NoError(t, err, "Active file chunk %d should still exist", i)
	}
}

func TestCleanupExpiredFiles_Integration_ChunksDeletedFromMinIO(t *testing.T) {
	env, cleanup := setupCleanupTestEnv(t)
	defer cleanup()

	ctx := context.Background()

	expiredFile := testutil.CreateExpiredFile(t, env.queries, env.db, ctx)

	chunkCount := int(expiredFile.ChunkCount)
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
