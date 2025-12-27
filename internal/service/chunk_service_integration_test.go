package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"testing"
	"time"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/crypto"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	minioc "github.com/testcontainers/testcontainers-go/modules/minio"
)

const testBucketName = "test-uploads"

// testEnvironment holds both PostgreSQL and MinIO containers
type testEnvironment struct {
	dbContainer    *testDBContainer
	minioContainer *minioc.MinioContainer
	minioClient    *minio.Client
	chunkService   *ChunkService
}

func setupTestEnvironment(t *testing.T) (*testEnvironment, func()) {
	ctx := context.Background()

	dbContainer, dbCleanup := setupTestDB(t)

	minioContainer, err := minioc.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z")
	require.NoError(t, err, "Failed to start MinIO container")

	endpoint, err := minioContainer.ConnectionString(ctx)
	require.NoError(t, err)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	require.NoError(t, err)

	err = minioClient.MakeBucket(ctx, testBucketName, minio.MakeBucketOptions{})
	require.NoError(t, err)

	chunkService := NewChunkService(dbContainer.queries, minioClient, testBucketName)

	cleanup := func() {
		objectsCh := minioClient.ListObjects(ctx, testBucketName, minio.ListObjectsOptions{Recursive: true})
		for object := range objectsCh {
			if object.Err != nil {
				continue
			}
			_ = minioClient.RemoveObject(ctx, testBucketName, object.Key, minio.RemoveObjectOptions{})
		}

		_ = minioClient.RemoveBucket(ctx, testBucketName)

		dbCleanup()
		if err := minioContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate MinIO container: %s", err)
		}
	}

	return &testEnvironment{
		dbContainer:    dbContainer,
		minioContainer: minioContainer,
		minioClient:    minioClient,
		chunkService:   chunkService,
	}, cleanup
}

func TestProcessChunkUpload_Integration_Success(t *testing.T) {
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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

	exists, err := env.dbContainer.queries.ChunkExistsByFileIdAndIndex(ctx, sqlc.ChunkExistsByFileIdAndIndexParams{
		FileID:     file.ID,
		ChunkIndex: 0,
	})
	require.NoError(t, err)
	assert.True(t, exists, "Chunk should exist in database")

	objectName := fmt.Sprintf("%s/0.enc", file.ID)
	object, err := env.minioClient.GetObject(ctx, testBucketName, objectName, minio.GetObjectOptions{})
	require.NoError(t, err)
	defer object.Close()

	downloadedData, err := io.ReadAll(object)
	require.NoError(t, err)
	assert.Equal(t, chunkData, downloadedData)
}

func TestProcessChunkUpload_Integration_HashMismatch(t *testing.T) {
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a file with "ready" status (not "uploading")
	file := createTestFile(t, env.dbContainer.queries, ctx, 5, 4)

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
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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

	file, err = env.dbContainer.queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
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
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	file := createTestFile(t, env.dbContainer.queries, ctx, 5, 4)

	_, err := env.chunkService.DownloadChunk(ctx, file.ShareID, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get chunk storage path")
}

func TestDownloadChunk_Integration_LimitReached(t *testing.T) {
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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

	_, err = env.dbContainer.pool.Exec(ctx, `
		UPDATE files SET max_downloads = 1, download_count = 1, status = 'ready'
		WHERE id = $1
	`, file.ID)
	require.NoError(t, err)

	_, err = env.chunkService.DownloadChunk(ctx, file.ShareID, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "download limit reached")
}

func TestCompleteUploadDownloadFlow_Integration(t *testing.T) {
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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

	_, err := env.dbContainer.queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
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
	env, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	file := createTestFileForUpload(t, env.dbContainer.queries, ctx)

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
	stat, err := env.minioClient.StatObject(ctx, testBucketName, objectName, minio.StatObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(len(chunkData)), stat.Size)

	assert.Equal(t, "large-chunk.bin", stat.UserMetadata["Original-Filename"])

	object, err := env.minioClient.GetObject(ctx, testBucketName, objectName, minio.GetObjectOptions{})
	require.NoError(t, err)
	defer object.Close()

	downloadedData, err := io.ReadAll(object)
	require.NoError(t, err)
	assert.Equal(t, chunkData, downloadedData)
}

func createTestFileForUpload(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()

	shareID := generateShareID()

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           shareID,
		EncryptedFilename: "encrypted-test-upload",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	return file
}
