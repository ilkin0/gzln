package sqlc

import (
	"context"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testQueries *Queries
	testPool    *pgxpool.Pool
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	_ = godotenv.Load("../../../.env")

	connString := os.Getenv("DB_URL")
	if connString == "" {
		os.Exit(0)
	}

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		os.Exit(0)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		os.Exit(0)
	}

	testPool = pool
	testQueries = New(pool)

	code := m.Run()

	testPool.Close()
	os.Exit(code)
}

func cleanupFiles(t *testing.T) {
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE files CASCADE")
	require.NoError(t, err)
}

func createTestFileParams(shareID string) CreateFileParams {
	return CreateFileParams{
		ShareID:           shareID,
		EncryptedFilename: "encrypted-test-file",
		EncryptedMimeType: "encrypted-mime-type",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        10,
		ChunkSize:         1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads: 5,
		DeletionTokenHash: pgtype.Text{
			String: "test-token-hash",
			Valid:  true,
		},
		UploaderIp: netip.MustParseAddr("192.168.1.1"),
	}
}

func TestCreateFile(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	params := createTestFileParams("share123")

	file, err := testQueries.CreateFile(ctx, params)

	require.NoError(t, err)
	assert.NotEmpty(t, file.ID)
	assert.Equal(t, params.ShareID, file.ShareID)
	assert.Equal(t, params.EncryptedFilename, file.EncryptedFilename)
	assert.Equal(t, params.EncryptedMimeType, file.EncryptedMimeType)
	assert.Equal(t, params.Salt, file.Salt)
	assert.Equal(t, params.Pbkdf2Iterations, file.Pbkdf2Iterations)
	assert.Equal(t, params.TotalSize, file.TotalSize)
	assert.Equal(t, params.ChunkCount, file.ChunkCount)
	assert.Equal(t, params.ChunkSize, file.ChunkSize)
	assert.Equal(t, params.MaxDownloads, file.MaxDownloads)
	assert.Equal(t, "uploading", file.Status)
	assert.Equal(t, int32(0), file.DownloadCount)
	assert.True(t, file.CreatedAt.Valid)
	assert.True(t, file.ExpiresAt.Valid)
	assert.False(t, file.LastDownloadedAt.Valid)
}

