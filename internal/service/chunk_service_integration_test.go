package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/crypto"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	chunkService *ChunkService
	queries      *sqlc.Queries
	minioClient  *minio.Client
	bucketName   string
	pool         interface {
		Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	}
}

func setupTestChunkService(t *testing.T) (*testEnv, func()) {
	t.Helper()

	containers := testutil.SetupTestContainers(t)

	chunkService := NewChunkService(containers.Database.Queries, containers.MinioClient.Client, containers.MinioClient.BucketName)

	return &testEnv{
		chunkService: chunkService,
		queries:      containers.Database.Queries,
		minioClient:  containers.MinioClient.Client,
		bucketName:   containers.MinioClient.BucketName,
		pool:         containers.Database.Pool,
	}, containers.Cleanup
}

func TestProcessChunkUpload_Integration_Success(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()

	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunkData := []byte("This is test chunk data for upload")
	expectedHash := crypto.HashBytes(chunkData)

	req := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: expectedHash,
		ContentType:  "application/octet-stream",
		Filename:     "test-file.txt",
	}

	resp, err := env.chunkService.ProcessChunkUpload(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.ChunkIndex)
	assert.Equal(t, "uploaded", resp.Status)
	assert.Equal(t, expectedHash, resp.ReceivedHash)

	exists, err := env.queries.ChunkExistsByFileIdAndIndex(ctx, sqlc.ChunkExistsByFileIdAndIndexParams{
		FileID:     file.ID,
		ChunkIndex: 0,
	})
	require.NoError(t, err)
	assert.True(t, exists, "Chunk should exist in database")

	objectName := fmt.Sprintf("%s/0.enc", file.ID)
	object, err := env.minioClient.GetObject(ctx, env.bucketName, objectName, minio.GetObjectOptions{})
	require.NoError(t, err)
	defer object.Close()

	downloadedData, err := io.ReadAll(object)
	require.NoError(t, err)
	assert.Equal(t, chunkData, downloadedData)
}

func TestProcessChunkUpload_Integration_HashMismatch(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()
	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunkData := []byte("Test data")
	wrongHash := "wrong-hash-value"

	req := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: wrongHash,
		ContentType:  "application/octet-stream",
		Filename:     "test.txt",
	}

	_, err := env.chunkService.ProcessChunkUpload(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash mismatch")
}

func TestProcessChunkUpload_Integration_DuplicateChunk(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()
	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunkData := []byte("Test data")
	expectedHash := crypto.HashBytes(chunkData)

	req := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: expectedHash,
		ContentType:  "application/octet-stream",
		Filename:     "test.txt",
	}

	_, err := env.chunkService.ProcessChunkUpload(ctx, req)
	require.NoError(t, err)

	_, err = env.chunkService.ProcessChunkUpload(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already uploaded")
}

func TestProcessChunkUpload_Integration_InvalidFileStatus(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a file with "ready" status (not "uploading")
	file := testutil.CreateReadyFile(t, env.queries, ctx)

	chunkData := []byte("Test data")
	expectedHash := crypto.HashBytes(chunkData)

	req := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: expectedHash,
		ContentType:  "application/octet-stream",
		Filename:     "test.txt",
	}

	_, err := env.chunkService.ProcessChunkUpload(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in uploading state")
}

func TestDownloadChunk_Integration_Success(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()

	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunkData := []byte("Test chunk data for download")
	expectedHash := crypto.HashBytes(chunkData)

	uploadReq := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: expectedHash,
		ContentType:  "application/octet-stream",
		Filename:     "test.txt",
	}
	_, err := env.chunkService.ProcessChunkUpload(ctx, uploadReq)
	require.NoError(t, err)

	file, err = env.queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	reader, err := env.chunkService.DownloadChunk(ctx, file.ShareID, 0)
	require.NoError(t, err)
	defer reader.Close()

	downloadedData, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, chunkData, downloadedData)
}

func TestDownloadChunk_Integration_ChunkNotFound(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()
	file := testutil.CreateReadyFile(t, env.queries, ctx)

	_, err := env.chunkService.DownloadChunk(ctx, file.ShareID, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get chunk storage path")
}

func TestDownloadChunk_Integration_LimitReached(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()

	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunkData := []byte("Test data")
	expectedHash := crypto.HashBytes(chunkData)
	uploadReq := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: expectedHash,
		ContentType:  "application/octet-stream",
		Filename:     "test.txt",
	}
	_, err := env.chunkService.ProcessChunkUpload(ctx, uploadReq)
	require.NoError(t, err)

	_, err = env.pool.Exec(ctx, `
		UPDATE files SET max_downloads = 1, download_count = 1, status = 'ready'
		WHERE id = $1
	`, file.ID)
	require.NoError(t, err)

	_, err = env.chunkService.DownloadChunk(ctx, file.ShareID, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download limit reached")
}

func TestCompleteUploadDownloadFlow_Integration(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()

	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunks := [][]byte{
		[]byte("Chunk 0 data - first part"),
		[]byte("Chunk 1 data - second part"),
		[]byte("Chunk 2 data - third part"),
		[]byte("Chunk 3 data - fourth part"),
	}

	for i, chunkData := range chunks {
		hash := crypto.HashBytes(chunkData)
		req := types.ChunkUploadRequest{
			FileID:       file.ID,
			ChunkIndex:   int64(i),
			ChunkData:    chunkData,
			ExpectedHash: hash,
			ContentType:  "application/octet-stream",
			Filename:     fmt.Sprintf("chunk-%d.txt", i),
		}

		resp, err := env.chunkService.ProcessChunkUpload(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int64(i), resp.ChunkIndex)
	}

	_, err := env.queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	for i, expectedData := range chunks {
		reader, err := env.chunkService.DownloadChunk(ctx, file.ShareID, int64(i))
		require.NoError(t, err)

		downloadedData, err := io.ReadAll(reader)
		reader.Close()
		require.NoError(t, err)
		assert.Equal(t, expectedData, downloadedData, "Chunk %d data mismatch", i)
	}
}

func TestMinIOStorageIntegrity_Integration(t *testing.T) {
	env, cleanup := setupTestChunkService(t)
	defer cleanup()

	ctx := context.Background()
	file := testutil.CreateUploadingFile(t, env.queries, ctx)

	chunkData := bytes.Repeat([]byte("X"), 1024*1024)
	expectedHash := crypto.HashBytes(chunkData)

	req := types.ChunkUploadRequest{
		FileID:       file.ID,
		ChunkIndex:   0,
		ChunkData:    chunkData,
		ExpectedHash: expectedHash,
		ContentType:  "application/octet-stream",
		Filename:     "large-chunk.bin",
	}

	_, err := env.chunkService.ProcessChunkUpload(ctx, req)
	require.NoError(t, err)

	objectName := fmt.Sprintf("%s/0.enc", file.ID)
	stat, err := env.minioClient.StatObject(ctx, env.bucketName, objectName, minio.StatObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(len(chunkData)), stat.Size)

	assert.Equal(t, "large-chunk.bin", stat.UserMetadata["Original-Filename"])

	object, err := env.minioClient.GetObject(ctx, env.bucketName, objectName, minio.GetObjectOptions{})
	require.NoError(t, err)
	defer object.Close()

	downloadedData, err := io.ReadAll(object)
	require.NoError(t, err)
	assert.Equal(t, chunkData, downloadedData)
}

