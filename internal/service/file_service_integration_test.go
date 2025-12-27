package service

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testDBContainer struct {
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
	queries   *sqlc.Queries
}

func setupTestDB(t *testing.T) (*testDBContainer, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	err = runMigrations(ctx, pool)
	require.NoError(t, err, "Failed to run migrations")

	queries := sqlc.New(pool)

	cleanup := func() {
		pool.Close()
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("Failed to terminate container: %s", err)
		}
	}

	return &testDBContainer{
		container: postgresContainer,
		pool:      pool,
		queries:   queries,
	}, cleanup
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS files (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			share_id VARCHAR(12) UNIQUE NOT NULL,
			encrypted_filename TEXT NOT NULL,
			encrypted_mime_type TEXT NOT NULL,
			salt TEXT NOT NULL,
			pbkdf2_iterations INTEGER NOT NULL,
			total_size BIGINT NOT NULL,
			chunk_count INTEGER NOT NULL,
			chunk_size INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'uploading',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			last_downloaded_at TIMESTAMPTZ,
			max_downloads INTEGER NOT NULL DEFAULT 1,
			download_count INTEGER NOT NULL DEFAULT 0,
			deletion_token_hash TEXT,
			uploader_ip INET NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create files table: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS chunks (
			id BIGSERIAL PRIMARY KEY,
			file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
			chunk_index INTEGER NOT NULL,
			storage_path TEXT NOT NULL,
			encrypted_size BIGINT NOT NULL,
			chunk_hash TEXT NOT NULL,
			uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(file_id, chunk_index)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create chunks table: %w", err)
	}

	return nil
}

func TestCompleteDownload_Integration_Success(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	file := createTestFile(t, testDB.queries, ctx, 5, 10)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	updatedFile, err := testDB.queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
}

func TestCompleteDownload_Integration_LimitReached(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	file := createTestFile(t, testDB.queries, ctx, 1, 1)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	updatedFile, err := testDB.queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
	assert.Equal(t, "exhausted", updatedFile.Status)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDownloadLimitReached)
}

func TestCompleteDownload_Integration_FileExpired(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	file := createExpiredFile(t, testDB.queries, ctx)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrExpired)
}

func TestCompleteDownload_Integration_FileNotFound(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	err := fileService.CompleteDownload(ctx, "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCompleteDownload_Integration_MultipleDownloads(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	file := createTestFile(t, testDB.queries, ctx, 3, 10)

	err := fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.NoError(t, err)

	updatedFile, err := testDB.queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), updatedFile.DownloadCount)
	assert.Equal(t, "exhausted", updatedFile.Status)

	err = fileService.CompleteDownload(ctx, file.ShareID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDownloadLimitReached)
}

func TestCompleteDownload_Integration_ConcurrentAccess(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	// Create a test file with 3 max downloads
	file := createTestFile(t, testDB.queries, ctx, 3, 10)

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
	updatedFile, err := testDB.queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), updatedFile.DownloadCount, "Download count should be exactly 3")
	assert.Equal(t, "exhausted", updatedFile.Status, "Status should be exhausted")
}

func TestCompleteDownload_Integration_ConcurrentAccessSingleLimit(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

	// Create a test file with 1 max download
	file := createTestFile(t, testDB.queries, ctx, 1, 5)

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
	updatedFile, err := testDB.queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount, "Download count should be exactly 1")
	assert.Equal(t, "exhausted", updatedFile.Status, "Status should be exhausted")
}

func TestInitAndFinalizeUpload_Integration(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

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
		_, err := testDB.queries.CreateChunk(ctx, sqlc.CreateChunkParams{
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

	file, err := testDB.queries.GetFileByID(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, "ready", file.Status)
}

func TestFinalizeUpload_Integration_ChunkCountMismatch(t *testing.T) {
	testDB, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	txRunner := database.NewTxRunner(testDB.pool)
	fileService := NewFileService(testDB.queries, txRunner, nil)

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
		_, err := testDB.queries.CreateChunk(ctx, sqlc.CreateChunkParams{
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

func createTestFile(t *testing.T, queries *sqlc.Queries, ctx context.Context, maxDownloads, chunkCount int32) sqlc.File {
	t.Helper()

	shareID := generateShareID()

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           shareID,
		EncryptedFilename: "encrypted-test-file",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        chunkCount,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      maxDownloads,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	file, err = queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	return file
}

func createExpiredFile(t *testing.T, queries *sqlc.Queries, ctx context.Context) sqlc.File {
	t.Helper()

	shareID := generateShareID()

	file, err := queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           shareID,
		EncryptedFilename: "encrypted-expired-file",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	file, err = queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	return file
}