func TestCreateFile_DuplicateShareID(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	shareID := "dup12345678"

	params1 := createTestFileParams(shareID)
	_, err := testQueries.CreateFile(ctx, params1)
	require.NoError(t, err)

	params2 := createTestFileParams(shareID)
	_, err = testQueries.CreateFile(ctx, params2)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestGetFileByID(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	params := createTestFileParams("getbyid123")

	createdFile, err := testQueries.CreateFile(ctx, params)
	require.NoError(t, err)

	retrievedFile, err := testQueries.GetFileByID(ctx, createdFile.ID)

	require.NoError(t, err)
	assert.Equal(t, createdFile.ID, retrievedFile.ID)
	assert.Equal(t, createdFile.ShareID, retrievedFile.ShareID)
	assert.Equal(t, createdFile.EncryptedFilename, retrievedFile.EncryptedFilename)
	assert.Equal(t, createdFile.Status, retrievedFile.Status)
}

func TestGetFileByID_NotFound(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	nonExistentID := pgtype.UUID{
		Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Valid: true,
	}

	_, err := testQueries.GetFileByID(ctx, nonExistentID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestGetFileByShareID(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	shareID := "share123xyz"
	params := createTestFileParams(shareID)

	createdFile, err := testQueries.CreateFile(ctx, params)
	require.NoError(t, err)

	retrievedFile, err := testQueries.GetFileByShareID(ctx, shareID)

	require.NoError(t, err)
	assert.Equal(t, createdFile.ID, retrievedFile.ID)
	assert.Equal(t, shareID, retrievedFile.ShareID)
	assert.Equal(t, createdFile.EncryptedFilename, retrievedFile.EncryptedFilename)
}

func TestGetFileByShareID_NotFound(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()

	_, err := testQueries.GetFileByShareID(ctx, "notfound123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestUpdateFileStatus(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	params := createTestFileParams("upd12345678")

	createdFile, err := testQueries.CreateFile(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "uploading", createdFile.Status)

	updatedFile, err := testQueries.UpdateFileStatus(ctx, UpdateFileStatusParams{
		ID:     createdFile.ID,
		Status: "ready",
	})

	require.NoError(t, err)
	assert.Equal(t, "ready", updatedFile.Status)
	assert.Equal(t, createdFile.ID, updatedFile.ID)

	retrievedFile, err := testQueries.GetFileByID(ctx, createdFile.ID)
	require.NoError(t, err)
	assert.Equal(t, "ready", retrievedFile.Status)
}

func TestUpdateFileStatus_NotFound(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	nonExistentID := pgtype.UUID{
		Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Valid: true,
	}

	_, err := testQueries.UpdateFileStatus(ctx, UpdateFileStatusParams{
		ID:     nonExistentID,
		Status: "ready",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestIncrementDownloadCount(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	params := createTestFileParams("inc12345678")

	createdFile, err := testQueries.CreateFile(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, int32(0), createdFile.DownloadCount)
	assert.False(t, createdFile.LastDownloadedAt.Valid)

	updatedFile, err := testQueries.IncrementDownloadCount(ctx, createdFile.ID)

	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
	assert.True(t, updatedFile.LastDownloadedAt.Valid)

	updatedFile2, err := testQueries.IncrementDownloadCount(ctx, createdFile.ID)

	require.NoError(t, err)
	assert.Equal(t, int32(2), updatedFile2.DownloadCount)
	assert.True(t, updatedFile2.LastDownloadedAt.Valid)
	assert.True(t, updatedFile2.LastDownloadedAt.Time.After(updatedFile.LastDownloadedAt.Time) ||
		updatedFile2.LastDownloadedAt.Time.Equal(updatedFile.LastDownloadedAt.Time))
}

func TestIncrementDownloadCount_NotFound(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()
	nonExistentID := pgtype.UUID{
		Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		Valid: true,
	}

	_, err := testQueries.IncrementDownloadCount(ctx, nonExistentID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestDeleteExpiredFiles(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()

	expiredParams := createTestFileParams("expired123")
	expiredParams.ExpiresAt = pgtype.Timestamptz{
		Time:  time.Now().Add(1 * time.Millisecond),
		Valid: true,
	}
	expiredFile, err := testQueries.CreateFile(ctx, expiredParams)
	require.NoError(t, err)

	maxDownloadParams := createTestFileParams("maxdl12345")
	maxDownloadParams.MaxDownloads = 2
	maxDownloadFile, err := testQueries.CreateFile(ctx, maxDownloadParams)
	require.NoError(t, err)

	_, err = testQueries.UpdateFileStatus(ctx, UpdateFileStatusParams{
		ID:     maxDownloadFile.ID,
		Status: "ready",
	})
	require.NoError(t, err)
	_, err = testQueries.IncrementDownloadCount(ctx, maxDownloadFile.ID)
	require.NoError(t, err)
	_, err = testQueries.IncrementDownloadCount(ctx, maxDownloadFile.ID)
	require.NoError(t, err)

	validParams := createTestFileParams("valid12345")
	validParams.ExpiresAt = pgtype.Timestamptz{
		Time:  time.Now().Add(24 * time.Hour),
		Valid: true,
	}
	validFile, err := testQueries.CreateFile(ctx, validParams)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	err = testQueries.DeleteExpiredFiles(ctx)
	require.NoError(t, err)

	_, err = testQueries.GetFileByID(ctx, expiredFile.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")

	_, err = testQueries.GetFileByID(ctx, maxDownloadFile.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")

	retrievedFile, err := testQueries.GetFileByID(ctx, validFile.ID)
	require.NoError(t, err)
	assert.Equal(t, validFile.ID, retrievedFile.ID)
}

func TestDeleteExpiredFiles_NoExpired(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}
	cleanupFiles(t)

	ctx := context.Background()

	params1 := createTestFileParams("valid1234")
	params1.ExpiresAt = pgtype.Timestamptz{
		Time:  time.Now().Add(24 * time.Hour),
		Valid: true,
	}
	file1, err := testQueries.CreateFile(ctx, params1)
	require.NoError(t, err)

	params2 := createTestFileParams("valid5678")
	params2.ExpiresAt = pgtype.Timestamptz{
		Time:  time.Now().Add(48 * time.Hour),
		Valid: true,
	}
	file2, err := testQueries.CreateFile(ctx, params2)
	require.NoError(t, err)

	err = testQueries.DeleteExpiredFiles(ctx)
	require.NoError(t, err)

	_, err = testQueries.GetFileByID(ctx, file1.ID)
	require.NoError(t, err)

	_, err = testQueries.GetFileByID(ctx, file2.ID)
	require.NoError(t, err)
}
