package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestFileService(t *testing.T) (*FileService, *sqlc.Queries, *database.Database, func()) {
	t.Helper()

	containers := testutil.SetupTestContainers(t)

	txRunner := database.NewTxRunner(containers.Database.Pool)
	fileService := NewFileService(containers.Database.Queries, txRunner, containers.MinioClient.Client)

	return fileService, containers.Database.Queries, containers.Database, containers.Cleanup
}

func createTestFileWithOpts(t *testing.T, queries *sqlc.Queries, ctx context.Context, maxDownloads, chunkCount int32) sqlc.File {
	t.Helper()
	opts := testutil.DefaultTestFileOptions()
	opts.MaxDownloads = maxDownloads
	opts.ChunkCount = chunkCount
	opts.TotalSize = int64(chunkCount) * int64(opts.ChunkSize)
	return testutil.CreateTestFile(t, queries, ctx, opts)
}

func TestCompleteDownload_Integration_Success(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileWithOpts(t, queries, ctx, 5, 10)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	updatedFile, err := queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
}

func TestCompleteDownload_Integration_LimitReached(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileWithOpts(t, queries, ctx, 1, 1)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	updatedFile, err := queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
	assert.Equal(t, "exhausted", updatedFile.Status)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDownloadLimitReached)
}

func TestCompleteDownload_Integration_FileExpired(t *testing.T) {
	fileService, queries, db, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	file := testutil.CreateExpiredFile(t, queries, db, ctx)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpired)
}

func TestCompleteDownload_Integration_FileNotFound(t *testing.T) {
	fileService, _, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	err := fileService.CompleteDownload(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCompleteDownload_Integration_MultipleDownloads(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	file := createTestFileWithOpts(t, queries, ctx, 3, 10)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	updatedFile, err := queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), updatedFile.DownloadCount)
	assert.Equal(t, "exhausted", updatedFile.Status)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDownloadLimitReached)
}

func TestCompleteDownload_Integration_ConcurrentAccess(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test file with 3 max downloads
	file := createTestFileWithOpts(t, queries, ctx, 3, 10)

	// Launch 10 concurrent goroutines trying to complete downloads
	concurrentRequests := 10
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			results <- fileService.CompleteDownload(ctx, file.ShareID)
		}()
	}

	// Collect results
	var successCount int
	var failureCount int
	for i := 0; i < concurrentRequests; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// Verify exactly 3 succeeded (max_downloads = 3)
	assert.Equal(t, 3, successCount, "Expected exactly 3 successful downloads")
	assert.Equal(t, 7, failureCount, "Expected 7 failed downloads")

	// Verify final state in database
	updatedFile, err := queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), updatedFile.DownloadCount, "Download count should be exactly 3")
	assert.Equal(t, "exhausted", updatedFile.Status, "Status should be exhausted")
}

func TestCompleteDownload_Integration_ConcurrentAccessSingleLimit(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test file with 1 max download
	file := createTestFileWithOpts(t, queries, ctx, 1, 5)

	// Launch 5 concurrent goroutines trying to complete downloads
	concurrentRequests := 5
	results := make(chan error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			results <- fileService.CompleteDownload(ctx, file.ShareID)
		}()
	}

	// Collect results
	var successCount int
	var failureCount int
	for i := 0; i < concurrentRequests; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// Verify exactly 1 succeeded (max_downloads = 1)
	assert.Equal(t, 1, successCount, "Expected exactly 1 successful download")
	assert.Equal(t, 4, failureCount, "Expected 4 failed downloads")

	// Verify final state in database
	updatedFile, err := queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount, "Download count should be exactly 1")
	assert.Equal(t, "exhausted", updatedFile.Status, "Status should be exhausted")
}

func TestInitAndFinalizeUpload_Integration(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	req := types.InitUploadRequest{
		Salt:              "test-salt",
		EncryptedFilename: "encrypted-name",
		EncryptedMimeType: "encrypted-mime",
		TotalSize:         1024 * 1024, // 1MB
		ChunkCount:        4,           // 4 chunks
		ChunkSize:         256 * 1024,  // 256KB
		Pbkdf2Iterations:  100000,
		MaxDownloads:      5,
		ExpiresInHours:    24,
	}

	resp, err := fileService.InitFileUpload(ctx, req, "192.168.1.1")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.FileID)
	assert.NotEmpty(t, resp.ShareID)

	var fileID pgtype.UUID
	err = fileID.Scan(resp.FileID)
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		_, err := queries.CreateChunk(ctx, sqlc.CreateChunkParams{
			FileID:        fileID,
			ChunkIndex:    int32(i),
			StoragePath:   fmt.Sprintf("test/%s/%d.enc", resp.FileID, i),
			EncryptedSize: 256 * 1024,
			ChunkHash:     fmt.Sprintf("hash-%d", i),
		})
		require.NoError(t, err)
	}

	finalizeResp, err := fileService.FinalizeUpload(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, resp.ShareID, finalizeResp.ShareID)

	file, err := queries.GetFileByID(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, "ready", file.Status)
}

func TestFinalizeUpload_Integration_ChunkCountMismatch(t *testing.T) {
	fileService, queries, _, cleanup := setupTestFileService(t)
	defer cleanup()

	ctx := context.Background()

	req := types.InitUploadRequest{
		Salt:              "test-salt",
		EncryptedFilename: "encrypted-name",
		EncryptedMimeType: "encrypted-mime",
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		Pbkdf2Iterations:  100000,
	}

	resp, err := fileService.InitFileUpload(ctx, req, "192.168.1.1")
	require.NoError(t, err)

	var fileID pgtype.UUID
	err = fileID.Scan(resp.FileID)
	require.NoError(t, err)

	// Only upload 2 chunks instead of 4
	for i := 0; i < 2; i++ {
		_, err := queries.CreateChunk(ctx, sqlc.CreateChunkParams{
			FileID:        fileID,
			ChunkIndex:    int32(i),
			StoragePath:   fmt.Sprintf("test/%s/%d.enc", resp.FileID, i),
			EncryptedSize: 256 * 1024,
			ChunkHash:     fmt.Sprintf("hash-%d", i),
		})
		require.NoError(t, err)
	}

	_, err = fileService.FinalizeUpload(ctx, fileID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chunk count does not match")
}
